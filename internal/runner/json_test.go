package runner

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

type invalidJSONRenderer struct{ *template.Context }

func (r invalidJSONRenderer) NewSession() template.Session {
	return invalidJSONRenderer{r.Context.NewSession().(*template.Context)}
}

func (r invalidJSONRenderer) RenderJSON([]byte) ([]byte, error) {
	return []byte(`{"invalid":secret}`), nil
}

func TestJSONRenderFailuresStopBeforeSend(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, mode := range []string{"missing", "object string", "invalid final"} {
			t.Run(mode+newline, func(t *testing.T) {
				source := strings.Join([]string{"GET http://example.test/first", "HTTP 200", "POST http://example.test/affected", "{", ` "escaped":"{{value}}",`, ` "next":{{missing}}`, "}", "GET http://example.test/last"}, newline)
				scenario, err := parser.Parse("safe.hurl", strings.NewReader(source))
				if err != nil {
					t.Fatal(err)
				}
				assignments := []string{`value="quotes\"\\\n雪"`}
				if mode == "object string" {
					assignments = []string{`value={"fake":"private"}`}
				}
				c, err := template.New(assignments, []string{"value"})
				if err != nil {
					t.Fatal(err)
				}
				var renderer Renderer = c
				wantPos := parser.Position{File: "safe.hurl", Line: 6, Column: 9}
				if mode == "object string" {
					wantPos.Line, wantPos.Column = 5, 13
				}
				if mode == "invalid final" {
					renderer = invalidJSONRenderer{c}
					wantPos.Line, wantPos.Column = 4, 1
				}
				calls, reports := 0, 0
				result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: 200}, nil }), renderer, func(entry EntryResult) error { reports++; return nil }).Run(context.Background(), scenario)
				var located *RenderError
				if !errors.As(result.Err, &located) || result.ErrorPosition != wantPos || calls != 1 || reports != 2 || result.Summary != (Summary{3, 1, 1, 1, 0, 2}) || result.Entries[1].Outcome != RenderFailed || strings.Contains(result.Err.Error(), "private") || strings.Contains(result.Err.Error(), "secret") {
					t.Fatalf("result=%+v calls=%d reports=%d wantPos=%+v", result, calls, reports, wantPos)
				}
			})
		}
	}
}

func TestJSONRenderedCloneAndIncrementalOrder(t *testing.T) {
	source := "POST http://example.test/first\n{\"x\":{{value}}}\nHTTP 201\nPOST http://example.test/last\n{{value}}"
	scenario, err := parser.Parse("original.hurl", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	before, err := parser.Parse("original.hurl", strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, assignment := range []string{`value={"a":[9007199254740993123456789,"{{literal}}"]}`, `value=null`} {
		c, _ := template.New([]string{assignment}, nil)
		calls, reports := 0, 0
		result := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
			if calls != reports {
				t.Fatal("next send preceded report")
			}
			calls++
			value := strings.TrimPrefix(assignment, "value=")
			want := value
			if calls == 1 {
				want = `{"x":` + value + `}`
			}
			if string(req.Body) != want {
				t.Fatalf("got=%s want=%s", req.Body, want)
			}
			req.Body[0] = 'X'
			return Response{Status: 201}, nil
		}), c, func(EntryResult) error { reports++; return nil }).Run(context.Background(), scenario)
		if result.Err != nil || calls != 2 || !reflect.DeepEqual(scenario, before) || result.Summary.WithoutValidation != 1 {
			t.Fatalf("result=%+v mutated=%v", result, !reflect.DeepEqual(scenario, before))
		}
	}
}
