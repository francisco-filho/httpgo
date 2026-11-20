package e2e

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigProcessOutputFailure(t *testing.T) {
	testConfiguredProcessOutput(t, "base")
}

func TestEnvironmentProcessOutputFailure(t *testing.T) {
	testConfiguredProcessOutput(t, "environment")
}

func TestProcessEnvironmentRealOutputFailure(t *testing.T) {
	testConfiguredProcessOutput(t, "process")
}

func TestEnvFileRealOutputFailure(t *testing.T) {
	testConfiguredProcessOutput(t, "env-file")
}

func testConfiguredProcessOutput(t *testing.T, source string) {
	t.Helper()
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				if stream == "stderr" {
					w.WriteHeader(503)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			dir := t.TempDir()
			configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables: {token: fake-output-token}\nsecrets: [token]")
			file := configFile(t, dir, "output.hurl", "GET "+server.URL+"/{{token}}\nHTTP 200\nGET "+server.URL+"/never\nHTTP 200")
			full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = full.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			args := []string{"test", file}
			if source == "process" || source == "env-file" {
				configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables: {token: '${OWNER_TOKEN}'}")
			}
			if source == "env-file" {
				configFile(t, dir, "output.env", "OWNER_TOKEN=fake-output-token")
				args = append(args, "--env-file", "output.env")
			}
			if source == "environment" {
				environmentFile(t, dir, "local", "variables: {token: fake-output-token}\nsecrets: []")
				args = append(args, "--env", "local")
			}
			command := exec.CommandContext(ctx, binary(t), args...)
			command.Dir = dir
			switch source {
			case "process":
				token := "fake-output-token"
				command.Env = childEnvironment(map[string]*string{"OWNER_TOKEN": &token})
			case "env-file":
				command.Env = childEnvironment(map[string]*string{"OWNER_TOKEN": nil})
			}
			var out, stderr bytes.Buffer
			command.Stdout = &out
			command.Stderr = &stderr
			if stream == "stdout" {
				command.Stdout = full
			} else {
				command.Stderr = full
			}
			err = command.Run()
			if ctx.Err() != nil || err == nil || command.ProcessState == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 {
				t.Fatalf("stream=%s state=%v err=%v hits=%d", stream, command.ProcessState, err, hits.Load())
			}
			if strings.Contains(out.String()+stderr.String(), "fake-output-token") {
				t.Fatalf("leak: out=%q err=%q", out.String(), stderr.String())
			}
			if stream == "stdout" && (!strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(stderr.String(), "resumo")) {
				t.Fatalf("diagnostic=%q", stderr.String())
			}
		})
	}
}
