package runner

import (
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-462–465: extraction is a barrier; assertions get only a complete overlay.
func TestTransactionalCaptureAttempt(t *testing.T) {
	for _, tc := range []struct {
		name, captures, assertions, body string
		status                           int
		outcome                          Outcome
		commit                           bool
	}{
		{"new expected", `id: jsonpath "$.id"`, `jsonpath "$.id" == {{id}}`, `{"id":"new"}`, 200, Passed, true},
		{"all before assertion", "id: jsonpath \"$.id\"\nfresh: jsonpath \"$.missing\"", `header "Missing" exists`, `{"id":"new"}`, 200, CaptureFailed, false},
		{"status barrier", `id: jsonpath "$.missing"`, `jsonpath "$" == {{missing}}`, `invalid`, 503, Mismatch, false},
		{"mismatch discards", "id: jsonpath \"$.id\"\nfresh: jsonpath \"$.id\"", "jsonpath \"$.id\" == \"other\"\njsonpath \"$\" == {{missing}}", `{"id":"new"}`, 200, JSONMismatch, false},
		{"evaluation discards", "id: jsonpath \"$.id\"\nfresh: jsonpath \"$.id\"", `jsonpath "$" == {{missing}}`, `{"id":"new"}`, 200, JSONFailed, false},
		{"duplicate discards", "id: jsonpath \"$.id\"\nfresh: jsonpath \"$.id\"\nid: jsonpath \"$.id\"", `jsonpath "$.id" == {{id}}`, `{"id":"new"}`, 200, CaptureFailed, false},
		{"capture queries use old", "id: jsonpath \"$.id\"\nfresh: jsonpath \"$.items[?(@.id=='{{id}}')].value\"", "jsonpath \"$.id\" == {{id}}\njsonpath \"$.items[0].value\" == {{fresh}}", `{"id":"new","items":[{"id":"old","value":42},{"id":"new","value":99}]}`, 200, Passed, true},
		{"batch cannot chain", "id: jsonpath \"$.id\"\nfresh: jsonpath \"$.items[?(@.id=={{not_yet}})].value\"\nnot_yet: jsonpath \"$.id\"", `header "Missing" exists`, `{"id":"new","items":[]}`, 200, CaptureFailed, false},
		{"header and body expected", `id: jsonpath "$.id"`, "header \"X\" == \"{{id}}\"\nbody contains \"{{id}}\"\njsonpath \"$.id\" matches \"^{{id}}$\"", `{"id":"new"}`, 200, Passed, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := template.New([]string{"id=old"}, nil)
			session := c.NewSession()
			s := dynamicScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\n"+tc.assertions+"\n[Captures]\n"+tc.captures+"\nGET http://example.test/next")
			calls := 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: tc.status, Body: []byte(tc.body), Headers: []query.HeaderValue{{Name: "X", Value: "new"}}}, nil
			}), observedHeaderSession{session}, nil).Run(context.Background(), s)
			wantCalls := 1
			wantID := "old"
			if tc.commit {
				wantCalls = 2
				wantID = "new"
			}
			if calls != wantCalls || got.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v", calls, got)
			}
			value, err := session.Render("{{id}}")
			if err != nil || value.Value != wantID {
				t.Fatalf("confirmed=%+v err=%v want=%s", value, err, wantID)
			}
			if !tc.commit {
				if _, err := session.Render("{{fresh}}"); err == nil {
					t.Fatal("partial confirmation")
				}
			}
		})
	}
}

func TestOwnCaptureLiteralExpected(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		source := "GET http://example.test\nHTTP *\n[Captures]\nid: jsonpath \"$.id\"\n[Asserts]\njsonpath \"$.id\" == \"{{id}}\""
		s := parseCaptureScenario(t, source)
		if enabled {
			s = dynamicScenario(t, source)
		}
		got := New(executorFunc(func(context.Context, Request) (Response, error) {
			return Response{Status: 200, Body: []byte(`{"id":"new"}`)}, nil
		}), nil).Run(context.Background(), s)
		if got.Err != nil || got.Diverged == enabled {
			t.Fatalf("enabled=%v result=%+v", enabled, got)
		}
		if !enabled && !strings.Contains(got.Entries[0].JSONFailure.Expected, "{{id}}") {
			t.Fatal("literal expected changed")
		}
	}
}
