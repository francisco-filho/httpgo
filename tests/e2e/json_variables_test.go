package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTypedProductsDemoByBinary(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_products_typed.hurl")
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || r.URL.Path != "/products" || r.Header.Get("Content-Type") != "application/json" || string(body) != `{"name":"Pizza","price":4500,"active":true,"code":"001"}` {
			t.Errorf("request=%+v body=%s", r, body)
		}
		w.WriteHeader(201)
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, string(input)), "--var", "api.baseUrl="+server.URL, "--var", "product.name=Pizza", "--var", "product.price=4500", "--var", "product.active=true", "--var", `product.code="001"`)
	if code != 0 || stderr != "" || hits.Load() != 1 || !strings.Contains(out, "validadas=1") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}

func TestTypedJSONRootsAndStringsByBinary(t *testing.T) {
	const special = "quote\" slash\\ newline\n雪 {{missing}}"
	encoded, _ := json.Marshal(special)
	for _, tc := range []struct{ value, want string }{
		{"null", "null"}, {"[]", "[]"}, {`{"big":9007199254740993123456789,"literal":"{{missing}}"}`, `{"big":9007199254740993123456789,"literal":"{{missing}}"}`},
		{"9007199254740993123456789", "9007199254740993123456789"}, {"1.2300e+99", "1.2300e+99"}, {"false", "false"},
		{`"001"`, `"001"`}, {"a=b=c", `"a=b=c"`}, {"", `""`}, {"001", `"001"`}, {string(encoded), string(encoded)},
	} {
		t.Run(tc.value, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				switch r.URL.Path {
				case "/root":
					if string(body) != tc.want {
						t.Errorf("got=%s want=%s", body, tc.want)
					}
				case "/string":
					var value string
					if err := json.Unmarshal(body, &value); err != nil || value != "pre\n"+special+"\t-42-true-null-" {
						t.Errorf("body=%s decoded=%q err=%v", body, value, err)
					}
				default:
					t.Errorf("unexpected %s", r.URL.Path)
				}
				w.WriteHeader(201)
			}))
			defer server.Close()
			source := "POST {{base}}/root\r\n{{value}}\r\nHTTP 201\r\nPOST {{base}}/string\r\n\"pre\\n{{special}}\\t-{{n}}-{{b}}-{{nil}}-{{empty}}\"\r\nHTTP 201"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--var", "value="+tc.value, "--var", "special="+string(encoded), "--var", "n=42", "--var", "b=true", "--var", "nil=null", "--var", "empty=")
			if code != 0 || stderr != "" || hits.Load() != 2 || !strings.Contains(out, "validadas=2") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
		})
	}
}

func TestTypedJSONFailureSequenceByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, second, value, position string
		allBlocked                    bool
	}{
		{"missing", "POST {{base}}/affected\r\n{\r\n \"first\":\"{{x}}\",\r\n \"next\":{{missing}}\r\n}", `"escapes\"\\\n雪"`, ":6:9:", false},
		{"opaque", "POST {{base}}/affected\r\n{{x.name}}", `{"name":"not-a-variable"}`, ":4:1:", false},
		{"array URL", "GET {{base}}/{{x}}", `[]`, ":3:", false},
		{"object header", "GET {{base}}/affected\r\nX-Probe: {{x}}", `{}`, ":4:", false},
		{"array string", "POST {{base}}/affected\r\n\"{{x}}\"", `[]`, ":4:2:", false},
		{"decoded URL", "GET {{base}}/{{x}}", `"unencoded space"`, ":3:", false},
		{"decoded header", "GET {{base}}/affected\r\nX-Probe: {{x}}", `"fictitious\r\nInjected: yes"`, ":4:", false},
		{"decoded Host", "GET {{base}}/affected\r\nHost: {{x}}", `"user:fictitious@example.test"`, ":4:", false},
		{"bad template", "POST {{base}}/affected\r\n{\r\n \"next\":{{bad-name}}\r\n}", `null`, ":5:9:", true},
		{"dynamic key", "POST {{base}}/affected\r\n{\"{{x}}\":1}", `"key"`, ":4:3:", true},
		{"invalid structure", "POST {{base}}/affected\r\n{\"a\":{{x}},}", `null`, ":4:", true},
		{"function", "POST {{base}}/affected\r\n{{x()}}", `null`, ":4:", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path != "/first" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				w.WriteHeader(200)
			}))
			defer server.Close()
			file := scenario(t, "GET {{base}}/first\r\nHTTP 200\r\n"+tc.second+"\r\nGET {{base}}/last")
			code, out, stderr := run(t, "test", file, "--var", "base="+server.URL, "--var", "x="+tc.value, "--secret", "x")
			wantHits := int32(1)
			if tc.allBlocked {
				wantHits = 0
			}
			if code != 2 || hits.Load() != wantHits || !strings.Contains(stderr, tc.position) || strings.Contains(out+stderr, "fictitious") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
			if tc.allBlocked {
				if out != "" {
					t.Fatalf("partial scenario executed: %q", out)
				}
			} else if !strings.Contains(out, "iniciadas=1 concluídas=1 validadas=1") || !strings.Contains(out, "não_iniciadas=2") {
				t.Fatalf("out=%q", out)
			}
		})
	}
}

func TestTypedVariableConflictsByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	file := scenario(t, "GET "+server.URL)
	for _, assignments := range [][]string{{`x={"name":"Pizza"}`, "x.name=Pizza"}, {"x.name=Pizza", `x={"name":"Pizza"}`}, {"x=null", "x.name=Pizza"}, {"x=[]", "x.name=Pizza"}, {"x=true", "x=true"}} {
		args := []string{"test", file}
		for _, value := range assignments {
			args = append(args, "--var", value)
		}
		code, out, stderr := run(t, args...)
		if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
			t.Fatalf("args=%q code=%d out=%q err=%q hits=%d", assignments, code, out, stderr, hits.Load())
		}
	}
}

func TestTypedSecretsByBinary(t *testing.T) {
	const secret = "fictional\"\\\n雪"
	encoded, _ := json.Marshal(secret)
	for index, tc := range []struct {
		value        string
		status, code int
	}{
		{string(encoded), 200, 0}, {string(encoded), 503, 1},
		{`["fictional\nvalue",987654321]`, 200, 0}, {`{"hidden":"fictional\nvalue"}`, 200, 0},
		{`987654321`, 200, 0}, {`true`, 200, 0}, {`null`, 200, 0},
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				body, _ := io.ReadAll(r.Body)
				if r.URL.Path != "/last" && !json.Valid(body) {
					t.Errorf("body=%s", body)
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "POST {{base}}/\n{\"value\":{{s}}}\nHTTP 200\nGET {{base}}/last"), "--secret", "s", "--var", "base="+server.URL, "--var", "s="+tc.value)
			wantHits := int32(2)
			if tc.code == 1 {
				wantHits = 1
			}
			if code != tc.code || hits.Load() != wantHits {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
			for _, variant := range []string{secret, string(encoded), string(encoded[1 : len(encoded)-1]), tc.value, "fictional", "987654321"} {
				if strings.Contains(out+stderr, variant) {
					t.Fatalf("leaked %q: %q %q", variant, out, stderr)
				}
			}
		})
	}
	for _, filename := range []string{secret, string(encoded[1 : len(encoded)-1]), url.PathEscape(secret), url.QueryEscape(secret)} {
		code, out, stderr := run(t, "test", filename, "--var", "s="+string(encoded), "--secret", "s")
		if code != 2 || strings.Contains(out+stderr, "fictional") || strings.Contains(out+stderr, "%E9%9B%AA") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}

func TestStaticCRLFAndJSONTemplateBoundariesByBinary(t *testing.T) {
	body := "{\r\n \"nested\":{\"empty\":{}},\r\n \"instruction\":\"HTTP 200 GET # [Asserts]\"\r\n}"
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		got, _ := io.ReadAll(r.Body)
		if r.URL.Path == "/static" && string(got) != body || r.URL.Path == "/root" && string(got) != `{"a":{}}` {
			t.Errorf("path=%s body=%q", r.URL.Path, got)
		}
		w.WriteHeader(201)
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, "POST {{base}}/static\r\n"+body+"\r\nHTTP 201\r\nPOST {{base}}/root\r\n{{root}}\r\nHTTP 201"), "--var", "base="+server.URL, "--var", `root={"a":{}}`)
	if code != 0 || stderr != "" || hits.Load() != 2 || !strings.Contains(out, "validadas=2") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}
