package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

// GO-406: none of the provisional batch is resolvable, and protection survives
// failed resolution, selection and publication without overwriting prior values.
func TestDynamicFilterCaptureAtomicity(t *testing.T) {
	for _, late := range []string{
		`next: jsonpath "$.items[?(@.id=={{ids}})].id"`,
		`next: jsonpath "$.items[?(@.id=={{future}})].id"`,
		`next: jsonpath "$.items[?(@.id=={{own}})].id"`,
		`next: jsonpath "$.items[?(@.id=={{key}})].absent"`,
		`next: jsonpath "$.bad[?(@.id.x=={{key}})]"`,
		`ids.child: jsonpath "$.items[?(@.key=={{key}})].id"`,
		`previous: jsonpath "$.items[?(@.key=={{key}})].id"`,
	} {
		t.Run(late, func(t *testing.T) {
			c, _ := template.New([]string{`key="private-key"`}, []string{"key"})
			session := c.NewSession()
			if err := session.Publish([]template.Capture{{Name: "previous", Value: json.RawMessage(`7`)}}); err != nil {
				t.Fatal(err)
			}
			source := "GET http://example.test\n[Captures]\n" + `ids: jsonpath "$.items[?(@.key=={{key}})].id"` + "\n" + late
			s := dynamicScenario(t, source)
			failure := captureResponse(session, s.Entries[0].Captures, Response{Body: []byte(`{"items":[{"key":"private-key","id":"discovered-private"}],"bad":[{"id":{"x":"private-key"}},{"id":null}]}`)})
			if failure == nil {
				t.Fatal("batch passed")
			}
			for _, name := range []string{"ids", "next"} {
				if _, err := session.RenderJSON([]byte("{{" + name + "}}")); err == nil {
					t.Fatalf("partial publication: %s", name)
				}
			}
			prior, err := session.RenderJSON([]byte(`{{previous}}`))
			if err != nil || string(prior) != "7" {
				t.Fatalf("prior=%s err=%v", prior, err)
			}
			if session.Sanitize("discovered-private") != "[oculto]" {
				t.Fatal("lost protection of derived capture")
			}
		})
	}
}

// GO-397,407: selections retain null, precision and collections; textual reuse
// never picks a first element or serializes an array implicitly.
func TestDynamicFilterCaptureReuse(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `9007199254740993`, `9223372036854775809`, `1.0000000000000000000002`, `1e999999999999999999999`, `"{{literal}}"`, `[]`, `{}`, `[1,2]`} {
		for _, multiple := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/plural=%v", raw, multiple), func(t *testing.T) {
				c, _ := template.New([]string{"key=true"}, nil)
				body := `[{"key":true,"v":` + raw + `}]`
				want := raw
				if multiple {
					body = `[{"key":true,"v":` + raw + `},{"key":true,"v":` + raw + `}]`
					want = "[" + raw + "," + raw + "]"
				}
				calls := 0
				s := dynamicScenario(t, "GET http://example.test\nHTTP 200\n[Captures]\n"+`value: jsonpath "$[?(@.key=={{key}})].v"`+"\nPOST http://example.test/echo\n{{value}}\nHTTP 200")
				result := NewWithRenderer(executorFunc(func(_ context.Context, r Request) (Response, error) {
					calls++
					if calls == 2 && string(r.Body) != want {
						t.Fatalf("body=%s want=%s", r.Body, want)
					}
					return Response{Status: 200, Body: []byte(body)}, nil
				}), c, nil).Run(context.Background(), s)
				if calls != 2 || result.Err != nil {
					t.Fatalf("calls=%d result=%+v", calls, result)
				}
			})
		}
	}
	for _, next := range []string{"GET http://example.test/{{ids}}", "GET http://example.test\nX-Ids: {{ids}}"} {
		c, _ := template.New([]string{"key=1"}, nil)
		calls := 0
		result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(`[{"key":1,"id":"one"},{"key":1,"id":"two"}]`)}, nil
		}), c, nil).Run(context.Background(), dynamicScenario(t, "GET http://example.test\n[Captures]\n"+`ids: jsonpath "$[?(@.key=={{key}})].id"`+"\n"+next))
		if calls != 1 || result.Err == nil || result.Entries[1].Outcome != RenderFailed {
			t.Fatalf("calls=%d result=%+v", calls, result)
		}
	}
}

