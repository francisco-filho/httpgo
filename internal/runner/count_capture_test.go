package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

// GO-431,433–435,440–441: count the reduced value, never matches or a fallback.
func TestCountCaptureSelectionAndTypedReuse(t *testing.T) {
	for _, tc := range []struct{ name, path, body, want string }{
		{"empty root", "$", `[]`, "0"}, {"null elements", "$", `[null,1,null]`, "3"},
		{"simple", "$.items", `{"items":[true,null]}`, "2"},
		{"missing", "$.items", `{}`, ""}, {"null", "$", `null`, ""},
		{"string", "$", `"abc"`, ""}, {"object", "$", `{}`, ""},
		{"number", "$", `7`, ""}, {"bool", "$", `true`, ""},
		{"zero wildcard", "$[*]", `[]`, ""}, {"one scalar wildcard", "$[*]", `[1]`, ""},
		{"many wildcard", "$[*]", `[null,2,3]`, "3"},
		{"one array wildcard", "$[*]", `[[1,2,3]]`, "3"},
		{"one empty wildcard", "$[*]", `[[]]`, "0"},
		{"many arrays wildcard", "$[*]", `[[1,2,3],[]]`, "2"},
		{"zero filter", "$[?(@.ok == true)].data", `[{"ok":false,"data":[]}]`, ""},
		{"one scalar filter", "$[?(@.ok == true)].data", `[{"ok":true,"data":1}]`, ""},
		{"many filter", "$[?(@.ok == true)].data", `[{"ok":true,"data":null},{"ok":true,"data":1}]`, "2"},
		{"one array filter", "$[?(@.ok == true)].data", `[{"ok":true,"data":[1,2,3]}]`, "3"},
		{"empty array filter", "$[?(@.ok == true)].data", `[{"ok":true,"data":[]}]`, "0"},
		{"many arrays filter", "$[?(@.ok == true)].data", `[{"ok":true,"data":[1,2,3]},{"ok":true,"data":[]}]`, "2"},
		{"branch traversal", "$[*].data", `[{"data":[]},null]`, ""},
		{"empty response", "$", ``, ""}, {"malformed", "$", `[1,`, ""},
		{"extra JSON", "$", `[] []`, ""}, {"traversal", "$.items", `null`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\n[Captures]\nbaseline: jsonpath \""+tc.path+"\" count\nPOST http://example.test/{{baseline}}\nX-Count: {{baseline}}\n{\"n\":{{baseline}},\"text\":\"{{baseline}}\"}\nHTTP 200")
			c, _ := template.New(nil, nil)
			session := c.NewSession()
			calls := 0
			result := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if calls == 1 {
					if !req.RetainBody {
						t.Fatal("body not retained")
					}
					return Response{Status: 200, Body: []byte(tc.body)}, nil
				}
				want := fmt.Sprintf(`{"n":%s,"text":"%s"}`, tc.want, tc.want)
				if string(req.Body) != want || req.Headers[0].Value != tc.want || req.URL != "http://example.test/"+tc.want {
					t.Fatalf("request=%+v want body=%s", req, want)
				}
				return Response{Status: 200}, nil
			}), observedHeaderSession{session}, nil).Run(context.Background(), s)
			if tc.want == "" {
				var failure *CaptureError
				if calls != 1 || !errors.As(result.Err, &failure) || !failure.Count || failure.Query != s.Entries[0].Captures[0].Path.RedactLiterals(tc.path) || failure.Position.Line != 4 || result.Diverged {
					t.Fatalf("calls=%d result=%+v", calls, result)
				}
				if _, err := session.Render("{{baseline}}"); err == nil {
					t.Fatal("failed capture published")
				}
			} else if calls != 2 || result.Err != nil || result.Diverged {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
		})
	}
}

