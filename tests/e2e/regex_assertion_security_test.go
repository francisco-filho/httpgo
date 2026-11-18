package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-498–500: real terminal/JUnit output, discarded and renewed secrets,
// isolated files, accumulated protection and independent invocations.
func TestRegexAssertionSecretSuiteByBinary(t *testing.T) {
	const old = "regex-old-private"
	const fresh = "regex-new-private"
	t.Setenv("HTTPGO_REGEX_SECRET", old)
	for _, mode := range []string{"success", "mismatch", "absence", "evaluation"} {
		for _, junit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/junit=%v", mode, junit), func(t *testing.T) {
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					switch r.URL.Path {
					case "/capture":
						if r.Header.Get("X-Token") != old {
							t.Error("initial not used")
						}
						w.Header().Set("X-Token", fresh)
						text := "id=prefix-" + fresh + "-suffix"
						if mode == "mismatch" {
							text = "id=unrelated-private-response"
						}
						if mode == "absence" {
							text = "no group"
						}
						_, _ = io.WriteString(w, text)
					case "/next":
						if r.Header.Get("X-Token") != fresh {
							t.Error("capture not confirmed")
						}
					case "/isolated":
						if r.Header.Get("X-Token") != old {
							t.Error("file shared capture")
						}
						_, _ = io.WriteString(w, "id="+old)
					case "/" + fresh:
					default:
						t.Error(r.URL.Path)
					}
				}))
				defer server.Close()
				dir := t.TempDir()
				config := configFile(t, t.TempDir(), "config.yaml", "version: 1\nvariables:\n  token: ${HTTPGO_REGEX_SECRET}\n")
				decl := `regex "id=(.*)" == "prefix-{{token}}-suffix"`
				if mode == "evaluation" {
					decl = `regex "id=(.*)" matches "prefix-{{token}}-suffix["`
				}
				configFile(t, dir, "01.hurl", "GET "+server.URL+"/capture\nX-Token: {{token}}\nHTTP 200\n[Captures]\ntoken: header \"X-Token\"\n[Asserts]\n"+decl+"\nGET "+server.URL+"/next\nX-Token: {{token}}\nHTTP 200")
				configFile(t, dir, "02-"+fresh+".hurl", "GET "+server.URL+"/isolated\nX-Token: {{token}}\nHTTP 200\n[Asserts]\nregex \"id=(.*)\" == \"{{token}}\"")
				args := []string{"test", dir, "--config", config, "--render-asserts"}
				xmlPath := filepath.Join(t.TempDir(), "report.xml")
				if junit {
					args = append(args, "--output", "junit", "--output-file", xmlPath)
				}
				code, out, stderr := run(t, args...)
				wantCode, wantHits := 1, int32(2)
				if mode == "success" {
					wantCode, wantHits = 0, 3
				}
				if mode == "evaluation" {
					wantCode = 2
				}
				var xmlText string
				if junit {
					doc, raw := junitXML(t, xmlPath)
					xmlText = raw
					if out != "" || doc.Tests != 2 || len(doc.Cases) != 2 || doc.Cases[1].Error != nil || doc.Cases[1].Failure != nil || wantCode == 1 && doc.Failures != 1 || wantCode == 2 && doc.Errors != 1 || wantCode == 0 && doc.Failures+doc.Errors != 0 {
						t.Fatalf("XML=%s stdout=%s", raw, out)
					}
				}
				if code != wantCode || hits.Load() != wantHits {
					t.Fatalf("code=%d hits=%d out=%s err=%s xml=%s", code, hits.Load(), out, stderr, xmlText)
				}
				for _, bad := range []string{old, fresh, "prefix-" + fresh + "-suffix", "unrelated-private-response", "{{token}}", "id=(.*)"} {
					if strings.Contains(out+stderr+xmlText, bad) {
						t.Fatalf("unsafe output contains %q: %s %s %s", bad, out, stderr, xmlText)
					}
				}
				if mode == "absence" && !strings.Contains(stderr+xmlText, "grupo ausente") {
					t.Fatalf("absence not distinguished: %s %s", stderr, xmlText)
				}
				plainCode, plainOut, plainErr := run(t, "test", scenario(t, "GET "+server.URL+"/"+fresh+"\nHTTP 200"))
				if plainCode != 0 || plainErr != "" || !strings.Contains(plainOut, fresh) {
					t.Fatalf("independent invocation: %d %s %s", plainCode, plainOut, plainErr)
				}
			})
		}
	}
}
