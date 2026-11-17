package suite

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-389,390: independent sessions, suite ordering/priority and sanitized file callbacks.
func TestRenderAssertsSuiteIsolation(t *testing.T) {
	for _, fast := range []bool{false, true} {
		dir := t.TempDir()
		fixture(t, dir, "01.hurl", "GET http://example.test/capture\nHTTP 200\n[Captures]\nsaved: jsonpath \"$\"\nGET http://example.test/check\nHTTP 200\n[Asserts]\njsonpath \"$\" == {{saved}}")
		fixture(t, dir, "02.hurl", "GET http://example.test/mismatch\nHTTP 200\n[Asserts]\njsonpath \"$\" == {{initial}}")
		fixture(t, dir, "03.hurl", "GET http://example.test/absent\nHTTP 200\n[Asserts]\njsonpath \"$\" == {{saved}}")
		fixture(t, dir, "04.hurl", "GET http://example.test/last\nHTTP 200\n[Asserts]\njsonpath \"$\" == {{initial}}")
		c, _ := template.New([]string{`initial="fake-private"`}, []string{"initial", "saved"})
		calls := 0
		s := New(runner.NewWithRenderer(executeFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
			calls++
			body := `"fake-private"`
			if strings.HasSuffix(r.URL, "/mismatch") {
				body = `"partial-private"`
			}
			return runner.Response{Status: 200, Body: []byte(body)}, nil
		}), c, nil))
		files, err := s.Discover(context.Background(), []string{dir})
		if err != nil {
			t.Fatal(err)
		}
		for range 2 {
			reports := 0
			got := s.Run(context.Background(), files, Options{Request: runner.Options{RenderAsserts: true}, FailFast: fast}, func(f FileResult, r runner.Result, loadErr error) error {
				reports++
				if f.State == Skipped {
					return nil
				}
				if loadErr != nil {
					t.Fatal(loadErr)
				}
				if reports == 3 && (r.Err == nil || !strings.Contains(r.Err.Error(), "variável ausente")) {
					t.Fatalf("capture shared: %+v", r)
				}
				raw, _ := json.Marshal(r)
				for _, bad := range []string{"fake-private", "partial-private", "{{saved}}", "{{initial}}"} {
					if strings.Contains(string(raw), bad) {
						t.Fatalf("unsafe callback: %s", raw)
					}
				}
				for _, entry := range r.Entries {
					for _, a := range entry.Entry.Assertions {
						if a.JSON != nil && (a.JSON.ExpectedTemplate != nil || a.JSON.Compiled != nil || a.JSON.Expected != nil) {
							t.Fatal("operational expected retained")
						}
					}
				}
				return nil
			})
			if fast {
				if got.Code != 1 || reports != 4 || got.Summary.Skipped != 2 || got.Summary.Errors != 0 || got.Summary.Requests.Started != 3 {
					t.Fatalf("result=%+v", got)
				}
			} else if got.Code != 2 || reports != 4 || got.Summary.Errors != 1 || got.Summary.Failed != 1 || got.Summary.Passed != 2 || got.Summary.Requests.Started != 5 {
				t.Fatalf("result=%+v", got)
			}
		}
		want := 10
		if fast {
			want = 6
		}
		if calls != want {
			t.Fatalf("calls=%d", calls)
		}
	}
}
