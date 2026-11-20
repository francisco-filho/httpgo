package cli

import (
	"context"
	"strings"
	"testing"
)

func TestEnvFileHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%q", code, stderr)
		}
		for _, text := range []string{"--env-file arquivo", "${NOME}", "uma ocorrência", "cwd", "Extensão HTTPGo", "literais", "sem escapes", "carga automática", "inclusive vazio", "base < ambiente < CLI", "secretos após override", "--env seleciona somente YAML", "--env-file .env.local"} {
			if !strings.Contains(out, text) {
				t.Errorf("help missing %q", text)
			}
		}
	}
}
