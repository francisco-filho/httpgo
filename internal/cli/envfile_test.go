package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/runner"
)

func writeEnvFile(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "local.env")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvFileRequestAndSuite(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(map[bool]string{false: "base", true: "environment"}[selected], func(t *testing.T) {
			base := writeConfig(t, "version: 1\nvariables: {url: '${HTTPGO_43_URL}', token: '${HTTPGO_43_TOKEN}', number: '${HTTPGO_43_NUMBER}', boolean: '${HTTPGO_43_BOOL}', nothing: '${HTTPGO_43_NULL}'}")
			args := []string{"--config", base, "--env-file", writeEnvFile(t, "HTTPGO_43_URL=http://example.test\nHTTPGO_43_TOKEN=fiction-file-token\nHTTPGO_43_NUMBER=123\nHTTPGO_43_BOOL=true\nHTTPGO_43_NULL=null\nHTTPGO_TIMEOUT=0.001\nNO_COLOR=1")}
			if selected {
				writeEnvironment(t, base, "local", "variables: {url: '${HTTPGO_43_URL}', token: '${HTTPGO_43_TOKEN}'}")
				args = append(args, "--env", "local")
			}
			file := writeScenario(t, "POST {{url}}/profile\nAuthorization: Bearer {{token}}\n{\"number\":{{number}},\"boolean\":{{boolean}},\"nothing\":{{nothing}}}\nHTTP 200")
			second := writeScenario(t, "GET {{url}}/second\nHTTP 200")
			calls := 0
			fake := executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
				calls++
				if r.Timeout != 10*time.Second || !strings.HasPrefix(r.URL, "http://example.test/") {
					t.Error("URL or timeout changed")
				}
				if calls == 1 {
					var body map[string]string
					if json.Unmarshal(r.Body, &body) != nil || body["number"] != "123" || body["boolean"] != "true" || body["nothing"] != "null" || r.Headers[0].Value != "Bearer fiction-file-token" {
						t.Error("content or string types changed")
					}
				}
				return runner.Response{Status: 200}, nil
			})
			code, out, stderr := execute(t, context.Background(), fake, append([]string{"test", file, second}, args...)...)
			if code != 0 || calls != 2 || stderr != "" || strings.Contains(out, "fiction-file-token") {
				t.Fatalf("code=%d calls=%d stderr=%q", code, calls, stderr)
			}
			if _, present := os.LookupEnv("HTTPGO_43_TOKEN"); present {
				t.Fatal("file changed process environment")
			}
		})
	}
}

func TestEnvFileInvalidBeforeHTTP(t *testing.T) {
	t.Setenv("HTTPGO_43_OWNER", "process-wins")
	for _, source := range []string{"HTTPGO_43_OWNER=fiction\nHTTPGO_43_OWNER=fiction", "UNUSED=fiction\ninvalid", "export UNUSED=fiction", "UNUSED='fiction", "UNUSED=\"fiction\" tail"} {
		fake := &fakeExecutor{}
		base := writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_43_OWNER}'}")
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/"), "--config", base, "--env-file", writeEnvFile(t, source), "--var", "token=substitute")
		if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 || strings.Contains(stderr, "fiction") {
			t.Fatalf("code=%d calls=%d err=%q", code, len(fake.calls), stderr)
		}
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "missing.env"), t.TempDir()} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/"), "--env-file", path)
		if code != 2 || out != "" || len(fake.calls) != 0 || !strings.Contains(stderr, path) {
			t.Fatalf("I/O failure code=%d calls=%d", code, len(fake.calls))
		}
	}
}

func TestEnvFileFlagValidation(t *testing.T) {
	path := writeEnvFile(t, "A=fiction")
	for _, flags := range [][]string{
		{"--env-file"}, {"--env-file="}, {"--env-file", ""}, {"--env-file", "--fail-fast"},
		{"--env-file", path, "--env-file", path}, {"--env-file=" + path, "--env-file", path}, {"--env-file=" + path, "--env-file=" + path},
	} {
		fake := &fakeExecutor{}
		destination := filepath.Join(t.TempDir(), "report.xml")
		args := []string{"test", writeScenario(t, "GET http://example.test/"), "--output", "junit", "--output-file", destination}
		code, out, stderr := execute(t, context.Background(), fake, append(args, flags...)...)
		if code != 2 || out != "" || len(fake.calls) != 0 || !strings.Contains(stderr, "--env-file") {
			t.Fatalf("flags=%v code=%d calls=%d err=%q", flags, code, len(fake.calls), stderr)
		}
		if _, err := os.Stat(destination); !os.IsNotExist(err) {
			t.Fatal("usage error created report")
		}
	}
}

func TestEnvFileExtrasNotVariables(t *testing.T) {
	fake := &fakeExecutor{}
	code, _, _ := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/{{UNREFERENCED}}"), "--env-file="+writeEnvFile(t, "UNREFERENCED=value"))
	if code != 2 || len(fake.calls) != 0 {
		t.Fatal("extra key became a scenario variable")
	}
}
