package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// GO-467–468,471,473: protect old/new/derived values in text and XML, even
// when the batch is discarded. Other files still resolve the original input.
func TestCaptureRenewalSecretSuiteByBinary(t *testing.T) {
	const old = "renewal-old-private"
	const fresh = "renewal-new-private"
	t.Setenv("HTTPGO_RENEWAL_SECRET", old)
	for _, mode := range []string{"success", "capture failure", "mismatch", "evaluation"} {
		for _, junit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/junit=%v", mode, junit), func(t *testing.T) {
				var mu sync.Mutex
				var paths []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					paths = append(paths, r.URL.Path)
					mu.Unlock()
					switch r.URL.Path {
					case "/capture":
						if r.Header.Get("X-Token") != old {
							t.Error("first request did not resolve initial")
						}
						value := "prefix-" + fresh + "-suffix"
						if mode == "mismatch" {
							value = "unrelated-private-actual"
						}
						w.Header().Set("X-Check", value)
						_, _ = io.WriteString(w, `{"token":"`+fresh+`"}`)
					case "/next":
						if r.Header.Get("X-Token") != fresh {
							t.Error("next entry did not resolve new value")
						}
					case "/isolated", "/" + fresh:
						if r.Header.Get("X-Token") != old {
							t.Error("file shared captured values")
						}
					default:
						t.Errorf("unexpected path %s", r.URL.Path)
					}
				}))
				defer server.Close()
				dir := t.TempDir()
				config := configFile(t, t.TempDir(), "config.yaml", "version: 1\nvariables:\n  token: ${HTTPGO_RENEWAL_SECRET}\n")
				source := "GET " + server.URL + "/capture\nX-Token: {{token}}\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\nfresh: jsonpath \"$.token\"\n"
				if mode == "capture failure" {
					source += "broken: jsonpath \"$.missing\"\n"
				}
				source += "[Asserts]\nheader \"X-Check\" == \"prefix-{{token}}-suffix\"\n"
				if mode == "evaluation" {
					source += "jsonpath \"$\" == {{missing}}\n"
				}
				source += "GET " + server.URL + "/next\nX-Token: {{token}}\nHTTP 200"
				configFile(t, dir, "01.hurl", source)
				configFile(t, dir, "02-"+fresh+".hurl", "GET "+server.URL+"/isolated\nX-Token: {{token}}\nHTTP 200")
				args := []string{"test", dir, "--config", config, "--render-asserts"}
				xmlPath := filepath.Join(t.TempDir(), "report.xml")
				if junit {
					args = append(args, "--output", "junit", "--output-file", xmlPath)
				}
				code, out, stderr := run(t, args...)
				wantCode := 2
				wantPaths := []string{"/capture", "/isolated"}
				if mode == "success" {
					wantCode = 0
					wantPaths = []string{"/capture", "/next", "/isolated"}
				}
				if mode == "mismatch" {
					wantCode = 1
				}
				var xmlText string
				if junit {
					doc, raw := junitXML(t, xmlPath)
					xmlText = raw
					if out != "" || doc.Tests != 2 || len(doc.Cases) != 2 || doc.Cases[1].Error != nil || doc.Cases[1].Failure != nil {
						t.Fatalf("XML=%s stdout=%s", raw, out)
					}
					if mode == "mismatch" && doc.Failures != 1 || wantCode == 2 && doc.Errors != 1 || wantCode == 0 && doc.Failures+doc.Errors != 0 {
						t.Fatalf("classification XML=%s", raw)
					}
				}
				mu.Lock()
				gotPaths := append([]string(nil), paths...)
				mu.Unlock()
				if code != wantCode || !reflect.DeepEqual(gotPaths, wantPaths) {
					t.Fatalf("code=%d paths=%v stdout=%s stderr=%s XML=%s", code, paths, out, stderr, xmlText)
				}
				for _, value := range []string{old, fresh, "prefix-" + fresh + "-suffix", "unrelated-private-actual", "$.token", "{{token}}"} {
					raw, _ := json.Marshal(value)
					for _, variant := range []string{value, string(raw)} {
						if strings.Contains(out+stderr+xmlText, variant) {
							t.Fatalf("secret or operational plan leaked: %q", variant)
						}
					}
				}
				// A separate invocation must not inherit the previous redaction state.
				visible := scenario(t, "GET "+server.URL+"/"+fresh+"\nX-Token: "+old+"\nHTTP 200")
				plainCode, plainOut, plainErr := run(t, "test", visible)
				if plainCode != 0 || plainErr != "" || !strings.Contains(plainOut, fresh) {
					t.Fatalf("fresh invocation: %d %s %s", plainCode, plainOut, plainErr)
				}
			})
		}
	}
}