// GO-404–405,407,411: all operands bind before selection, predicates see the
// reduced value, and malformed input/strict traversal never approve negation.
func TestDynamicFilterPredicatesAndFailures(t *testing.T) {
	for _, tc := range []struct {
		path, predicate, body string
		outcome               Outcome
	}{
		{`$[?(@.x=={{v}})].v`, `count == 2`, `[{"x":1,"v":1},{"x":1,"v":2}]`, Passed},
		{`$[?(@.x=={{v}})].v`, `count == 1`, `[{"x":1,"v":1}]`, JSONMismatch},
		{`$[?(@.x=={{v}})].v`, `count == 2`, `[{"x":1,"v":[1,2]}]`, Passed},
		{`$[?(@.x=={{v}})].v`, `== null`, `[]`, JSONMismatch},
		{`$[?(@.x=={{v}})].v`, `== null`, `[{"x":1,"v":null}]`, Passed},
		{`$[?(@.x=={{v}})].v`, `not exists`, `[]`, Passed},
		{`$[?(@.x=={{v}})].v`, `contains 2`, `[{"x":1,"v":1},{"x":1,"v":2}]`, Passed},
		{`$[?(@.x=={{v}})].v`, `== [1,2]`, `[{"x":1,"v":1},{"x":1,"v":2}]`, Passed},
		{`$[?(@.x=={{v}})].v`, `not exists`, ``, JSONFailed},
		{`$[?(@.x=={{v}})].v`, `not exists`, `[] []`, JSONFailed},
		{`$[?(@.x=={{v}})].v`, `not exists`, `[`, JSONFailed},
		{`$[?(@.x.y=={{v}})].v`, `not exists`, `[{"x":{"y":1},"v":2},{"x":null}]`, JSONFailed},
		{`$.groups[*].items[?(@.x=={{v}})].v`, `not exists`, `{"groups":[{"items":[{"x":1,"v":2}]},{"items":null}]}`, JSONFailed},
		{`$[?(@.x=={{v}})].items[?(@.y=={{missing}})]`, `not exists`, `[]`, JSONFailed},
	} {
		t.Run(tc.path+tc.body+tc.predicate, func(t *testing.T) {
			c, _ := template.New([]string{"v=1"}, nil)
			calls := 0
			s := dynamicScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\njsonpath "+strconv.Quote(tc.path)+" "+tc.predicate+"\nGET http://example.test/next")
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), c, nil).Run(context.Background(), s)
			want := 1
			if tc.outcome == Passed {
				want = 2
			}
			if calls != want || result.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v err=%v", calls, result, result.Err)
			}
			if strings.Contains(tc.path, "missing") && (result.Err == nil || !strings.Contains(result.Err.Error(), "variável ausente")) {
				t.Fatal(result.Err)
			}
		})
	}
}

func TestDynamicFilterOwnCaptureUnavailable(t *testing.T) {
	for _, assertion := range []bool{false, true} {
		declaration := "[Captures]\n" + `own: jsonpath "$[?(@.x=={{own}})].id"`
		if assertion {
			declaration = "[Asserts]\n" + `jsonpath "$[?(@.x=={{own}})].id" exists` + "\n[Captures]\nown: jsonpath \"$\""
		}
		calls := 0
		result := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(`[]`)}, nil
		}), nil).Run(context.Background(), dynamicScenario(t, "GET http://example.test\nHTTP 200\n"+declaration+"\nGET http://example.test/never"))
		if calls != 1 || result.Err == nil || !strings.Contains(result.Err.Error(), "variável ausente") {
			t.Fatalf("calls=%d result=%+v err=%v", calls, result, result.Err)
		}
	}
}
