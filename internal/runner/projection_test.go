package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-324,329,330,334,336,337: existing predicates see only the reduced value.
func TestProjectionPredicatesAndOrder(t *testing.T) {
	for _, tc := range []struct {
		line, body string
		outcome    Outcome
	}{
		{`jsonpath "$[*].code" contains "REQUIRED"`, `[{"code":"INVALID"},{"code":"REQUIRED"}]`, Wildcard},
		{`jsonpath "$[*].code" not contains "OTHER_TENANT"`, `[{"code":"REQUIRED"},{"code":"OTHER_TENANT"}]`, JSONMismatch},
		{`jsonpath "$[*]" count == 2`, `[1,2]`, Wildcard},
		{`jsonpath "$[*]" count == 1`, `[1]`, JSONMismatch},
		{`jsonpath "$[*]" count == 0`, `[]`, JSONMismatch},
		{`jsonpath "$[*]" count == 2`, `[[1,2]]`, Wildcard},
		{`jsonpath "$[*]" not exists`, `[]`, Wildcard},
		{`jsonpath "$[*]" exists`, `[[]]`, Wildcard},
		{`jsonpath "$[*]" exists`, `[null]`, Wildcard},
		{`jsonpath "$[*]" contains "REQ"`, `["REQUIRED"]`, Wildcard},
		{`jsonpath "$[*]" contains "REQ"`, `["REQUIRED","INVALID"]`, JSONMismatch},
		{`jsonpath "$[*]" == [1,2]`, `[1,2]`, Wildcard},
		{`jsonpath "$[*]" matches "^x"`, `["x","y"]`, JSONMismatch},
		{`jsonpath "$[*]" matches "^x"`, `["xyz"]`, Wildcard},
		{`jsonpath "$[*]" > 1`, `[2]`, Wildcard},
		{`jsonpath "$[*]" > 1`, `[2,3]`, JSONMismatch},
		{`jsonpath "$[*].x" not exists`, `[{"x":1},null]`, JSONFailed},
		{`jsonpath "$[*]" not exists`, `{}`, JSONFailed},
		{`jsonpath "$[*]" not exists`, ``, JSONFailed},
		{`jsonpath "$[*]" not exists`, `[] []`, JSONFailed},
		{"header \"X\" == \"bad\"\njsonpath \"$[*]\" exists", `invalid`, HeaderMismatch},
		{"jsonpath \"$[*]\" exists\nheader \"X\" == \"bad\"", `invalid`, JSONFailed},
	} {
		t.Run(tc.line+tc.body, func(t *testing.T) {
			calls := 0
			got := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), nil).Run(context.Background(), parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+tc.line+"\nGET http://example.test/next"))
			wantCalls := 1
			if tc.outcome == Wildcard {
				wantCalls = 2
			}
			if calls != wantCalls || got.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v", calls, got)
			}
		})
	}
}

// GO-331,332: captures preserve precision, numeric spelling and literal strings.
func TestProjectionCaptureReuse(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `9007199254740993`, `9223372036854775809`, `1.0000000000000000000002`, `1e99999999999999999999999`, `1e-99999999999999999999999`, `1.2300e+99`, `"{{literal}}"`, `[]`, `{}`, `[9007199254740992,9007199254740993,9223372036854775809,1.00,1e-99999999999999999999999]`, `{"x":[1.2300e+99]}`} {
		for _, multiple := range []bool{false, true} {
			body, want := "["+raw+"]", raw
			if multiple {
				body = "[" + raw + "," + raw + "]"
				want = body
			}
			calls := 0
			got := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls == 1 {
					return Response{Status: 200, Body: []byte(body)}, nil
				}
				if string(req.Body) != want {
					t.Fatalf("body=%s want=%s", req.Body, want)
				}
				return Response{Status: 200}, nil
			}), nil).Run(context.Background(), parseCaptureScenario(t, "GET http://example.test\nHTTP 200\n[Captures]\nx: jsonpath \"$[*]\"\nPOST http://example.test/echo\n{{x}}\nHTTP 200"))
			if got.Err != nil || calls != 2 {
				t.Fatalf("%s multiple=%v: %+v", raw, multiple, got)
			}
		}
	}
	for _, next := range []string{"GET http://example.test/{{x}}", "GET http://example.test\nX: {{x}}", "POST http://example.test\n{\"x\":\"{{x}}\"}"} {
		calls := 0
		got := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(`[1,2]`)}, nil
		}), nil).Run(context.Background(), parseCaptureScenario(t, "GET http://example.test\n[Captures]\nx: jsonpath \"$[*]\"\n"+next))
		if calls != 1 || got.Err == nil || got.Entries[1].Outcome != RenderFailed {
			t.Fatalf("%+v", got)
		}
	}
}

