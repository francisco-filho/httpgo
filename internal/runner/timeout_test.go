package runner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestTimeoutOptionIsPerRunMetadata(t *testing.T) {
	scenario, err := parser.Parse("timeout.hurl", strings.NewReader("GET http://example.test/one\nHTTP 200\nGET http://example.test/two\nHTTP 200"))
	if err != nil {
		t.Fatal(err)
	}
	want := time.Duration(0)
	calls := 0
	executor := configExecutor(func(ctx context.Context, request runner.Request) (runner.Response, error) {
		calls++
		if request.Timeout != want {
			t.Fatalf("timeout=%s want=%s", request.Timeout, want)
		}
		if _, ok := ctx.Deadline(); ok {
			t.Fatal("runner took ownership of transport deadline")
		}
		return runner.Response{Status: 200}, nil
	})
	r := runner.New(executor, nil)
	for _, timeout := range []time.Duration{time.Millisecond, 24 * time.Hour, 0} {
		want = timeout
		got := r.RunWithOptions(context.Background(), scenario, runner.Options{Timeout: timeout})
		if got.Err != nil || got.Summary.Completed != 2 {
			t.Fatalf("result=%+v", got)
		}
	}
	if calls != 6 {
		t.Fatal(calls)
	}
}
