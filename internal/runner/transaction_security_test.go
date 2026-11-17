package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestDiscardedRenewalProtectsOldNewAndDerivedBeforeCallback(t *testing.T) {
	for _, mode := range []string{"capture error", "mismatch", "evaluation", "success"} {
		t.Run(mode, func(t *testing.T) {
			c, _ := template.New([]string{"token=old-private", "key=selector-private"}, []string{"key"})
			session := c.NewSession()
			source := "GET http://example.test\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.items[?(@.key=='{{key}}')].id\"\nfresh: jsonpath \"$.items[0].id\"\n"
			if mode == "capture error" {
				source += "broken: jsonpath \"$.missing\"\n"
			}
			source += "[Asserts]\nheader \"X\" == \"prefix-{{token}}-suffix\"\n"
			if mode == "evaluation" {
				source += "jsonpath \"$\" == {{missing}}\n"
			}
			source += "GET http://example.test/next"
			calls, callbacks := 0, 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				value := "prefix-new-private-suffix"
				if mode == "mismatch" {
					value = "unrelated-private"
				}
				return Response{Status: 200, Body: []byte(`{"items":[{"key":"selector-private","id":"new-private"}]}`), Headers: []query.HeaderValue{{Name: "X", Value: value}}}, nil
			}), observedHeaderSession{session}, func(e EntryResult) error {
				callbacks++
				if session.Sanitize("old-private/new-private/selector-private") != "[oculto]/[oculto]/[oculto]" {
					t.Fatal("callback preceded protection")
				}
				if mode != "capture error" && session.Sanitize("prefix-new-private-suffix") != "[oculto]" {
					t.Fatal("derived value not retained in base protection")
				}
				raw, _ := json.Marshal(e)
				for _, value := range []string{"old-private", "new-private", "selector-private", "unrelated-private", "$.items", "{{token}}"} {
					if strings.Contains(string(raw), value) {
						t.Fatalf("unsafe callback: %s", raw)
					}
				}
				return nil
			}).Run(context.Background(), dynamicScenario(t, source))
			wantCalls := 1
			wantValue := "old-private"
			if mode == "success" {
				wantCalls = 2
				wantValue = "new-private"
			}
			if calls != wantCalls || callbacks != wantCalls {
				t.Fatalf("calls=%d callbacks=%d result=%+v", calls, callbacks, got)
			}
			value, _ := session.Render("{{token}}")
			if value.Value != wantValue || !session.IsSecret("token") {
				t.Fatalf("confirmed=%+v", value)
			}
			if mode != "success" {
				if _, err := session.Render("{{fresh}}"); err == nil {
					t.Fatal("discarded values remain resolvable")
				}
			}
			raw, _ := json.Marshal(got)
			if strings.Contains(string(raw), "new-private") || strings.Contains(string(raw), "old-private") {
				t.Fatalf("unsafe result=%s", raw)
			}
		})
	}
}
