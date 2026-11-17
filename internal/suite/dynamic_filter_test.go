package suite

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-409,414: the profile applies per invocation, resolvable values stay per file,
// and category 2 outranks divergence without disclosing operational metadata.
func TestDynamicFilterSuiteIsolation(t *testing.T) {
	for _, fast := range []bool{false, true} {
		dir := t.TempDir()
		fixture(t, dir, "01.hurl", "GET http://example.test/capture\nHTTP 200\n[Captures]\nsaved: jsonpath \"$.key\"\nGET http://example.test/check\nHTTP 200\n[Asserts]\n"+`jsonpath "$.items[?(@.key=={{saved}})].id" == 1`)
		for _, f := range []struct{ name, operand, expected string }{{"02.hurl", "initial", "2"}, {"03.hurl", "saved", "1"}, {"04.hurl", "initial", "1"}} {
			fixture(t, dir, f.name, "GET http://example.test/check\nHTTP 200\n[Asserts]\njsonpath \"$.items[?(@.key=={{"+f.operand+"}})].id\" == "+f.expected)
		}
		c, _ := template.New([]string{`initial="fake-private"`}, []string{"initial", "saved"})
		calls := 0
		s := New(runner.NewWithRenderer(executeFunc(func(context.Context, runner.Request) (runner.Response, error) {
			calls++
			return runner.Response{Status: 200, Body: []byte(`{"key":"fake-private","items":[{"key":"fake-private","id":1}]}`)}, nil
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
				for _, bad := range []string{"fake-private", "{{saved}}", "{{initial}}"} {
					if strings.Contains(string(raw), bad) {
						t.Fatalf("unsafe callback: %s", raw)
					}
				}
				for _, e := range r.Entries {
					for _, a := range e.Entry.Assertions {
						if a.JSON != nil && (!reflect.DeepEqual(a.JSON.Path, query.Path{}) || a.JSON.ExpectedTemplate != nil || a.JSON.Expected != nil) {
							t.Fatal("operational metadata retained")
						}
					}
				}
				return nil
			})
			if fast {
				if got.Code != 1 || reports != 4 || got.Summary.Skipped != 2 || got.Summary.Requests.Started != 3 {
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
