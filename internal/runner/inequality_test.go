package runner

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-419–424: reached expected precedes selection; captures publish only on success.
func TestJSONInequalityOrderAndPublication(t *testing.T) {
	for _, tc := range []struct {
		name, assertions, body, value, capture, detail string
		status                                         int
		outcome                                        Outcome
	}{
		{"different", `jsonpath "$" != {{v}}`, `2`, `1`, `checked: jsonpath "$"`, "", 200, Passed},
		{"equal", `jsonpath "$" != {{v}}`, `1.0`, `1`, `checked: jsonpath "$"`, "", 200, JSONMismatch},
		{"missing", `jsonpath "$.missing" != null`, `{}`, `1`, `checked: jsonpath "$"`, "", 200, JSONMismatch},
		{"index", `jsonpath "$[2]" != null`, `[]`, `1`, `checked: jsonpath "$"`, "", 200, JSONMismatch},
		{"decode", `jsonpath "$" != {{v}}`, `bad`, `1`, `checked: jsonpath "$"`, "resposta não contém", 200, JSONFailed},
		{"traversal", `jsonpath "$.x.y" != {{v}}`, `{"x":1}`, `1`, `checked: jsonpath "$"`, "percurso", 200, JSONFailed},
		{"expected before decode", `jsonpath "$" != {{missing}}`, `bad`, `1`, `checked: jsonpath "$"`, "variável ausente", 200, JSONFailed},
		{"expected before absence", `jsonpath "$.missing" != {{missing}}`, `{}`, `1`, `checked: jsonpath "$"`, "variável ausente", 200, JSONFailed},
		{"own capture", `jsonpath "$" != {{checked}}`, `1`, `1`, `checked: jsonpath "$"`, "variável ausente", 200, JSONFailed},
		{"future capture", `jsonpath "$" != {{future}}`, `1`, `1`, `checked: jsonpath "$"`, "variável ausente", 200, JSONFailed},
		{"status", `jsonpath "$" != {{missing}}`, `bad`, `1`, `checked: jsonpath "$"`, "", 503, Mismatch},
		{"first assertion", "jsonpath \"$\" != 1\njsonpath \"$\" != {{missing}}", `1`, `1`, `checked: jsonpath "$"`, "", 200, JSONMismatch},
		{"capture error", `jsonpath "$" != 2`, `1`, `1`, "checked: jsonpath \"$\"\nbroken: jsonpath \"$.missing\"", "", 200, CaptureFailed},
		{"no recursion", `jsonpath "$" != {{v}}`, `"{{missing}}"`, `"{{missing}}"`, `checked: jsonpath "$"`, "", 200, JSONMismatch},
		{"escapes", `jsonpath "$" != "{{v}}"`, `"a\"\\\n雪"`, `"a\"\\\n雪"`, `checked: jsonpath "$"`, "", 200, JSONMismatch},
		{"typed", `jsonpath "$" != {{v}}`, `9007199254740992`, `9007199254740993`, `checked: jsonpath "$"`, "", 200, Passed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "GET http://example.test/first\nHTTP 200\n[Asserts]\n" + tc.assertions + "\n[Captures]\n" + tc.capture + "\nGET http://example.test/next\n[Captures]\nfuture: jsonpath \"$\""
			s := dynamicScenario(t, source)
			before := dynamicScenario(t, source)
			c, _ := template.New([]string{"v=" + tc.value}, nil)
			session := c.NewSession()
			calls := 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: tc.status, Body: []byte(tc.body)}, nil
			}), observedHeaderSession{session}, nil).Run(context.Background(), s)
			wantCalls := 1
			if tc.outcome == Passed {
				wantCalls = 2
			}
			if calls != wantCalls || got.Entries[0].Outcome != tc.outcome || !reflect.DeepEqual(s, before) {
				t.Fatalf("calls=%d result=%+v mutated=%v", calls, got, !reflect.DeepEqual(s, before))
			}
			_, err := session.Render("{{checked}}")
			if (err == nil) != (tc.outcome == Passed) {
				t.Fatalf("publication err=%v", err)
			}
			if tc.detail != "" && (got.Err == nil || !strings.Contains(got.Err.Error(), tc.detail)) {
				t.Fatalf("error=%v", got.Err)
			}
			for _, a := range got.Entries[0].Entry.Assertions {
				if a.JSON.Expected != nil || a.JSON.ExpectedTemplate != nil {
					t.Fatalf("retained plan: %+v", a.JSON)
				}
			}
		})
	}
}

// GO-421: absent/defined variables have no effect in the default literal profile.
func TestJSONInequalityLiteralProfile(t *testing.T) {
	for _, vars := range [][]string{nil, {"v=resolved"}} {
		s, err := parser.Parse("literal.hurl", strings.NewReader("GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \"$\" != \"{{v}}\""))
		if err != nil {
			t.Fatal(err)
		}
		c, _ := template.New(vars, nil)
		got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			return Response{Status: 200, Body: []byte(`"{{v}}"`)}, nil
		}), c, nil).Run(context.Background(), s)
		if !got.Diverged || got.Err != nil {
			t.Fatalf("result=%+v", got)
		}
	}
}

// GO-425: structures and secret derivatives never enter callback/result operands.
func TestJSONInequalitySafeOperands(t *testing.T) {
	for _, mode := range []string{"collection", "marked", "derived", "pending"} {
		t.Run(mode, func(t *testing.T) {
			expected, body := `{"private-key":["private-value"]}`, `{"private-key":["private-value"]}`
			vars, secrets := []string(nil), []string(nil)
			capture := ""
			if mode == "marked" || mode == "derived" {
				expected = `"prefix-{{v}}-suffix"`
				body = `"prefix-fake-private-suffix"`
				vars = []string{"v=fake-private"}
				secrets = []string{"v"}
			}
			if mode == "pending" {
				secrets = []string{"pending"}
				capture = "\n[Captures]\npending: jsonpath \"$.missing.bad\""
			}
			source := "GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \"$\" != " + expected + capture + "\nGET http://example.test/never"
			s := dynamicScenario(t, source)
			c, _ := template.New(vars, secrets)
			calls, reports := 0, 0
			check := func(e EntryResult) {
				t.Helper()
				f := e.JSONFailure
				if f == nil || f.Operator != "!=" || f.Position.Line != 4 {
					t.Fatalf("failure=%+v", f)
				}
				if mode != "collection" && (f.Expected != "[oculto]" || f.Actual != "[oculto]") {
					t.Fatalf("not concealed: %+v", f)
				}
			}
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(body)}, nil
			}), c, func(e EntryResult) error { reports++; check(e); return nil }).Run(context.Background(), s)
			if calls != 1 || reports != 1 || !got.Diverged || got.Err != nil {
				t.Fatalf("result=%+v", got)
			}
			check(got.Entries[0])
			data, _ := json.Marshal(got)
			for _, bad := range []string{"private-key", "private-value", "fake-private", "{{v}}"} {
				if strings.Contains(string(data), bad) {
					t.Fatalf("unsafe result: %s", data)
				}
			}
		})
	}
}
