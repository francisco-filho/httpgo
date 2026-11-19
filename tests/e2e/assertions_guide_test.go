package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAssertionsGuideCatalog(t *testing.T) {
	data, err := os.ReadFile("../../docs/assertions.md")
	if err != nil {
		t.Fatal(err)
	}
	jsonBody := guideBlock(t, "docs/assertions.md", "### Entrada comum e execução dos exemplos", "json")
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" {
			t.Errorf("method=%s", r.Method)
		}
		switch r.URL.Path {
		case "/validation":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, jsonBody)
		case "/headers":
			w.Header().Add("X-Request-Id", "req-123")
			w.Header().Add("X-Request-Id", "other")
			w.Header().Set("X-Empty", "")
		case "/export":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, "customer Pizza")
		case "/message/123":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, "Seu código de verificação é: 123456")
		default:
			t.Errorf("unexpected path=%s", r.URL.Path)
		}
	}))
	defer server.Close()
	// Every executable first-column example in the catalog is run verbatim.
	rows := regexp.MustCompile("(?m)^\\| `((?:jsonpath|header|body|regex) [^`]+)` \\|")
	counts := make(map[string]int)
	for _, row := range rows.FindAllStringSubmatch(string(data), -1) {
		declaration := row[1]
		kind, _, _ := strings.Cut(declaration, " ")
		counts[kind]++
		t.Run(declaration, func(t *testing.T) {
			path := map[string]string{"jsonpath": "/validation", "header": "/headers", "body": "/export", "regex": "/message/123"}[kind]
			file := writeReadmeFile(t, t.TempDir(), "catalogo.httpgo", "GET {{base_url}}"+path+"\nHTTP 200\n[Asserts]\n"+declaration+"\n")
			before := calls.Load()
			code, out, stderr := run(t, "test", file, "--var", "base_url="+server.URL)
			if code != 0 || stderr != "" || calls.Load()-before != 1 || !strings.Contains(out, "validadas=1") {
				t.Fatalf("code=%d calls=%d out=%q stderr=%q", code, calls.Load()-before, out, stderr)
			}
		})
	}
	for kind, want := range map[string]int{"jsonpath": 20, "header": 5, "body": 2, "regex": 5} {
		if counts[kind] != want {
			t.Errorf("catalog examples for %s=%d want=%d", kind, counts[kind], want)
		}
	}
}

func TestAssertionsGuideRotation(t *testing.T) {
	input := guideBlock(t, "docs/assertions.md", "Salve `rotacao.httpgo`", "hurl")
	for _, tc := range []struct {
		name, renewed string
		code, calls   int
	}{
		{"different", `{"accessToken":"novo-ficticio"}`, 0, 3},
		{"repeated", `{"accessToken":"antigo-ficticio"}`, 1, 2},
		{"missing", `{}`, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch r.URL.Path {
				case "/session/login":
					_, _ = io.WriteString(w, `{"accessToken":"antigo-ficticio"}`)
				case "/session/renew":
					_, _ = io.WriteString(w, tc.renewed)
				case "/next":
					if r.Header.Get("Authorization") != "Bearer novo-ficticio" {
						t.Error("new token not reused in Authorization")
					}
				default:
					t.Errorf("unexpected path=%s", r.URL.Path)
				}
			}))
			defer server.Close()
			file := writeReadmeFile(t, t.TempDir(), "rotacao.httpgo", input)
			code, out, stderr := run(t, "test", file, "--render-asserts", "--secret", "old_token", "--secret", "new_token", "--var", "base_url="+server.URL)
			if code != tc.code || int(calls.Load()) != tc.calls || (code == 0 && stderr != "") || (code != 0 && stderr == "") {
				t.Fatalf("code=%d want=%d calls=%d want=%d out=%q stderr=%q", code, tc.code, calls.Load(), tc.calls, out, stderr)
			}
			if strings.Contains(out+stderr, "antigo-ficticio") || strings.Contains(out+stderr, "novo-ficticio") {
				t.Fatal("rotation disclosed a protected token")
			}
			if tc.name == "missing" && !strings.Contains(stderr, "erro de captura") {
				t.Errorf("missing capture error: %s", stderr)
			}
		})
	}
}

func TestAssertionsGuideFailureDistinctions(t *testing.T) {
	for _, tc := range []struct {
		name, declaration, body string
		code, calls             int
		render                  bool
	}{
		{"missing inequality", `jsonpath "$.missing" != null`, `{}`, 1, 1, false},
		{"null exists", `jsonpath "$.value" exists`, `{"value":null}`, 0, 1, false},
		{"not exists null", `jsonpath "$.value" not exists`, `{"value":null}`, 1, 1, false},
		{"null membership negation", `jsonpath "$.value" not contains "x"`, `{"value":null}`, 1, 1, false},
		{"strict traversal", `jsonpath "$.value.id" not exists`, `{"value":null}`, 2, 1, false},
		{"integer notation", `jsonpath "$.value" isInteger`, `{"value":3.0}`, 1, 1, false},
		{"count non array", `jsonpath "$.value" count == 0`, `{"value":null}`, 1, 1, false},
		{"regex missing group", `regex "code=([0-9]+)" exists`, `no match`, 1, 1, false},
		{"regex missing negated", `regex "code=([0-9]+)" not contains "x"`, `no match`, 0, 1, false},
		{"regex without group", `regex "code" not contains "x"`, `code`, 2, 1, false},
		{"regex empty group", `regex "code=(.*)" == ""`, `code=`, 0, 1, false},
		{"literal expected", `jsonpath "$.value" == "{{expected}}"`, `{"value":"Pizza"}`, 1, 1, false},
		{"rendered expected", `jsonpath "$.value" == "{{expected}}"`, `{"value":"Pizza"}`, 0, 1, true},
		{"missing dynamic before absence", `jsonpath "$.missing" not contains "{{unknown}}"`, `{}`, 2, 1, true},
		{"invalid dynamic regex", `jsonpath "$.value" matches "{{pattern}}"`, `{"value":"Pizza"}`, 2, 1, true},
		{"bare literal placeholder", `jsonpath "$.value" == {{expected}}`, ``, 2, 0, false},
		{"unsupported count inequality", `jsonpath "$.value" count != 0`, ``, 2, 0, false},
		{"unsupported body matches", `body matches "Pizza"`, ``, 2, 0, false},
		{"unsupported header not exists", `header "X-Missing" not exists`, ``, 2, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			file := writeReadmeFile(t, t.TempDir(), "falha.httpgo", "GET {{base_url}}\nHTTP 200\n[Asserts]\n"+tc.declaration+"\n")
			args := []string{"test", file, "--var", "base_url=" + server.URL, "--var", "expected=Pizza", "--var", "pattern=["}
			if tc.render {
				args = append(args, "--render-asserts")
			}
			code, out, stderr := run(t, args...)
			if code != tc.code || int(calls.Load()) != tc.calls || (code == 0 && stderr != "") || (code != 0 && !strings.Contains(stderr, ":4:")) {
				t.Fatalf("code=%d want=%d calls=%d want=%d out=%q stderr=%q", code, tc.code, calls.Load(), tc.calls, out, stderr)
			}
		})
	}
}
