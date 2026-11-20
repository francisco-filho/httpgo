package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func writeE2EFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	configFile(t, filepath.Dir(path), filepath.Base(path), text)
}

func TestOptionVariablesHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
		for _, token := range []string{"[Options]", "variable: nome=literal", "sem pontos", "outro arquivo", "sem expansão", "arrays/objetos são texto", "Marcas de segredo", "--render-files", "--render-asserts", "[Variables]"} {
			if !strings.Contains(out, token) {
				t.Fatalf("missing %q in help: %s", token, out)
			}
		}
	}
}

func optionFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/scenarios", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// GO-571–573: real directory execution overrides base/environment/CLI locally.
func TestOptionVariablesDirectoryByBinary(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || r.Method != "POST" || r.Header.Get("Variable") != "" {
			t.Errorf("request=%s err=%v", r.Method, err)
		}
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.WriteHeader(201)
	}))
	defer server.Close()
	dir := t.TempDir()
	for i, fixture := range []string{"scenario_option_variables_etl.httpgo", "scenario_option_variables_iagen.httpgo", "scenario_option_variables_initial.httpgo"} {
		writeE2EFile(t, filepath.Join(dir, "tests", fmt.Sprintf("%02d.httpgo", i)), optionFixture(t, fixture))
	}
	base := filepath.Join(dir, ".httpgo.yaml")
	writeE2EFile(t, base, "version: 1\nvariables: {tipo_validacao: 5, base_url: '"+server.URL+"'}")
	writeE2EFile(t, filepath.Join(dir, "environments", "local.yaml"), "variables: {tipo_validacao: 7}")
	code, out, stderr := run(t, "test", filepath.Join(dir, "tests"), "--config", base, "--env", "local", "--var", "tipo_validacao=9")
	mu.Lock()
	defer mu.Unlock()
	want := []string{`{"tipoValidacao": 1, "clienteExterno": false}`, `{"tipoValidacao": 3, "clienteExterno": true}`, `{"tipoValidacao": 9}`}
	if code != 0 || stderr != "" || len(bodies) != 3 || !strings.Contains(out, "Cenários executados: 3") || !strings.Contains(out, "Testes executados: 3") {
		t.Fatalf("code=%d bodies=%v out=%s err=%s", code, bodies, out, stderr)
	}
	for i := range want {
		if bodies[i] != want[i] {
			t.Fatalf("body[%d]=%s want=%s", i, bodies[i], want[i])
		}
	}
}

// GO-574–577: real wire types, opaque values, external rendering and dynamic
// filters/expecteds keep their independent opt-in boundaries.
func TestOptionVariableLiteralsByBinary(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, render := range []bool{false, true} {
			t.Run(fmt.Sprintf("external=%v/render=%v", external, render), func(t *testing.T) {
				payload := `{"n":{{n}},"decimal":{{decimal}},"flag":{{flag}},"nil":{{nil}},"code":{{code}},"empty":{{empty}},"array":{{array}},"object":{{object}},"ref":{{ref}},"env":{{env}},"function":{{function}}}`
				want := `{"n":9007199254740993,"decimal":1.000000000000000001,"flag":false,"nil":null,"code":"123","empty":"","array":"[1,true]","object":"{\"a\":1}","ref":"{{other}}","env":"${NAME}","function":"newUuid()"}`
				if external && !render {
					want = payload
				}
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != want || r.Header.Get("X-Code") != "123" || r.URL.Path != "/123" {
						t.Errorf("body=%s header=%s path=%s err=%v want=%s", body, r.Header.Get("X-Code"), r.URL.Path, err, want)
					}
					_, _ = io.WriteString(w, `[{"key":"selected","id":42}]`)
				}))
				defer server.Close()
				dir := t.TempDir()
				body := payload
				if external {
					writeE2EFile(t, filepath.Join(dir, "body.json"), payload)
					body = "file,body.json;"
				}
				source := "POST " + server.URL + "/{{code}}\nX-Code: {{code}}\n[Options]\nvariable: n=9007199254740993\nvariable: decimal=1.000000000000000001\nvariable: flag=false\nvariable: nil=null\nvariable: code=\"123\"\nvariable: empty=\nvariable: array=[1,true]\nvariable: object={\"a\":1}\nvariable: ref={{other}}\nvariable: env=${NAME}\nvariable: function=newUuid()\nvariable: selected=selected\n" + body + "\nHTTP 200\n[Captures]\nid: jsonpath \"$[?(@.key=='{{selected}}')].id\"\n[Asserts]\njsonpath \"$[0].id\" == {{id}}"
				file := filepath.Join(dir, "input.httpgo")
				writeE2EFile(t, file, source)
				args := []string{"test", file, "--render-asserts"}
				if render {
					args = append(args, "--render-files")
				}
				code, out, stderr := run(t, args...)
				if code != 0 || stderr != "" || calls.Load() != 1 {
					t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls.Load(), out, stderr)
				}
			})
		}
	}
}

