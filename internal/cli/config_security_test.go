package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestConfigProtectionBeforeSession(t *testing.T) {
	for _, mode := range []string{"scenario syntax", "CLI conflict", "missing scenario", "invalid flag", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			base := writeConfig(t, "version: 1\nvariables: {token: base-fake-token}\nsecrets: [token]")
			dir := t.TempDir()
			file := filepath.Join(dir, "base-fake-token-override-fake-token.hurl")
			text := "GET http://example.test/{{token}}\nHTTP 200"
			if mode == "scenario syntax" {
				text += "\nHEAD http://example.test/"
			}
			if mode != "missing scenario" {
				if err := os.WriteFile(file, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"test", file, "--config", base, "--var", "token=override-fake-token"}
			if mode == "CLI conflict" {
				args = append(args, "--var", "token=duplicate-fake-token")
			}
			if mode == "invalid flag" {
				args = append(args, "--unsupported")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			fake := &fakeExecutor{}
			code, out, stderr := execute(t, ctx, fake, args...)
			if code != 2 || len(fake.calls) != 0 || stderr == "" {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
			for _, secret := range []string{"base-fake-token", "override-fake-token", "duplicate-fake-token"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("leaked %q: %q %q", secret, out, stderr)
				}
			}
		})
	}
}

func TestConfigProtectionCapturesAndIsolation(t *testing.T) {
	base := writeConfig(t, "version: 1\nvariables: {token: base-fake-token}\nsecrets: [token, future]")
	file := writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\n[Captures]\nfuture: jsonpath \"$.value\"\nGET http://example.test/{{future}}\nHTTP 200")
	calls := 0
	command := NewRootCommand(executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
		calls++
		if calls%2 == 1 {
			if request.URL != "http://example.test/override-fake-token" {
				t.Fatal(request.URL)
			}
			return runner.Response{Status: 200, Body: []byte(`{"value":"captured-fake-token"}`)}, nil
		}
		if request.URL != "http://example.test/captured-fake-token" {
			t.Fatal(request.URL)
		}
		return runner.Response{Status: 200}, nil
	}), BuildInfo{})
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	command.SetArgs([]string{"test", file, "--config", base, "--var", "token=override-fake-token"})
	for range 2 {
		out.Reset()
		stderr.Reset()
		if code := command.ExecuteContext(context.Background()); code != 0 {
			t.Fatalf("code=%d err=%q", code, stderr.String())
		}
		for _, secret := range []string{"base-fake-token", "override-fake-token", "captured-fake-token"} {
			if strings.Contains(out.String()+stderr.String(), secret) {
				t.Fatalf("leak: %q %q", out.String(), stderr.String())
			}
		}
	}
	// Same command, now no YAML and no marks. Nothing from the prior invocation survives.
	out.Reset()
	stderr.Reset()
	visible := writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200")
	command.SetArgs([]string{"test", visible, "--var", "token=override-fake-token"})
	if code := command.ExecuteContext(context.Background()); code != 0 || !strings.Contains(out.String(), "override-fake-token") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), stderr.String())
	}
}

func TestConfigMissingAndInitialCaptureConflict(t *testing.T) {
	for _, text := range []string{"GET http://example.test/{{future}}", "GET http://example.test/\nHTTP 200\n[Captures]\ntoken.child: jsonpath \"$.value\"\nGET http://example.test/last"} {
		fake := executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			return runner.Response{Status: 200, Body: []byte(`{"value":"captured-fake"}`)}, nil
		})
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, text), "--config", writeConfig(t, "version: 1\nvariables: {token: initial-fake}\nsecrets: [future, token]"))
		if code != 2 || strings.Contains(out+stderr, "captured-fake") || strings.Contains(out+stderr, "initial-fake") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
}
