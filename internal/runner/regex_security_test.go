package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-498/500: protection is monotonic through renewal and discarded overlays;
// neither callbacks nor stored results retain extraction plans or response data.
func TestRegexAssertionSafeResults(t *testing.T) {
	for _, mode := range []string{"success", "mismatch", "absence", "evaluation"} {
		t.Run(mode, func(t *testing.T) {
			c, _ := template.New([]string{"token=old-private"}, []string{"token"})
			session := c.NewSession()
			decl := `regex "id=(.*)" == "prefix-{{token}}-suffix"`
			if mode == "evaluation" {
				decl = `regex "id=(.*)" matches "prefix-{{token}}-suffix["`
			}
			source := "GET http://example.test\nHTTP 200\n[Captures]\ntoken: header \"X-ID\"\nfresh: header \"X-ID\"\n[Asserts]\n" + decl + "\nGET http://example.test/next"
			scenario := dynamicScenario(t, source)
			calls, reports := 0, 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				body := "id=prefix-new-private-suffix"
				if mode == "mismatch" {
					body = "id=unrelated-response-private"
				}
				if mode == "absence" {
					body = "no selected value"
				}
				return Response{Status: 200, Body: []byte(body), Headers: []query.HeaderValue{{Name: "X-ID", Value: "new-private"}}}, nil
			}), observedHeaderSession{session}, func(e EntryResult) error {
				reports++
				derived := "prefix-new-private-suffix"
				if mode == "evaluation" {
					derived += "["
				}
				if session.Sanitize("old-private/new-private/"+derived) != "[oculto]/[oculto]/[oculto]" {
					t.Fatal("callback preceded protection")
				}
				raw, err := json.Marshal(e)
				if err != nil {
					t.Fatal(err)
				}
				for _, bad := range []string{"old-private", "new-private", "unrelated-response-private", "{{token}}", "id=(.*)"} {
					if strings.Contains(string(raw), bad) {
						t.Fatalf("unsafe callback=%s", raw)
					}
				}
				for _, a := range e.Entry.Assertions {
					if a.Regex != nil && (a.Regex.Extraction != nil || a.Regex.Compiled != nil || a.Regex.ExpectedTemplate != nil || a.Regex.Pattern != "" || a.Regex.Expected != "") {
						t.Fatalf("unsafe metadata=%+v", a.Regex)
					}
				}
				return nil
			}).Run(context.Background(), scenario)
			wantCalls, wantID := 1, "old-private"
			if mode == "success" {
				wantCalls, wantID = 2, "new-private"
			}
			value, _ := session.Render("{{token}}")
			if calls != wantCalls || reports != wantCalls || value.Value != wantID {
				t.Fatalf("calls=%d reports=%d value=%s result=%+v", calls, reports, value.Value, got)
			}
			if mode != "success" {
				if _, err := session.Render("{{fresh}}"); err == nil {
					t.Fatal("discarded capture published")
				}
			}
			raw, _ := json.Marshal(got)
			if strings.Contains(string(raw), "private") {
				t.Fatalf("unsafe result=%s", raw)
			}
		})
	}
}