func TestOptionVariablesPollingByBinary(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		want := "/assigned"
		if n == 3 {
			want = "/accepted"
		}
		if r.URL.Path != want {
			t.Errorf("path=%s want=%s", r.URL.Path, want)
		}
		_, _ = fmt.Fprintf(w, `{"id":"accepted","ready":%v}`, n >= 2)
	}))
	defer server.Close()
	source := "GET " + server.URL + "/{{id}}\n[Options]\nvariable: id=discarded\nvariable: id=assigned\nretry: 2\nretry-interval: 0\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"\n[Asserts]\njsonpath \"$.id\" == {{id}}\njsonpath \"$.ready\" == true\nGET " + server.URL + "/{{id}}\nHTTP 200"
	code, out, stderr := run(t, "test", scenario(t, source), "--render-asserts", "--var", "id=initial")
	if code != 0 || stderr != "" || calls.Load() != 3 || !strings.Contains(out, "[tentativas=2]") || !strings.Contains(out, "tentativas_iniciadas=3") || !strings.Contains(out, "Testes executados: 2") {
		t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls.Load(), out, stderr)
	}
}

func TestOptionVariableProtectionByBinary(t *testing.T) {
	t.Setenv("HTTPGO_OPTION_E2E_SECRET", "initial-private")
	for _, format := range []string{"verbose", "junit", "html"} {
		t.Run(format, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/assigned-private" || r.Header.Get("X-Derived") != "prefix-assigned-private-suffix" {
					t.Errorf("request=%s headers=%v", r.URL.Path, r.Header)
				}
				_, _ = io.WriteString(w, `{"echo":"initial-private discarded-private assigned-private future-private"}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			base := configFile(t, dir, "config.yaml", "version: 1\nvariables: {token: '${HTTPGO_OPTION_E2E_SECRET}'}")
			file := scenario(t, "GET "+server.URL+"/{{token}}\nX-Derived: prefix-{{token}}-suffix\n[Options]\nvariable: token=discarded-private\nvariable: token=assigned-private\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"different\"\nGET "+server.URL+"\n[Options]\nvariable: token=future-private")
			args := []string{"test", file, "--config", base}
			path := filepath.Join(dir, "report."+format)
			if format == "verbose" {
				args = append(args, "-vv")
			} else {
				args = append(args, "--output", format, "--output-file", path)
			}
			code, out, stderr := run(t, args...)
			if code != 1 || calls.Load() != 1 {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls.Load(), out, stderr)
			}
			if format == "html" {
				_, html := htmlArtifact(t, path)
				out += html
			}
			if format == "junit" {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				out += string(data)
			}
			for _, value := range []string{"initial-private", "discarded-private", "assigned-private", "future-private", "prefix-assigned-private-suffix"} {
				if strings.Contains(out+stderr, value) {
					t.Fatalf("secret %s exposed: out=%s err=%s", value, out, stderr)
				}
			}
		})
	}
}

func TestOptionVariableRejectionsByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, name := range []string{"invalid_option_variable_name.hurl", "invalid_option_variable_assignment.hurl", "invalid_option_variable_literal.hurl", "unsupported_option_variable_query.hurl", "unsupported_option_variables_section.hurl"} {
		code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(optionFixture(t, name), "http://example.invalid", server.URL)))
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":4:1:") && !strings.Contains(stderr, ":5:1:") || strings.Contains(stderr, "private") {
			t.Fatalf("fixture=%s code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
	// A valid option conflicting with an initial descendant fails at preparation,
	// not parsing; its preceding entry may run, but neither the batch nor next HTTP.
	code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"\n[Options]\nvariable: root=private\nHTTP 200\nGET "+server.URL), "--var", "root.child=initial")
	if code != 2 || hits.Load() != 0 || !strings.Contains(stderr, ":3:1:") || strings.Contains(stderr, "private") || strings.Contains(stderr, "erro de captura") {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}