// GO-333,334: earlier discovered values are protected, but the batch is atomic.
func TestProjectionCaptureAtomic(t *testing.T) {
	for _, declarations := range []string{
		"x: jsonpath \"$.items[*]\"\nmissing: jsonpath \"$.none[*]\"",
		"x: jsonpath \"$.items[*]\"\nmissing: jsonpath \"$.bad[*].x\"",
		"x: jsonpath \"$.items[*]\"\nx.child: jsonpath \"$.items[0]\"",
		"x: jsonpath \"$.items[*]\"\nprevious.child: jsonpath \"$.items[0]\"",
	} {
		c, _ := template.New(nil, []string{"x"})
		session := c.NewSession()
		if err := session.Publish([]template.Capture{{Name: "previous", Value: json.RawMessage(`7`)}}); err != nil {
			t.Fatal(err)
		}
		s := parseCaptureScenario(t, "GET http://example.test\n[Captures]\n"+declarations)
		failure := captureResponse(session, s.Entries[0].Captures, Response{Body: []byte(`{"items":["private-one","private-two"],"bad":[{"x":1},null]}`)})
		if failure == nil {
			t.Fatal("capture passed")
		}
		if _, err := session.Render("{{x}}"); err == nil {
			t.Fatal("partial publication")
		}
		prior, err := session.Render("{{previous}}")
		if err != nil || prior.Value != "7" {
			t.Fatal("prior value overwritten")
		}
		if session.Sanitize("private-one") != "[oculto]" {
			t.Fatal("discovered protection lost")
		}
		if strings.Contains(declarations, "$.bad") && !strings.Contains(failure.Detail, "percurso") {
			t.Fatal(failure)
		}
	}
}

func TestProjectionSafeFailure(t *testing.T) {
	for _, line := range []string{`jsonpath "$[*]" == ["expected-private"]`, `jsonpath "$[*]" matches "expected-private"`, `jsonpath "$[*]" count == 0`, `jsonpath "$[*]" not exists`} {
		for _, secret := range []string{"", "known", "pending"} {
			c, _ := template.New([]string{"token=actual-private"}, nil)
			if secret == "known" {
				c, _ = template.New([]string{"token=actual-private"}, []string{"token"})
			}
			source := "GET http://example.test\nHTTP *\n[Asserts]\n" + line
			if secret == "pending" {
				c, _ = template.New(nil, []string{"future"})
				source += "\n[Captures]\nfuture: jsonpath \"$\""
			}
			check := func(e EntryResult) error {
				raw, _ := json.Marshal(e)
				for _, text := range []string{"actual-private", "unrelated-private"} {
					if strings.Contains(string(raw), text) {
						t.Fatalf("leaked %s", raw)
					}
				}
				if secret != "" && (e.JSONFailure.Actual != "[oculto]" || strings.Contains(e.JSONFailure.Detail, "contagem")) {
					t.Fatalf("unprotected summary: %+v", e.JSONFailure)
				}
				if e.Entry.Assertions[0].JSON.Path.Expansive() {
					t.Fatal("operational query retained")
				}
				return nil
			}
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				return Response{Status: 200, Body: []byte(`["actual-private","unrelated-private"]`), Headers: []query.HeaderValue{}}, nil
			}), c, check).Run(context.Background(), parseCaptureScenario(t, source))
			if got.Err != nil || !got.Diverged {
				t.Fatalf("%+v", got)
			}
			_ = check(got.Entries[0])
		}
	}
}
