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

func TestTypedCaptureHelpByBinary(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%q", code, stderr)
		}
		for _, text := range []string{"Capturas tipadas", "9007199254740993", "número, booleano, null, array ou objeto", "raiz e vazios", "precisão", "Array/objeto exige valor JSON completo", "Objetos não criam variáveis", "índice fora do array"} {
			if !strings.Contains(out, text) {
				t.Fatalf("help lacks %q", text)
			}
		}
		if strings.Contains(out, "Capturas string") || strings.Contains(out, "Somente strings") {
			t.Fatal("stale help")
		}
	}
}

func TestTypedCaptureProductsDemoByBinary(t *testing.T) {
	for _, authenticated := range []bool{false, true} {
		t.Run(fmt.Sprint(authenticated), func(t *testing.T) {
			file := "scenario_capture_products.hurl"
			if authenticated {
				file = "scenario_capture_auth_products.hurl"
			}
			data, err := os.ReadFile(filepath.Join("../../testdata/scenarios", file))
			if err != nil {
				t.Fatal(err)
			}
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				body, _ := io.ReadAll(r.Body)
				if authenticated && r.URL.Path != "/auth/login" && r.Header.Get("Authorization") != "Bearer fixture-token" {
					t.Error("missing captured token")
				}
				switch r.URL.Path {
				case "/auth/login":
					if !authenticated || r.Method != "POST" || string(body) != `{"username":"demo","password":"senha-ficticia"}` {
						t.Errorf("%s %s", r.Method, body)
					}
					_, _ = io.WriteString(w, `{"accessToken":"fixture-token"}`)
				case "/products":
					if r.Method != "POST" || string(body) != `{"name":"Pizza"}` || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("%s %s", r.Method, body)
					}
					w.WriteHeader(201)
					_, _ = io.WriteString(w, `{"id":9007199254740993}`)
				case "/products/9007199254740993":
					if r.Method != "GET" || r.RequestURI != "/products/9007199254740993" {
						t.Errorf("%s %s", r.Method, r.RequestURI)
					}
				case "/echo":
					if r.Method != "POST" || string(body) != `{"productId":9007199254740993}` {
						t.Errorf("%s %s", r.Method, body)
					}
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
			}))
			defer server.Close()
			args := []string{"test", scenario(t, string(data)), "--var", "base_url=" + server.URL}
			total := 3
			if authenticated {
				total++
				args = append(args, "--var", "username=demo", "--var", "password=senha-ficticia", "--secret", "password", "--secret", "accessToken")
			}
			code, out, stderr := run(t, args...)
			if code != 0 || stderr != "" || hits.Load() != int32(total) || !strings.Contains(out, fmt.Sprintf("validadas=%d", total)) || strings.Contains(out+stderr, "fixture-token") || strings.Contains(out+stderr, "senha-ficticia") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			code, out, stderr = run(t, "test", scenario(t, "GET {{base}}/products/{{productId}}"), "--var", "base="+server.URL)
			if code != 2 || hits.Load() != int32(total) || !strings.Contains(stderr, "variável ausente") || !strings.Contains(out, "iniciadas=0") {
				t.Fatalf("capture leaked to independent process: code=%d out=%q err=%q", code, out, stderr)
			}
		})
	}
}

func TestTypedCaptureValuesAndPresenceByBinary(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/scenarios/scenario_capture_typed_root.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`""`, `"123"`, `"null"`, `"true"`, `"[1]"`, `"{\"a\":1}"`, `"{{missing}}"`, `"quote\"\\\n\t雪"`, `9007199254740993`, `-9007199254740993123456789`, `1.2300e+99`, `true`, `false`, `null`, `[]`, `{}`, `{"nested":[9007199254740993,1.2300e+99,"{{missing}}",null]}`, `[{},[],9007199254740993,false]`} {
		for _, wrapper := range []struct{ path, prefix, suffix string }{{"$", "", ""}, {"$.value", `{"value":`, `}`}, {"$[0].value[0]", `[{"value":[`, `]}]`}} {
			t.Run(raw+wrapper.path, func(t *testing.T) {
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					n := hits.Add(1)
					if n == 1 {
						_, _ = io.WriteString(w, wrapper.prefix+raw+wrapper.suffix)
						return
					}
					body, _ := io.ReadAll(r.Body)
					// Exact JSON bytes check all numeric spellings without float64 conversion.
					if n != 2 || r.Method != "POST" || r.URL.Path != "/echo" || string(body) != raw {
						t.Errorf("request %d: %s %s body=%s want=%s", n, r.Method, r.URL.Path, body, raw)
					}
				}))
				defer server.Close()
				source := strings.ReplaceAll(string(fixture), `jsonpath "$"`, `jsonpath "`+wrapper.path+`"`)
				code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL)
				if code != 0 || stderr != "" || hits.Load() != 2 || !strings.Contains(out, "validadas=2") {
					t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
				}
			})
		}
	}
}

