package runner_test

import (
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/config"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

type configExecutor func(context.Context, runner.Request) (runner.Response, error)

func (f configExecutor) Execute(ctx context.Context, r runner.Request) (runner.Response, error) {
	return f(ctx, r)
}

func TestConfigProtectionReachesCallback(t *testing.T) {
	settings := config.Settings{Secrets: []string{"token"}, Variables: nil}
	renderer, err := settings.Compose([]string{"token=fake-config-secret"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	scenario, err := parser.Parse("fake-config-secret.hurl", strings.NewReader("GET http://example.test/{{token}}\nHTTP 200\n[Asserts]\nheader \"X-Probe\" == \"prefix-fake-config-secret\""))
	if err != nil {
		t.Fatal(err)
	}
	callbacks := 0
	r := runner.NewWithRenderer(configExecutor(func(context.Context, runner.Request) (runner.Response, error) {
		return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "X-Probe", Value: "actual-fake-config-secret"}}}, nil
	}), renderer, func(entry runner.EntryResult) error {
		callbacks++
		for _, text := range []string{entry.DisplayURL, entry.Entry.Position.File, entry.HeaderFailure.Expected, entry.HeaderFailure.Actual} {
			if strings.Contains(text, "fake-config-secret") {
				t.Fatalf("callback leaked %q", text)
			}
		}
		return nil
	})
	result := r.Run(context.Background(), scenario)
	if !result.Diverged || result.Err != nil || callbacks != 1 {
		t.Fatalf("result=%+v callbacks=%d", result, callbacks)
	}
}
