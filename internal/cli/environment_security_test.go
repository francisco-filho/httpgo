package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestEnvironmentProtectionBeforeSession(t *testing.T) {
	for _, mode := range []string{"merge conflict", "CLI duplicate", "scenario syntax", "missing scenario", "invalid flag"} {
		t.Run(mode, func(t *testing.T) {
			base := writeConfig(t, "version: 1\nvariables: {token: base-fake-token, structure: 1}")
			env := "variables: {token: env-fake-token}\nsecrets: [token]"
			if mode == "merge conflict" {
				env = "variables: {token: env-fake-token, structure: {child: 1}}\nsecrets: [token]"
			}
			writeEnvironment(t, base, "local", env)
			file := filepath.Join(t.TempDir(), "base-fake-token-env-fake-token-cli-fake-token.hurl")
			text := "GET http://example.test/{{token}}\nHTTP 200"
			if mode == "scenario syntax" {
				text += "\nHEAD http://example.test/"
			}
			if mode != "missing scenario" {
				if err := os.WriteFile(file, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"test", file, "--config", base, "--env", "local", "--var", "token=cli-fake-token"}
			if mode == "CLI duplicate" {
				args = append(args, "--var", "token=duplicate-fake-token")
			}
			if mode == "invalid flag" {
				args = append(args, "--unsupported")
			}
			fake := &fakeExecutor{}
			code, out, stderr := execute(t, context.Background(), fake, args...)
			if code != 2 || len(fake.calls) != 0 || stderr == "" {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
			for _, secret := range []string{"base-fake-token", "env-fake-token", "cli-fake-token", "duplicate-fake-token"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("leaked %s: %q %q", secret, out, stderr)
				}
			}
		})
	}
}

func TestEnvironmentReuseAndCaptureIsolation(t *testing.T) {
	base := writeConfig(t, "version: 1\nvariables: {token: base-fake-token}\ndefaults: {timeout: 3}")
	writeEnvironment(t, base, "local", "variables: {token: local-fake-token}\nsecrets: [token, future]\ndefaults: {timeout: 0.001}")
	writeEnvironment(t, base, "staging", "variables: {token: staging-fake-token}\nsecrets: []\ndefaults: {timeout: 86400}")
	file := writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\n[Captures]\nfuture: jsonpath \"$.value\"\nGET http://example.test/{{future}}\nHTTP 200")
	wantTimeout := time.Duration(0)
	calls := 0
	command := NewRootCommand(executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
		calls++
		if r.Timeout != wantTimeout {
			t.Fatalf("timeout=%s want=%s", r.Timeout, wantTimeout)
		}
		return runner.Response{Status: 200, Body: []byte(`{"value":"captured-fake-token"}`)}, nil
	}), BuildInfo{})
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	for _, tc := range []struct {
		env       string
		timeout   time.Duration
		protected bool
	}{{"local", time.Millisecond, true}, {"staging", 24 * time.Hour, false}, {"local", time.Millisecond, true}, {"", 3 * time.Second, false}} {
		out.Reset()
		stderr.Reset()
		wantTimeout = tc.timeout
		args := []string{"test", file, "--config", base}
		if tc.env != "" {
			args = append(args, "--env", tc.env)
		}
		command.SetArgs(args)
		if code := command.ExecuteContext(context.Background()); code != 0 || stderr.Len() != 0 {
			t.Fatalf("env=%s code=%d out=%q err=%q", tc.env, code, out.String(), stderr.String())
		}
		if strings.Contains(out.String(), "captured-fake-token") == tc.protected {
			t.Fatalf("protection leaked between executions: %s", out.String())
		}
	}
	if calls != 8 {
		t.Fatal(calls)
	}
	out.Reset()
	stderr.Reset()
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test/{{future}}"), "--config", base, "--env", "local"})
	if code := command.ExecuteContext(context.Background()); code != 2 || calls != 8 || !strings.Contains(stderr.String(), "variável ausente") {
		t.Fatalf("code=%d calls=%d err=%q", code, calls, stderr.String())
	}
}

func TestEnvironmentTimeoutMetadata(t *testing.T) {
	for _, tc := range []struct {
		env  string
		want time.Duration
	}{{"", 2500 * time.Millisecond}, {"defaults: {timeout: 0.001}", time.Millisecond}, {"defaults: {timeout: 86400}", 24 * time.Hour}} {
		base := writeConfig(t, "version: 1\ndefaults: {timeout: 2.5}")
		writeEnvironment(t, base, "local", tc.env)
		fake := executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
			if r.Timeout != tc.want {
				t.Fatalf("timeout=%s want=%s", r.Timeout, tc.want)
			}
			return runner.Response{}, context.DeadlineExceeded
		})
		code, _, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/\nHTTP 200"), "--config", base, "--env", "local")
		if code != 2 || !strings.Contains(stderr, "limite total de") {
			t.Fatalf("code=%d err=%q", code, stderr)
		}
	}
}