func TestTypedScalarCaptureTextByBinary(t *testing.T) {
	for _, tc := range []struct{ raw, text string }{{`9007199254740993`, "9007199254740993"}, {`1.2300e+99`, "1.2300e+99"}, {`true`, "true"}, {`false`, "false"}, {`null`, "null"}, {`"a%2Fb"`, "a%2Fb"}} {
		t.Run(tc.raw, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if hits.Add(1) == 1 {
					_, _ = io.WriteString(w, tc.raw)
					return
				}
				body, _ := io.ReadAll(r.Body)
				var got map[string]json.RawMessage
				var text string
				if r.RequestURI != "/"+tc.text || r.Header.Get("X-Probe") != "pre"+tc.text || json.Unmarshal(body, &got) != nil || json.Unmarshal(got["text"], &text) != nil || text != "pre"+tc.text || string(got["value"]) != tc.raw {
					t.Errorf("uri=%s headers=%v body=%s", r.RequestURI, r.Header, body)
				}
			}))
			defer server.Close()
			source := "GET {{base}}/capture\nHTTP 200\n[Captures]\nx: jsonpath \"$\"\nPOST {{base}}/{{x}}\nX-Probe: pre{{x}}\nContent-Type: application/json\n{\"text\":\"pre{{x}}\",\"value\":{{x}}}\nHTTP 200"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL)
			if code != 0 || stderr != "" || hits.Load() != 2 {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestCompositeCaptureTextRejectionByBinary(t *testing.T) {
	data, err := os.ReadFile("../../testdata/scenarios/scenario_capture_composite_text.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`[]`, `{"private":"hidden"}`} {
		for _, nl := range []string{"\n", "\r\n"} {
			for _, tc := range []struct{ next, position string }{
				{string(data), ":8:18:"},
				{"GET {{base}}/capture\nHTTP 200\n[Captures]\nvalue: jsonpath \"$\"\nGET {{base}}/{{value}}\nGET {{base}}/last", ":5:14:"},
				{"GET {{base}}/capture\nHTTP 200\n[Captures]\nvalue: jsonpath \"$\"\nGET {{base}}/next\nX-Probe: {{value}}\nGET {{base}}/last", ":6:10:"},
			} {
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); _, _ = io.WriteString(w, raw) }))
				code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(tc.next, "\n", nl)), "--var", "base="+server.URL, "--secret", "value")
				server.Close()
				if code != 2 || hits.Load() != 1 || !strings.Contains(stderr, tc.position+" erro de renderização") || !strings.Contains(out, "concluídas=1 validadas=1") || strings.Contains(out+stderr, "hidden") {
					t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
				}
			}
		}
	}
}

func TestTypedCaptureSecretVariantsByBinary(t *testing.T) {
	for _, tc := range []struct{ raw, part string }{
		{`9007199254740993`, "9007199254740993"}, {`true`, "true"}, {`null`, "null"},
		{`{"private":["hidden\"\\\n雪",9007199254740993]}`, "hidden\"\\\n雪"},
		{`["hidden/a b%?=雪",1.2300e+99]`, "hidden/a b%?=雪"},
	} {
		for _, mode := range []string{"success", "status", "capture", "render", "transport"} {
			t.Run(tc.raw+mode, func(t *testing.T) {
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					n := hits.Add(1)
					if n == 1 {
						_, _ = io.WriteString(w, tc.raw)
						return
					}
					if mode == "transport" {
						connection, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = connection.Close()
						return
					}
					if n == 2 {
						body, _ := io.ReadAll(r.Body)
						if string(body) != tc.raw {
							t.Errorf("protection changed data: %s", body)
						}
						if mode == "status" {
							w.WriteHeader(503)
						}
					}
				}))
				defer server.Close()
				source := "GET {{base}}/capture\nHTTP 200\n[Captures]\nsecret: jsonpath \"$\"\n"
				if mode == "capture" {
					source += "missing: jsonpath \"$.absent\"\n"
				}
				source += "POST {{base}}/" + url.PathEscape(tc.part) + "\n{{secret}}\nHTTP 200\n"
				if mode == "render" {
					source += "GET {{base}}/{{missing}}\n"
				}
				source += "GET {{base}}/last"
				file := filepath.Join(t.TempDir(), "flow-"+url.QueryEscape(tc.part)+".hurl")
				if err := os.WriteFile(file, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				code, out, stderr := run(t, "test", file, "--var", "base="+server.URL, "--secret", "secret")
				wantCode, wantHits := 0, int32(3)
				switch mode {
				case "status":
					wantCode, wantHits = 1, 2
				case "capture":
					wantCode, wantHits = 2, 1
				case "render", "transport":
					wantCode, wantHits = 2, 2
				}
				if code != wantCode || hits.Load() != wantHits {
					t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
				}
				for _, variant := range template.SecretVariants(tc.raw) {
					if strings.Contains(out+stderr, variant) {
						t.Fatalf("leaked %q in %q %q", variant, out, stderr)
					}
				}
			})
		}
	}
}
