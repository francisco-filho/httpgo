package suite

import (
	"context"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

func TestRegexAssertionFileIsolation(t *testing.T) {
	dir := t.TempDir()
	fixture(t, dir, "01.hurl", "GET http://example.test/first\nHTTP 200\n[Captures]\nid: header \"X-ID\"\n[Asserts]\nregex \"(.*)\" == \"{{id}}\"")
	fixture(t, dir, "02.hurl", "GET http://example.test/second\nHTTP 200\n[Asserts]\nregex \"(.*)\" == \"{{id}}\"")
	c, _ := template.New([]string{"id=old"}, nil)
	calls := 0
	s := New(runner.NewWithRenderer(executeFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		calls++
		if req.URL == "http://example.test/first" {
			return runner.Response{Status: 200, Body: []byte("fresh"), Headers: []query.HeaderValue{{Name: "X-ID", Value: "fresh"}}}, nil
		}
		return runner.Response{Status: 200, Body: []byte("old")}, nil
	}), c, nil))
	// A confirmed renewal in the first file cannot replace the second's initial.
	files, err := s.Discover(context.Background(), []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		got := s.Run(context.Background(), files, Options{Request: runner.Options{RenderAsserts: true}}, nil)
		if got.Code != 0 || got.Summary.Errors != 0 || got.Summary.Passed != 2 {
			t.Fatalf("%+v", got)
		}
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}
