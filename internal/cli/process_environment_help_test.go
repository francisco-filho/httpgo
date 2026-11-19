package cli

import (
	"context"
	"strings"
	"testing"
)

func TestProcessEnvironmentHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%q", code, stderr)
		}
		for _, text := range []string{"--var nome=valor", "não sigilosa", "--secret nome", "sem fornecer valor secreto", "docs/subset.md"} {
			if !strings.Contains(out, text) {
				t.Errorf("help missing %q", text)
			}
		}
	}
}
