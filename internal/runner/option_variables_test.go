package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-572–573/577: assignments precede their own request; captures and later
// assignments replace them persistently, but never mutate the initial context.
func TestOptionVariableFlow(t *testing.T) {
	s := dynamicScenario(t, `POST http://example.test/{{id}}
X-ID: {{id}}
[Options]
variable: id=discarded
variable: id=assigned
variable: expected=renewed
{"id":"{{id}}"}
HTTP 200
[Captures]
id: jsonpath "$.id"
[Asserts]
jsonpath "$.id" == {{expected}}
GET http://example.test/{{id}}
HTTP 200
GET http://example.test/{{id}}
[Options]
variable: id=final
HTTP 200
GET http://example.test/{{id}}
HTTP 200`)
	c, _ := template.New([]string{"id=initial"}, nil)
	for range 2 {
		calls, callbacks := 0, 0
		got := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
			want := []string{"assigned", "renewed", "final", "final"}[calls]
			if req.URL != "http://example.test/"+want {
				t.Fatalf("request=%+v want=%s", req, want)
			}
			if calls == 0 && (len(req.Headers) != 1 || req.Headers[0].Value != want || string(req.Body) != `{"id":"assigned"}`) {
				t.Fatalf("request=%+v", req)
			}
			calls++
			return Response{Status: 200, Body: []byte(`{"id":"renewed"}`)}, nil
		}), c, func(entry EntryResult) error {
			callbacks++
			if entry.Entry.Options.Variables != nil {
				t.Fatal("operational declarations in callback")
			}
			return nil
		}).Run(context.Background(), s)
		if got.Err != nil || got.Diverged || calls != 4 || callbacks != 4 {
			t.Fatalf("result=%+v calls=%d callbacks=%d", got, calls, callbacks)
		}
		for _, entry := range got.Entries {
			if entry.Entry.Options.Variables != nil {
				t.Fatal("operational declarations in result")
			}
		}
	}
	value, err := c.NewSession().Render("{{id}}")
	if err != nil || value.Value != "initial" || len(s.Entries[0].Options.Variables) != 3 {
		t.Fatalf("initial=%+v err=%v scenario mutated", value, err)
	}
}

func TestOptionVariableTypedJSON(t *testing.T) {
	for _, tc := range []struct{ literal, want string }{
		{"false", "false"}, {"null", "null"}, {"9007199254740993", "9007199254740993"},
		{"1.000000000000000001", "1.000000000000000001"}, {`"123"`, `"123"`}, {"", `""`},
		{`[1]`, `"[1]"`}, {`{"x":1}`, `"{\"x\":1}"`}, {`{{other}}`, `"{{other}}"`},
		{`${NAME}`, `"${NAME}"`}, {`newUuid()`, `"newUuid()"`},
	} {
		t.Run(tc.literal, func(t *testing.T) {
			s := pollingScenario(t, "POST http://example.test\n[Options]\nvariable: value="+tc.literal+"\n{{value}}\nHTTP 200")
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				if string(req.Body) != tc.want {
					t.Fatalf("body=%s want=%s", req.Body, tc.want)
				}
				return Response{Status: 200}, nil
			}), nil).Run(context.Background(), s)
			if got.Err != nil || got.Diverged {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

func TestOptionVariablesExternalAndAssertionOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "literal", true: "rendered"}[enabled], func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "payload.json"), []byte(`{"value":{{value}}}`), 0600); err != nil {
				t.Fatal(err)
			}
			source := "POST http://example.test\n[Options]\nvariable: value=42\nfile,payload.json;\nHTTP 200\n[Asserts]\njsonpath \"$.value\" == \"{{value}}\""
			s, err := parser.ParseWithOptions("external.hurl", strings.NewReader(source), parser.ParseOptions{RenderAsserts: enabled})
			if err != nil {
				t.Fatal(err)
			}
			s.OriginDir = dir
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				want := `{"value":{{value}}}`
				if enabled {
					want = `{"value":42}`
				}
				if string(req.Body) != want {
					t.Fatalf("body=%s want=%s", req.Body, want)
				}
				return Response{Status: 200, Body: []byte(`{"value":"42"}`)}, nil
			}), nil).RunWithOptions(context.Background(), s, Options{RenderFiles: enabled})
			if got.Err != nil || got.Diverged == enabled {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

// GO-580: publication is atomic and failures are preparation, not captures.
func TestOptionVariableConflict(t *testing.T) {
	for _, captured := range []bool{false, true} {
		source := "GET http://example.test\n[Options]\nvariable: fresh=value\nvariable: root=private\nHTTP 200\nGET http://example.test/never"
		initial := []string{"root.child=initial"}
		if captured {
			source = "GET http://example.test/seed\nHTTP 200\n[Captures]\nroot.child: jsonpath \"$.value\"\n" + source
			initial = nil
		}
		c, _ := template.New(initial, nil)
		session := c.NewSession()
		calls := 0
		got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(`{"value":"initial"}`)}, nil
		}), observedHeaderSession{session}, nil).Run(context.Background(), pollingScenario(t, source))
		var render *RenderError
		if !errors.As(got.Err, &render) || got.Diverged || calls != map[bool]int{false: 0, true: 1}[captured] || got.ErrorPosition.Line != map[bool]int{false: 4, true: 8}[captured] || strings.Contains(got.Err.Error(), "private") || got.Entries[len(got.Entries)-1].Diagnostic.Phase != "renderização" {
			t.Fatalf("result=%+v calls=%d", got, calls)
		}
		if _, err := session.Render("{{fresh}}"); err == nil {
			t.Fatal("partial publication")
		}
	}
}

func TestOptionVariableDynamicQuery(t *testing.T) {
	s := dynamicScenario(t, "GET http://example.test\n[Options]\nvariable: selected=chosen\nHTTP 200\n[Captures]\nid: jsonpath \"$[?(@.key=='{{selected}}')].id\"\n[Asserts]\njsonpath \"$[?(@.key=='{{selected}}')].id\" == {{id}}\nGET http://example.test/{{id}}")
	calls := 0
	got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
		if calls == 1 && req.URL != "http://example.test/42" {
			t.Fatalf("request=%+v", req)
		}
		calls++
		return Response{Status: 200, Body: []byte(`[{"key":"chosen","id":42}]`)}, nil
	}), nil).Run(context.Background(), s)
	if got.Err != nil || got.Diverged || calls != 2 {
		t.Fatalf("result=%+v calls=%d", got, calls)
	}
}

func TestOptionVariablesNotAnticipated(t *testing.T) {
	s := pollingScenario(t, "GET http://example.test/{{later}}\nGET http://example.test\n[Options]\nvariable: later=value")
	calls := 0
	got := New(executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		return Response{}, nil
	}), nil).Run(context.Background(), s)
	if got.Err == nil || calls != 0 {
		t.Fatalf("result=%+v calls=%d", got, calls)
	}
}
