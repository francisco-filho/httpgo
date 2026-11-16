package suite

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestPollingSuiteMetricsAndFailFast(t *testing.T) {
	for _, ready := range []bool{true, false} {
		for _, fast := range []bool{true, false} {
			t.Run(fmt.Sprintf("ready=%v/fast=%v", ready, fast), func(t *testing.T) {
				dir := t.TempDir()
				fixture(t, dir, "01.hurl", "GET http://example.test/job\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n")
				fixture(t, dir, "02.hurl", "GET http://example.test/last\nHTTP 200\n")
				jobs, last := 0, 0
				s := New(runner.New(executeFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					if strings.HasSuffix(req.URL, "/job") {
						jobs++
						if !ready || jobs < 3 {
							return runner.Response{Status: 503}, nil
						}
					} else {
						last++
					}
					return runner.Response{Status: 200}, nil
				}), nil))
				files, err := s.Discover(context.Background(), []string{dir})
				if err != nil {
					t.Fatal(err)
				}
				got := s.Run(context.Background(), files, Options{FailFast: fast}, nil)
				wantLast, wantCode := 1, 0
				if !ready {
					wantCode = 1
					if fast {
						wantLast = 0
					}
				}
				if jobs != 3 || last != wantLast || got.Code != wantCode || got.Summary.Attempts != (runner.AttemptSummary{Started: 3 + wantLast, Completed: 3 + wantLast, StatusChecks: 3 + wantLast}) || got.Summary.Requests.Started != 1+wantLast || got.Files[0].Attempts.Started != 3 {
					t.Fatalf("jobs=%d last=%d result=%+v", jobs, last, got)
				}
				if ready && (got.Summary.Failed != 0 || got.Summary.AssertionFailures != 0) {
					t.Fatalf("transient failure persisted: %+v", got)
				}
			})
		}
	}
}
