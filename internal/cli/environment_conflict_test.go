package cli

import (
	"context"
	"testing"
)

func TestEnvironmentConflictsCannotBeHiddenByCLI(t *testing.T) {
	for _, tc := range []struct {
		base, env string
		vars      []string
	}{
		{"a: 1", "a: {b: 2}", []string{"a=3"}},
		{"a: {b: 2}", "a: null", []string{"a.b=3"}},
		{"a: {}", "", []string{"a=3"}},
		{"", "a: {}, a.b: 1, a.b: 2", []string{"a.b=3"}},
		{"", "a: 1", []string{"a=2", "a=3"}},
		{"", "a: 1", []string{"a.b=3"}},
	} {
		base := writeConfig(t, "version: 1\nvariables: {"+tc.base+"}")
		writeEnvironment(t, base, "local", "variables: {"+tc.env+"}")
		fake := &fakeExecutor{}
		args := []string{"test", writeScenario(t, "GET http://example.test/\nHTTP 200"), "--config", base, "--env", "local"}
		for _, assignment := range tc.vars {
			args = append(args, "--var", assignment)
		}
		code, out, stderr := execute(t, context.Background(), fake, args...)
		if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 {
			t.Fatalf("case=%+v code=%d out=%q err=%q calls=%v", tc, code, out, stderr, fake.calls)
		}
	}
}
