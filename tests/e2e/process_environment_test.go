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

// Each child receives an exact override/absence, never duplicate environment keys.
func runProcessEnvironment(t *testing.T, dir string, environment map[string]*string, args ...string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), args...)
	command.Dir = dir
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if _, overridden := environment[name]; !overridden {
			command.Env = append(command.Env, entry)
		}
	}
	for name, value := range environment {
		if value != nil {
			command.Env = append(command.Env, name+"="+*value)
		}
	}
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
	if hits.Load() != 2 {
		t.Fatalf("hits=%d", hits.Load())
	}
}
