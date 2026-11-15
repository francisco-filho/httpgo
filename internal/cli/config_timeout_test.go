package cli

import (
	"context"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestConfigTimeoutMetadataAndDiagnostic(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   time.Duration
		label  string
	}{{"0.001", time.Millisecond, "0.001"}, {"2.5", 2500 * time.Millisecond, "2.5"}, {"86400", 24 * time.Hour, "86400"}} {
		calls := 0
		fake := executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
			calls++
			if r.Timeout != tc.want {
				t.Fatalf("timeout=%s want=%s", r.Timeout, tc.want)
			}
			return runner.Response{}, context.DeadlineExceeded
		})
		file := writeScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/last")
		code, out, stderr := execute(t, context.Background(), fake, "test", file, "--config", writeConfig(t, "version: 1\ndefaults: {timeout: "+tc.source+"}"))
		if code != 2 || calls != 1 || !strings.Contains(stderr, "limite total de "+tc.label+" segundos") || !strings.Contains(out, "não_iniciadas=1") {
			t.Fatalf("source=%s code=%d out=%q err=%q calls=%d", tc.source, code, out, stderr, calls)
		}
	}
	for _, source := range []string{"0", "1.0001", "'2s'", ".inf"} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/"), "--config", writeConfig(t, "version: 1\ndefaults: {timeout: "+source+"}"))
		if code != 2 || len(fake.calls) != 0 || out != "" || !strings.Contains(stderr, "timeout") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}
