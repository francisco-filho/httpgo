package runner

import (
	"context"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

// GO-338: synthesized metadata is protected too, even when every selected leaf
// is public. In particular the length must not escape through EntryResult.
func TestProjectionSynthesizedSummaryProtection(t *testing.T) {
	for _, tc := range []struct{ secret, line string }{
		{"2", `jsonpath "$[*]" count == 0`},
		{"array", `jsonpath "$[*]" count == 0`},
		{"objeto", `jsonpath "$[*]" == {}`},
	} {
		t.Run(tc.secret, func(t *testing.T) {
			renderer, err := template.New([]string{"token=" + tc.secret}, []string{"token"})
			if err != nil {
				t.Fatal(err)
			}
			check := func(entry EntryResult) error {
				t.Helper()
				failure := entry.JSONFailure
				if failure == nil || failure.Actual != "[oculto]" || failure.Expected != "[oculto]" || failure.Detail != "" {
					t.Fatalf("protected summary reached presentation: %+v", failure)
				}
				return nil
			}
			r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				return Response{Status: 200, Body: []byte(`["public-one","public-two"]`)}, nil
			}), renderer, check)
			got := r.Run(context.Background(), parseCaptureScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\n"+tc.line))
			if got.Err != nil || !got.Diverged || len(got.Entries) != 1 {
				t.Fatalf("result=%+v", got)
			}
			_ = check(got.Entries[0])
		})
	}
}
