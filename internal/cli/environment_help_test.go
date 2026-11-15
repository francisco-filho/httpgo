package cli

import (
	"context"
	"strings"
	"testing"
)

func TestEnvironmentSelectionDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		flags  []string
		detail string
	}{
		{[]string{"--env"}, "exige um nome"},
		{[]string{"--env="}, "nome de ambiente inválido"},
		{[]string{"--env", "private-fixture/traversal"}, "nome de ambiente inválido"},
		{[]string{"--env=local", "--env=local"}, "somente uma vez"},
	} {
		_, err := parseArguments(append([]string{"test", "scenario.hurl"}, tc.flags...))
		if err == nil || !strings.Contains(err.Error(), tc.detail) || strings.Contains(err.Error(), "private-fixture") {
			t.Fatalf("flags=%v err=%v want=%s", tc.flags, err, tc.detail)
		}
	}
}

func TestEnvironmentHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%q", code, stderr)
		}
		for _, text := range []string{"--env local", "--env staging", "seleção única", "mesmo nome", "environments/<nome>.yaml", "inclusive vazia", "defaults internos < YAML base < ambiente selecionado < CLI", "extensão HTTPGo (US-12)", "NÃO é ambiente do processo", "${NOME} continua rejeitado", "null substituem folhas", "lista vazia nunca remove proteção"} {
			if !strings.Contains(out, text) {
				t.Errorf("missing %q", text)
			}
		}
	}
}