// GO-432,436: the five existing operators consume a numeric baseline.
func TestCountCaptureComparisonsAndOrder(t *testing.T) {
	for _, tc := range []struct {
		predicate, body string
		outcome         Outcome
	}{
		{"count == {{baseline}}", `[null,null]`, Passed},
		{"count == {{baseline}}", `[null,null,null]`, JSONMismatch},
		{"count > {{baseline}}", `[null,null]`, JSONMismatch},
		{"count > {{baseline}}", `[null,null,null]`, Passed},
		{"count >= {{baseline}}", `[null,null]`, Passed},
		{"count < {{baseline}}", `[null]`, Passed},
		{"count < {{baseline}}", `[null,null]`, JSONMismatch},
		{"count <= {{baseline}}", `[null,null]`, Passed},
	} {
		t.Run(tc.predicate+tc.body, func(t *testing.T) {
			s := dynamicScenario(t, "GET http://example.test/before\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\nGET http://example.test/after\nHTTP 200\n[Asserts]\njsonpath \"$\" "+tc.predicate+"\n[Captures]\nchecked: jsonpath \"$\" count\nGET http://example.test/end")
			c, _ := template.New(nil, nil)
			session := c.NewSession()
			calls := 0
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				body := tc.body
				if calls == 1 {
					body = `[1,2]`
				}
				return Response{Status: 200, Body: []byte(body)}, nil
			}), observedHeaderSession{session}, nil).Run(context.Background(), s)
			wantCalls := 3
			if tc.outcome != Passed {
				wantCalls = 2
			}
			if result.Err != nil || calls != wantCalls || result.Entries[1].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
			_, err := session.Render("{{checked}}")
			if (err == nil) != (tc.outcome == Passed) {
				t.Fatalf("capture after assertion err=%v", err)
			}
		})
	}
	for _, tc := range []struct {
		status     int
		assertions string
		outcome    Outcome
	}{
		{503, `jsonpath "$" count == 1`, Mismatch},
		{200, `jsonpath "$" count == 1`, CaptureFailed},
		{200, `jsonpath "$" count == {{baseline}}`, CaptureFailed},
	} {
		s := dynamicScenario(t, "GET http://example.test/\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$.missing\" count\n[Asserts]\n"+tc.assertions+"\nGET http://example.test/next")
		c, _ := template.New(nil, nil)
		session := c.NewSession()
		calls := 0
		result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: tc.status, Body: []byte(`[]`)}, nil
		}), observedHeaderSession{session}, nil).Run(context.Background(), s)
		if calls != 1 || result.Entries[0].Outcome != tc.outcome {
			t.Fatalf("calls=%d result=%+v", calls, result)
		}
		if _, err := session.Render("{{baseline}}"); err == nil {
			t.Fatal("capture evaluated before assertions")
		}
	}
}

// GO-437,439: protect discovered counts even when the entire batch is rejected.
func TestCountCaptureAtomicity(t *testing.T) {
	for _, tc := range []struct{ name, initial, declarations string }{
		{"missing other", "", "other: jsonpath \"$.missing\""},
		{"initial exact duplicate", "baseline=7", "baseline: jsonpath \"$\" count"}, {"initial parent", "baseline=7", "child: jsonpath \"$\" count\nbaseline.child: jsonpath \"$\" count"},
		{"initial descendant", "baseline.child=7", ""},
		{"previous exact duplicate", "", "previous: jsonpath \"$\" count\nprevious: jsonpath \"$\" count"},
		{"previous parent", "", "previous.child: jsonpath \"$\" count"},
		{"previous descendant", "", "old: jsonpath \"$\" count"},
		{"batch exact", "", "baseline: jsonpath \"$\" count"},
		{"batch parent", "", "baseline.child: jsonpath \"$\" count"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var initial []string
			if tc.initial != "" {
				initial = []string{tc.initial}
			}
			c, _ := template.New(initial, []string{"baseline"})
			session := c.NewSession()
			if err := session.Publish([]template.Capture{{Name: "previous", Value: json.RawMessage(`8`)}, {Name: "old.child", Value: json.RawMessage(`9`)}}); err != nil {
				t.Fatal(err)
			}
			s := parseCaptureScenario(t, "GET http://example.test/\n[Captures]\nbaseline: jsonpath \"$\" count\n"+tc.declarations)
			err := captureResponse(session, s.Entries[0].Captures, Response{Body: []byte(`[null,null]`)})
			if err == nil || session.Sanitize("2") != "[oculto]" {
				t.Fatalf("error=%v protection=%q", err, session.Sanitize("2"))
			}
			if v, err := session.Render("{{previous}}/{{old.child}}"); err != nil || v.Value != "8/9" {
				t.Fatalf("previous changed: %+v %v", v, err)
			}
			if tc.initial == "" {
				if _, err := session.Render("{{baseline}}"); err == nil {
					t.Fatal("partial publication")
				}
			}
			if tc.initial == "baseline=7" {
				if v, err := session.Render("{{baseline}}"); err != nil || v.Value != "7" {
					t.Fatalf("initial changed: %+v %v", v, err)
				}
			}
			if strings.Contains(fmt.Sprint(err), "null") {
				t.Fatal("collection in error")
			}
		})
	}
}
