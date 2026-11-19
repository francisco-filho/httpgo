package suite

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-444: count values are local to a file; protection remains invocation-wide.
func TestCountCaptureSuiteIsolation(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, "01.hurl", "GET http://example.test/before\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\nGET http://example.test/check\nHTTP 200\n[Asserts]\njsonpath \"$\" count == {{baseline}}")
	fixture(t, dir, "02.hurl", "GET http://example.test/after\nHTTP 200\n[Asserts]\njsonpath \"$\" count == {{baseline}}\nGET http://example.test/never")
	fixture(t, dir, "03.hurl", "GET http://example.test/last\nHTTP 200")
	c, _ := template.New(nil, []string{"baseline"})
	calls := 0
	s := New(runner.NewWithRenderer(executeFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200, Body: []byte(`[1,2,3]`)}, nil
	}), c, nil))
	files, err := s.Discover(context.Background(), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		reports := 0
		got := s.Run(context.Background(), files, Options{Request: runner.Options{RenderAsserts: true}}, func(f FileResult, r runner.Result, loadErr error) error {
			reports++
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if reports == 2 && (r.Err == nil || !strings.Contains(r.Err.Error(), "variável ausente")) {
				t.Fatalf("shared baseline: %+v", r)
			}
			for _, e := range r.Entries {
				if len(e.Entry.Captures) != 0 || len(e.Entry.Body) != 0 {
					t.Fatalf("operational entry: %+v", e.Entry)
				}
				for _, a := range e.Entry.Assertions {
					if a.JSON != nil && (a.JSON.ExpectedTemplate != nil || a.JSON.Expected != nil) {
						t.Fatal("operational expected")
					}
				}
			}
			raw, _ := json.Marshal(r)
			if strings.Contains(string(raw), "{{baseline}}") || strings.Contains(string(raw), "[1,2,3]") {
				t.Fatalf("unsafe: %s", raw)
			}
			return nil
		})
		if got.Code != 2 || reports != 3 || got.Summary.Errors != 1 || got.Summary.Passed != 2 || got.Summary.Requests.Started != 4 {
			t.Fatalf("result=%+v", got)
		}
	}
	if calls != 8 {
		t.Fatalf("calls=%d", calls)
	}
}
