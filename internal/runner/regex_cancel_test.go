package runner

import (
	"context"
	"errors"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestRegexCancelBeforeConfirmation(t *testing.T) {
	c, _ := template.New([]string{"id=old"}, nil)
	session := c.NewSession()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := "GET http://example.test\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n[Captures]\nid: header \"X-ID\"\n[Asserts]\nregex \"(.*)\" == \"{{id}}\"\nGET http://example.test/never"
	calls := 0
	got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		return Response{Status: 200, Body: []byte("new"), Headers: []query.HeaderValue{{Name: "X-ID", Value: "new"}}}, nil
	}), observedHeaderSession{cancelAfterStage{session, cancel}}, nil).Run(ctx, dynamicScenario(t, source))
	value, _ := session.Render("{{id}}")
	if !errors.Is(got.Err, context.Canceled) || calls != 1 || got.Diverged || value.Value != "old" {
		t.Fatalf("value=%s calls=%d result=%+v", value.Value, calls, got)
	}
}
