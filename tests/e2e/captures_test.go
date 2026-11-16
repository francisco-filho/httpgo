package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

func TestAuthenticationCaptureDemoByBinary(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_auth_capture.hurl")
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		switch n {
		case 1:
			body, _ := io.ReadAll(r.Body)
			if r.Method != "POST" || r.URL.Path != "/auth/login" || r.Header.Get("Content-Type") != "application/json" || string(body) != `{"username":"demo","password":"senha-ficticia"}` {
				t.Errorf("method=%s path=%s body=%s", r.Method, r.URL.Path, body)
			}
			_, _ = io.WriteString(w, `{"accessToken":"token-ficticio"}`)
		case 2:
			if r.Method != "GET" || r.URL.Path != "/products" || r.Header.Get("Authorization") != "Bearer token-ficticio" {
				t.Error("protected request did not receive the captured token")
			}
		default:
			t.Error("unexpected request")
		}
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, string(input)), "--var", "base_url="+server.URL, "--var", "username=demo", "--var", "password=senha-ficticia", "--secret", "password", "--secret", "accessToken")
	if code != 0 || stderr != "" || hits.Load() != 2 || !strings.Contains(out, "validadas=2") || strings.Contains(out+stderr, "token-ficticio") || strings.Contains(out+stderr, "senha-ficticia") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
	// A second process/file cannot resolve the first process's capture.
	code, out, stderr = run(t, "test", scenario(t, "GET {{base}}/products\nAuthorization: Bearer {{accessToken}}"), "--var", "base="+server.URL, "--secret", "accessToken")
	if code != 2 || hits.Load() != 2 || !strings.Contains(stderr, "variável ausente") || !strings.Contains(out, "iniciadas=0") {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
}

func TestCaptureStringsContextsAndMethodsByBinary(t *testing.T) {
	for _, value := range []string{"123", "null", "true", "", `{{missing}}`, "quote\"\\\n雪", "a%2Fb"} {
		t.Run(value, func(t *testing.T) {
			encoded, _ := json.Marshal(value)
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				if n == 1 {
					_, _ = io.WriteString(w, `{"items":[{"value":`+string(encoded)+`}],"empty":""}`)
					return
				}
				methods := []string{"POST", "PUT", "PATCH", "DELETE"}
				body, _ := io.ReadAll(r.Body)
				var got map[string]string
				if n > 5 || r.Method != methods[n-2] || json.Unmarshal(body, &got) != nil || got["value"] != value || got["text"] != "prefix-"+value || got["empty"] != "" {
					t.Errorf("request %d method=%s body=%s", n, r.Method, body)
				}
				if value == "a%2Fb" && r.RequestURI != "/a%2Fb" {
					t.Errorf("encoding changed: %s", r.RequestURI)
				}
			}))
			defer server.Close()
			source := "GET {{base}}/capture\nHTTP 200\n[Captures]\nauth.token: jsonpath \"$.items[0].value\"\nempty: jsonpath \"$.empty\"\n"
			for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
				target := "/products"
				if value == "a%2Fb" {
					target = "/{{auth.token}}"
				}
				source += method + " {{base}}" + target + "\nContent-Type: application/json\n{\"value\":{{auth.token}},\"text\":\"prefix-{{auth.token}}\",\"empty\":{{empty}}}\nHTTP 200\n"
			}
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--secret", "auth.token", "--secret", "empty")
			if code != 0 || stderr != "" || hits.Load() != 5 || !strings.Contains(out, "validadas=5") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
		})
	}
}

func TestCaptureResponseFailuresByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, body, path, expectation string
		status, code                  int
		detail                        string
	}{
		{"invalid", "response-body-secret", "$", "HTTP 200\n", 200, 2, "JSON válido"},
		{"additional values", `null true`, "$", "HTTP 200\n", 200, 2, "JSON válido"},
		{"missing", `{}`, "$.missing", "HTTP 200\n", 200, 2, "não encontrou"},
		{"index", `[]`, "$[0]", "HTTP 200\n", 200, 2, "não encontrou"},
		{"null traversal", `null`, "$.child", "HTTP 200\n", 200, 2, "não encontrou"},
		{"number traversal", `123`, "$[0]", "HTTP 200\n", 200, 2, "não encontrou"},
		{"bool traversal", `true`, "$.child", "HTTP 200\n", 200, 2, "não encontrou"},
		{"object index", `{}`, "$[0]", "HTTP 200\n", 200, 2, "não encontrou"},
		{"array property", `[]`, "$.child", "HTTP 200\n", 200, 2, "não encontrou"},
		{"mismatch", "invalid-json", "$", "HTTP 200\n", 503, 1, "status divergente"},
		{"expected 404", `"captured"`, "$", "HTTP 404\n", 404, 0, ""},
		{"no expectation", `"captured"`, "$", "", 500, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/login" {
					w.WriteHeader(tc.status)
					_, _ = io.WriteString(w, tc.body)
					return
				}
				if r.Header.Get("X-Token") != "captured" {
					t.Error("capture not reused")
				}
			}))
			defer server.Close()
			source := "GET {{base}}/login\r\n" + strings.ReplaceAll(tc.expectation, "\n", "\r\n") + "[Captures]\r\ntoken: jsonpath \"" + tc.path + "\"\r\nGET {{base}}/next\r\nX-Token: {{token}}"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--secret", "token")
			wantHits := int32(1)
			if tc.code == 0 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits || !strings.Contains(stderr, tc.detail) || strings.Contains(out+stderr, "response-body-secret") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
			if code == 2 && (!strings.Contains(stderr, ":4:18:") || !strings.Contains(out, "concluídas=1 validadas=1") || strings.Contains(out, "erro de transporte")) {
				t.Fatalf("out=%q err=%q", out, stderr)
			}
		})
	}
}

func TestCaptureStructuralCorpusByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, pattern := range []string{"invalid_capture*.hurl", "unsupported_capture*.hurl"} {
		files, err := filepath.Glob(filepath.Join("../../testdata/scenarios", pattern))
		if err != nil || len(files) == 0 {
			t.Fatalf("%v %v", files, err)
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			source := "GET " + server.URL + "/first\n" + strings.ReplaceAll(string(data), "http://localhost", server.URL)
			code, out, stderr := run(t, "test", scenario(t, source))
			if code != 2 || !prevalidationSummary(out) || stderr == "" || hits.Load() != 0 {
				t.Fatalf("%s code=%d out=%q err=%q hits=%d", file, code, out, stderr, hits.Load())
			}
		}
	}
	for _, declaration := range []string{`bad-name: jsonpath "$"`, `token jsonpath "$"`, `token: jsonpath "$" garbage`} {
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\r\nGET "+server.URL+"/second\r\n[Captures]\r\n  "+declaration))
		if code != 2 || !prevalidationSummary(out) || !strings.Contains(stderr, ":4:3:") || hits.Load() != 0 {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}

func TestCaptureNamesEarlyUseAndConflictsByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, first, next string
		variables         []string
		hits              int32
	}{
		{"initial", `token: jsonpath "$"`, "GET {{base}}/last", []string{"token=null"}, 1},
		{"ancestor", `token.child: jsonpath "$"`, "GET {{base}}/last", []string{"token=[]"}, 1},
		{"descendant", `token: jsonpath "$"`, "GET {{base}}/last", []string{`token.child={}`}, 1},
		{"batch", "token: jsonpath \"$\"\ntoken.child: jsonpath \"$\"", "GET {{base}}/last", nil, 1},
		{"previous", `token: jsonpath "$"`, "GET {{base}}/again\n[Captures]\ntoken: jsonpath \"$\"\nGET {{base}}/last", nil, 2},
		{"previous hierarchy", `auth.token: jsonpath "$"`, "GET {{base}}/again\n[Captures]\nauth: jsonpath \"$\"\nGET {{base}}/last", nil, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				_, _ = io.WriteString(w, `"received-token"`)
			}))
			defer server.Close()
			args := []string{"test", scenario(t, "GET {{base}}/login\n[Captures]\n"+tc.first+"\n"+tc.next), "--var", "base=" + server.URL, "--secret", "token", "--secret", "auth.token"}
			for _, v := range tc.variables {
				args = append(args, "--var", v)
			}
			code, out, stderr := run(t, args...)
			if code != 2 || hits.Load() != tc.hits || !strings.Contains(stderr, "erro de captura") || strings.Contains(out+stderr, "received-token") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
		})
	}
	for _, next := range []string{"GET {{base}}/{{token}}\n[Captures]\ntoken: jsonpath \"$\"", "GET {{base}}/first\nHTTP 200\nGET {{base}}/{{missing}}\n[Captures]\nmissing: jsonpath \"$\""} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
		code, out, stderr := run(t, "test", scenario(t, next), "--var", "base="+server.URL, "--secret", "token")
		server.Close()
		want := int32(0)
		if strings.Contains(next, "/first") {
			want = 1
		}
		if code != 2 || hits.Load() != want || !strings.Contains(stderr, "variável ausente") || !strings.Contains(out, fmt.Sprintf("iniciadas=%d", want)) {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}

func TestCaptureSecretVariantsAndInvalidRequestsByBinary(t *testing.T) {
	for _, tc := range []struct {
		value, next string
		code        int
		hits        int32
	}{
		{"secret%2Fknown", "GET {{base}}/{{token}}\nX-Ordinary: {{token}}\nHTTP 200", 0, 2},
		{"secret%2Fknown", "GET {{base}}/{{token}}\nHTTP 201", 1, 2},
		{"unencoded secret", "GET {{base}}/{{token}}", 2, 1},
		{"secret\r\nInjected: yes", "GET {{base}}/next\nX-Ordinary: {{token}}", 2, 1},
		{"escaped\"\\\n雪", "POST {{base}}/next\n{\"value\":{{token}}}\nGET {{base}}/{{missing}}", 2, 2},
		{"null", "POST {{base}}/next\n{{token}}", 0, 2},
		{"", "GET {{base}}/{{token}}", 0, 2},
	} {
		t.Run(tc.value+tc.next, func(t *testing.T) {
			var hits atomic.Int32
			encoded, _ := json.Marshal(tc.value)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/login" {
					_, _ = io.WriteString(w, `{"token":`+string(encoded)+`}`)
				}
			}))
			defer server.Close()
			file := filepath.Join(t.TempDir(), "flow-"+url.QueryEscape(tc.value)+".hurl")
			source := "POST {{base}}/login\n{\"password\":\"{{password}}\"}\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\n" + tc.next
			if err := os.WriteFile(file, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			code, out, stderr := run(t, "test", file, "--var", "base="+server.URL, "--secret", "token", "--secret", "password", "--var", "password=senha-ficticia")
			if code != tc.code || hits.Load() != tc.hits {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
			if tc.value != "" {
				for _, variant := range template.SecretVariants(string(encoded)) {
					if strings.Contains(out+stderr, variant) {
						t.Fatalf("leaked %q in %q %q", variant, out, stderr)
					}
				}
			}
			if strings.Contains(out+stderr, "senha-ficticia") {
				t.Fatal("password exposed")
			}
		})
	}
}
