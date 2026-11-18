package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestRegexAssertionCommandReuse(t *testing.T) {
	calls := 0
	command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		calls++
		return runner.Response{Status: 200, Body: []byte("id=fresh"), Headers: []query.HeaderValue{{Name: "X-ID", Value: "fresh"}}}, nil
	}), BuildInfo{})
	capture := writeScenario(t, "GET http://example.test\nHTTP 200\n[Captures]\nid: header \"X-ID\"\n[Asserts]\nregex \"id=(.*)\" == \"{{id}}\"")
	absent := writeScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\nregex \"id=(.*)\" not contains \"{{id}}\"")
	for _, tc := range []struct {
		file string
		code int
	}{{capture, 0}, {absent, 2}, {capture, 0}, {absent, 2}} {
		var out, stderr bytes.Buffer
		command.SetOut(&out)
		command.SetErr(&stderr)
		command.SetArgs([]string{"test", tc.file, "--render-asserts"})
		if code := command.ExecuteContext(context.Background()); code != tc.code || tc.code == 2 && !strings.Contains(stderr.String(), "variável ausente") {
			t.Fatalf("code=%d out=%s err=%s", code, out.String(), stderr.String())
		}
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}
