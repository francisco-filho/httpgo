package e2e

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Each child receives an exact override/absence, never duplicate environment keys.
func runProcessEnvironment(t *testing.T, dir string, environment map[string]*string, args ...string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), args...)
	command.Dir = dir
	command.Env = childEnvironment(environment)
	var out, stderr bytes.Buffer
	command.Stdout, command.Stderr = &out, &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("harness deadline: %v", ctx.Err())
	}
	code := 0
	if err != nil {
		if command.ProcessState == nil {
			t.Fatal(err)
		}
		code = command.ProcessState.ExitCode()
	}
	return code, out.String(), stderr.String()
}

func childEnvironment(overrides map[string]*string) []string {
	var environment []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, overridden := overrides[name]; !overridden {
			environment = append(environment, entry)
		}
	}
	for name, value := range overrides {
		if value != nil {
			environment = append(environment, name+"="+*value)
		}
	}
	return environment
}

func TestProcessEnvironmentDemonstration(t *testing.T) {
	const token = "fiction-from-child-environment"
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/profile" || r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("incorrect path or credential")
		}
		w.Header().Set("X-Echo", token)
		_, _ = w.Write([]byte(`{"token":"` + token + `"}`))
	}))
	defer server.Close()
	for _, selected := range []bool{false, true} {
		dir := t.TempDir()
		file := configFile(t, dir, "profile.hurl", "GET "+server.URL+"/profile\nAuthorization: Bearer {{auth.token}}\nHTTP 200")
		source := "variables: {auth: {token: '${OWNER_TOKEN}'}}"
		args := []string{"test", file}
		if selected {
			configFile(t, dir, ".httpgo.yaml", "version: 1")
			environmentFile(t, dir, "local", source)
			args = append(args, "--env", "local")
		} else {
			configFile(t, dir, ".httpgo.yaml", "version: 1\n"+source)
		}
		value := token
		code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"OWNER_TOKEN": &value}, args...)
		if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") || strings.Contains(out+stderr, token) {
			t.Fatalf("code=%d safe=%v", code, !strings.Contains(out+stderr, token))
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "testdata", "scenarios", "scenario_process_profile.hurl")
	value := token
	code, out, stderr := runProcessEnvironment(t, t.TempDir(), map[string]*string{"OWNER_TOKEN": &value}, "test", file, "--config", filepath.Join(root, "testdata", "config", "process_base.yaml"), "--env", "process", "--var", "api.baseUrl="+server.URL)
	if code != 0 || stderr != "" || strings.Contains(out+stderr, token) || hits.Load() != 3 {
		t.Fatalf("documented example code=%d hits=%d", code, hits.Load())
	}
	for _, name := range []string{"invalid_process_reference.yaml", "unsupported_process_list.yaml", "unsupported_environment.yaml"} {
		code, out, stderr := runProcessEnvironment(t, t.TempDir(), map[string]*string{"OWNER_TOKEN": &value}, "test", file, "--config", filepath.Join(root, "testdata", "config", name), "--var", "auth.token=fiction", "--var", "tokens=fiction")
		if code != 2 || out != "" || stderr == "" || hits.Load() != 3 || strings.Contains(stderr, token) {
			t.Fatalf("negative fixture %s code=%d hits=%d", name, code, hits.Load())
		}
	}
}
