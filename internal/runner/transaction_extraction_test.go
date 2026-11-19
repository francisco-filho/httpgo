package runner

import (
	"context"
	"errors"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestCaptureExtractionErrorsPrecedeAssertionsAndNeverRetry(t *testing.T) {
	for _, tc := range []struct{ name, declaration, body, contentType string }{
		{"missing", `broken: jsonpath "$.missing"`, `{}`, ""},
		{"invalid JSON", `broken: jsonpath "$"`, `invalid`, ""},
		{"invalid charset", `broken: regex "(.*)"`, "\xff", "text/plain; charset=utf-8"},
		{"unsupported charset", `broken: regex "(.*)"`, `body`, "text/plain; charset=unknown"},
		{"missing regex group", `broken: regex "(a)?b"`, `b`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := template.New([]string{"id=old"}, []string{"id"})
			session := c.NewSession()
			s := dynamicScenario(t, "GET http://example.test\n[Options]\nretry: 2\nretry-interval: 0\nHTTP *\n[Captures]\nid: header \"X\"\n"+tc.declaration+"\n[Asserts]\njsonpath \"$\" == {{missing}}\nGET http://example.test/never")
			calls := 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(tc.body), Headers: []query.HeaderValue{{Name: "X", Value: "discarded-private"}, {Name: "Content-Type", Value: tc.contentType}}}, nil
			}), observedHeaderSession{session}, nil).Run(context.Background(), s)
			var failure *CaptureError
			if calls != 1 || !errors.As(got.Err, &failure) || got.Diverged || got.ErrorPosition.Line != 8 || got.Entries[0].Outcome != CaptureFailed {
				t.Fatalf("result=%+v calls=%d", got, calls)
			}
			old, _ := session.Render("{{id}}")
			if old.Value != "old" || session.Sanitize("discarded-private") != "[oculto]" {
				t.Fatalf("old=%+v protection lost", old)
			}
		})
	}
}
