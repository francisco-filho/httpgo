package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// GO-459: real binary, real HTTP and both section orders.
func TestCaptureRenewalDemonstration(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scenarios", "scenario_capture_renewal.hurl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, inverted := range []bool{false, true} {
		for _, ready := range []bool{false, true} {
			t.Run(fmt.Sprintf("inverted=%v/ready=%v", inverted, ready), func(t *testing.T) {
				var mu sync.Mutex
				var paths []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					paths = append(paths, r.URL.Path)
					mu.Unlock()
					switch r.URL.Path {
					case "/jobs/current":
						status := "PENDING"
						if ready {
							status = "READY"
						}
						_, _ = fmt.Fprintf(w, `{"selectedId":"job-a","jobs":[{"id":"inicial","status":"READY"},{"id":"job-a","status":%q}]}`, status)
					case "/jobs/job-a/successor":
						_, _ = io.WriteString(w, `{"id":"job-b","status":"READY"}`)
					case "/jobs/job-b":
					default:
						t.Errorf("unexpected path %s", r.URL.Path)
						w.WriteHeader(404)
					}
				}))
				defer server.Close()
				source := string(data)
				if inverted {
					source = strings.Replace(source, "[Captures]\njob_id: jsonpath \"$.selectedId\"\n[Asserts]\njsonpath \"$.jobs[?(@.id=='{{job_id}}')].status\" == \"READY\"", "[Asserts]\njsonpath \"$.jobs[?(@.id=='{{job_id}}')].status\" == \"READY\"\n[Captures]\njob_id: jsonpath \"$.selectedId\"", 1)
				}
				code, out, stderr := run(t, "test", scenario(t, source), "--render-asserts", "--var", "base_url="+server.URL, "--var", "job_id=inicial")
				wantCode := 1
				wantPaths := []string{"/jobs/current"}
				if ready {
					wantCode = 0
					wantPaths = append(wantPaths, "/jobs/job-a/successor", "/jobs/job-b")
				}
				mu.Lock()
				defer mu.Unlock()
				if code != wantCode || !reflect.DeepEqual(paths, wantPaths) || ready && stderr != "" || !ready && !strings.Contains(stderr, "assertion") || !strings.Contains(out, "resumo") {
					t.Fatalf("code=%d paths=%v stdout=%s stderr=%s", code, paths, out, stderr)
				}
			})
		}
	}
}
