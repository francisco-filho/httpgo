package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-349–350: capture reduction preserves every JSON type and numeric spelling;
// a dependent textual request cannot silently choose one of multiple IDs.
func TestFilterCaptureTypedReuse(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `9007199254740993`, `9223372036854775809`, `1.0000000000000000000002`, `1e-99999999999999999999999`, `1e99999999999999999999999`, `1.2300e+99`, `"{{literal}}"`, `[]`, `{}`, `[1,2]`, `{"x":[1,null]}`} {
		for _, multiple := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/multiple=%v", raw, multiple), func(t *testing.T) {
				body := `[{"selected":true,"v":` + raw + `},{"selected":false,"v":"wrong"}]`
				want := raw
				if multiple {
					body = `[{"selected":true,"v":` + raw + `},{"selected":true,"v":` + raw + `}]`
					want = "[" + raw + "," + raw + "]"
				}
				calls := 0
				result := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
					calls++
					if calls == 1 {
						return Response{Status: 200, Body: []byte(body)}, nil
					}
					if string(req.Body) != want {
						t.Fatalf("body=%s want=%s", req.Body, want)
					}
					return Response{Status: 200}, nil
				}), nil).Run(context.Background(), parseCaptureScenario(t, "GET http://example.test\nHTTP 200\n[Captures]\nx: jsonpath \"$[?(@.selected==true)].v\"\nPOST http://example.test/echo\n{{x}}\nHTTP 200"))
				if result.Err != nil || calls != 2 {
					t.Fatalf("calls=%d result=%+v", calls, result)
				}
			})
		}
	}
	for _, next := range []string{"GET http://example.test/{{ids}}", "GET http://example.test\nX-Ids: {{ids}}", "POST http://example.test\n{\"ids\":\"{{ids}}\"}"} {
		calls := 0
		result := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(`[{"x":1,"id":"p1"},{"x":1,"id":"p2"}]`)}, nil
		}), nil).Run(context.Background(), parseCaptureScenario(t, "GET http://example.test\n[Captures]\nids: jsonpath \"$[?(@.x==1)].id\"\n"+next))
		if calls != 1 || result.Err == nil || result.Entries[1].Outcome != RenderFailed {
			t.Fatalf("calls=%d result=%+v", calls, result)
		}
	}
}

// GO-354–355: failure in the batch never publishes or overwrites previous values,
// but protection for discovered secret leaves survives the discarded batch.
func TestFilterCaptureAtomicAndProtected(t *testing.T) {
	for _, late := range []string{"missing: jsonpath \"$.missing\"", "bad: jsonpath \"$.bad[?(@.x.y==1)]\"", "ids.child: jsonpath \"$.values[0].id\"", "previous: jsonpath \"$.values[0].id\"", "previous.child: jsonpath \"$.values[0].id\""} {
		t.Run(late, func(t *testing.T) {
			c, _ := template.New(nil, []string{"ids"})
			session := c.NewSession()
			if err := session.Publish([]template.Capture{{Name: "previous", Value: json.RawMessage(`7`)}}); err != nil {
				t.Fatal(err)
			}
			s := parseCaptureScenario(t, "GET http://example.test\n[Captures]\nids: jsonpath \"$.values[?(@.selected==true)].id\"\n"+late)
			failure := captureResponse(session, s.Entries[0].Captures, Response{Body: []byte(`{"values":[{"selected":true,"id":9223372036854775809},{"selected":true,"id":"discovered-private"}],"bad":[{"x":{"y":1}},{"x":null}]}`)})
			if failure == nil {
				t.Fatal("batch passed")
			}
			if _, err := session.RenderJSON([]byte(`{{ids}}`)); err == nil {
				t.Fatal("partial batch published")
			}
			prior, err := session.RenderJSON([]byte(`{{previous}}`))
			if err != nil || string(prior) != "7" {
				t.Fatalf("prior=%s err=%v", prior, err)
			}
			for _, leaf := range []string{"discovered-private", "9223372036854775809"} {
				if session.Sanitize(leaf) != "[oculto]" {
					t.Fatalf("lost protection: %s", leaf)
				}
			}
		})
	}
}

// GO-351,353,357: predicates apply to the reduced value and status/assertions
// keep written order, even with invalid JSON or a pending capture.
func TestFilterPredicatesAndOrder(t *testing.T) {
	for _, tc := range []struct {
		lines, body string
		outcome     Outcome
	}{
		{`jsonpath "$[?(@.x==1)].v" count == 2`, `[{"x":1,"v":1},{"x":1,"v":2}]`, Wildcard},
		{`jsonpath "$[?(@.x==1)].v" count == 1`, `[{"x":1,"v":1}]`, JSONMismatch},
		{`jsonpath "$[?(@.x==1)].v" count == 2`, `[{"x":1,"v":[1,2]}]`, Wildcard},
		{`jsonpath "$[?(@.x==1)].v" count == 0`, `[]`, JSONMismatch},
		{`jsonpath "$[?(@.x==1)].v" == null`, `[]`, JSONMismatch},
		{`jsonpath "$[?(@.x==1)].v" == null`, `[{"x":1,"v":null}]`, Wildcard},
		{`jsonpath "$[?(@.x==1)].v" not exists`, `[]`, Wildcard},
		{`jsonpath "$[?(@.x==1)].v" exists`, `[{"x":1,"v":[]}]`, Wildcard},
		{`jsonpath "$[?(@.x==1)].v" == [1,2]`, `[{"x":1,"v":1},{"x":1,"v":2}]`, Wildcard},
		{`jsonpath "$[?(@.x==1)].v" > 0`, `[{"x":1,"v":1},{"x":1,"v":2}]`, JSONMismatch},
		{`jsonpath "$[?(@.x.y==1)].v" not exists`, `[{"x":{"y":1},"v":9},{"x":null}]`, JSONFailed},
		{"header \"X\" == \"bad\"\njsonpath \"$[?(@.x==1)]\" exists", `invalid`, HeaderMismatch},
		{"body contains \"absent\"\njsonpath \"$[?(@.x==1)]\" exists", `invalid`, BodyMismatch},
		{"jsonpath \"$[?(@.x==1)]\" exists\nheader \"X\" == \"bad\"", `invalid`, JSONFailed},
	} {
		t.Run(tc.lines+tc.body, func(t *testing.T) {
			calls := 0
			source := "GET http://example.test\nHTTP *\n[Asserts]\n" + tc.lines + "\n[Captures]\ncopy: jsonpath \"$\"\nPOST http://example.test\n{{copy}}\nHTTP 200"
			result := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls == 2 {
					got, err := query.Decode(req.Body)
					want, _ := query.Decode([]byte(tc.body))
					if err != nil || !query.JSONEqual(want, got) {
						t.Fatalf("copy=%s err=%v", req.Body, err)
					}
				}
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), nil).Run(context.Background(), parseCaptureScenario(t, source))
			want := 1
			if tc.outcome == Wildcard {
				want = 2
			}
			if calls != want || result.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
		})
	}
	for _, body := range []string{`[]`, `[{"x":0}]`} {
		calls := 0
		result := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(body)}, nil
		}), nil).Run(context.Background(), parseCaptureScenario(t, "GET http://example.test\n[Captures]\nx: jsonpath \"$[?(@.x==1)]\"\nGET http://example.test/never"))
		if calls != 1 || result.Err == nil || result.Entries[0].Outcome != CaptureFailed || !strings.Contains(result.Err.Error(), "não encontrou") {
			t.Fatalf("calls=%d result=%+v", calls, result)
		}
	}
}
