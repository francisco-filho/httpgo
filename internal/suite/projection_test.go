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

// GO-458: compiled projections are reusable, but published values are file-local.
func TestProjectionFileIsolationAndSafeCallbacks(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, "01.hurl", "GET http://example.test/values\nHTTP 200\n[Asserts]\njsonpath \"$[*]\" count == 2\n[Captures]\nids: jsonpath \"$[*]\"\nPOST http://example.test/echo\n{{ids}}\nHTTP 204")
	fixture(t, dir, "02.hurl", "POST http://example.test/never\n{{ids}}")
	fixture(t, dir, "03.hurl", "GET http://example.test/last\nHTTP 200\n[Asserts]\njsonpath \"$[*]\" count == 2")
	const numbers = `[9007199254740992,9007199254740993]`
	calls := 0
	c, _ := template.New(nil, []string{"ids"})
	s := New(runner.NewWithRenderer(executeFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		calls++
		if req.Method == "POST" {
			if string(req.Body) != numbers {
				t.Errorf("body=%s", req.Body)
			}
			return runner.Response{Status: 204}, nil
		}
		return runner.Response{Status: 200, Body: []byte(numbers)}, nil
	}), c, nil))
	files, err := s.Discover(context.Background(), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		reports := 0
		got := s.Run(context.Background(), files, Options{}, func(_ FileResult, r runner.Result, _ error) error {
			reports++
			for _, entry := range r.Entries {
				for _, a := range entry.Entry.Assertions {
					if a.JSON != nil && !reflect.DeepEqual(a.JSON.Path, query.Path{}) {
						t.Fatal("compiled plan retained")
					}
				}
			}
			raw, _ := json.Marshal(r)
			if strings.Contains(string(raw), "900719925474099") {
				t.Fatalf("numeric secret leaked: %s", raw)
			}
			return nil
		})
		if got.Code != 2 || reports != 3 || got.Summary.Passed != 2 || got.Summary.Errors != 1 || got.Summary.Requests.Started != 3 {
			t.Fatalf("%+v", got)
		}
	}
	if calls != 6 {
		t.Fatalf("calls=%d", calls)
	}
}
