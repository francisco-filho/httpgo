package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func writeEnvironment(t *testing.T, base, name, text string) string {
	t.Helper()
	dir := filepath.Join(filepath.Dir(base), "environments")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvironmentInvalidBeforeHTTP(t *testing.T) {
	base := writeConfig(t, "version: 1")
	writeEnvironment(t, base, "local", "variables: {base: http://example.test}")
	file := writeScenario(t, "GET http://example.test/\nHTTP 200")
	for _, flags := range [][]string{{"--env"}, {"--env="}, {"--env", "../local"}, {"--env", "a.b"}, {"--env", "a b"}, {"--env", "a/b"}, {"--env", "local", "--env", "local"}, {"--env=local", "--env=other"}, {"--env", "missing"}} {
		fake := &fakeExecutor{}
		args := append([]string{"test", file, "--config", base}, flags...)
		code, out, stderr := execute(t, context.Background(), fake, args...)
		if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 {
			t.Fatalf("flags=%v code=%d out=%q err=%q calls=%v", flags, code, out, stderr, fake.calls)
		}
	}
	for _, source := range []string{"version: 2", "variables: {a: '${TOKEN}'}", "variables: {a: &a hidden-fixture}", "variables: [", "variables: {}\n---", "unknown: hidden-fixture"} {
		t.Setenv("TOKEN", "must-not-resolve")
		writeEnvironment(t, base, "local", source)
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, "test", file, "--config", base, "--env", "local", "--var", "a=override")
		if code != 2 || out != "" || len(fake.calls) != 0 || !strings.Contains(stderr, "local.yaml") || strings.Contains(stderr, "hidden-fixture") {
			t.Fatalf("source=%q code=%d out=%q err=%q", source, code, out, stderr)
		}
	}
	// An invalid but unselected environment must not affect US-11.
	code, _, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
		return runner.Response{Status: 200}, nil
	}), "test", file, "--config", base)
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d err=%q", code, stderr)
	}
}
