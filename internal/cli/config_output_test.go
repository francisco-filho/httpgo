package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestConfigWriterFailureKeepsPriorityAndProtection(t *testing.T) {
	testConfiguredWriterFailure(t, false)
}

func TestEnvironmentWriterFailureKeepsPriorityAndProtection(t *testing.T) {
	testConfiguredWriterFailure(t, true)
}

func testConfiguredWriterFailure(t *testing.T, environment bool) {
	t.Helper()
	for _, mode := range []string{"result error", "result short", "diagnostic error", "diagnostic short"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 503}, nil
			}), BuildInfo{})
			var out, stderr bytes.Buffer
			var output, diagnostic io.Writer = &out, &stderr
			broken := &failAfterWriter{cause: errors.New("fake-output-secret")}
			switch mode {
			case "result error":
				output = broken
			case "result short":
				output = shortWriter{}
			case "diagnostic error":
				diagnostic = broken
			case "diagnostic short":
				diagnostic = shortWriter{}
			}
			command.SetOut(output)
			command.SetErr(diagnostic)
			base := writeConfig(t, "version: 1\nvariables: {token: fake-output-secret}\nsecrets: [token]")
			args := []string{"test", writeScenario(t, "GET http://example.test/{{token}}\nHTTP 200\nGET http://example.test/never\nHTTP 200"), "--config", base}
			if environment {
				writeEnvironment(t, base, "local", "secrets: []")
				args = append(args, "--env", "local")
			}
			command.SetArgs(args)
			code := command.ExecuteContext(context.Background())
			if code != 2 || calls != 1 || strings.Contains(out.String()+stderr.String()+broken.String(), "fake-output-secret") {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out.String(), stderr.String())
			}
			if strings.HasPrefix(mode, "result") && strings.Contains(out.String()+stderr.String(), "resumo") {
				t.Fatal("summary retried after result writer failure")
			}
		})
	}
}

func TestConfigHelpDocumentsOnlyImplementedSources(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%q", code, stderr)
		}
		for _, text := range []string{"--config arquivo", "configuração YAML do projeto", "docs/subset.md"} {
			if !strings.Contains(out, text) {
				t.Errorf("missing %q in help", text)
			}
		}
	}
}
