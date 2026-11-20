package e2e

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEnvFileDemonstration(t *testing.T) {
	const token = "fiction-env-file-token"
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/profile" || r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("incorrect path or credential")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	for _, selected := range []bool{false, true} {
		dir := t.TempDir()
		// Config and scenario are elsewhere; the env-file path is cwd-relative.
		other := t.TempDir()
		base := configFile(t, other, ".httpgo.yaml", "version: 1\nvariables: {base_url: '${API_URL}', auth: {token: '${OWNER_TOKEN}'}}")
		file := configFile(t, other, "profile.httpgo", "GET {{base_url}}/profile\nAuthorization: Bearer {{auth.token}}\nHTTP 200")
		configFile(t, dir, ".env.local", "API_URL="+server.URL+"\nOWNER_TOKEN="+token)
		args := []string{"test", file, "--config", base, "--env-file", ".env.local"}
		if selected {
			environmentFile(t, other, "local", "variables: {auth: {token: '${OWNER_TOKEN}'}}")
			args = append(args, "--env", "local")
		}
		code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"API_URL": nil, "OWNER_TOKEN": nil}, args...)
		if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") || strings.Contains(out+stderr, token) {
			t.Fatalf("code=%d stderr=%q", code, stderr)
		}
		// Process URL wins even when the fallback URL cannot be used.
		configFile(t, dir, ".env.local", "API_URL=http://invalid.test\nOWNER_TOKEN="+token)
		value := server.URL
		code, _, stderr = runProcessEnvironment(t, dir, map[string]*string{"API_URL": &value, "OWNER_TOKEN": nil}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("process precedence code=%d err=%q", code, stderr)
		}
		// No automatic loading, neither cwd nor ancestor.
		configFile(t, dir, ".env", "invalid content")
		code, out, stderr = runProcessEnvironment(t, dir, map[string]*string{"API_URL": nil, "OWNER_TOKEN": nil}, "test", file, "--config", base)
		if code != 2 || out != "" || !strings.Contains(stderr, "ausente") {
			t.Fatalf("unselected source used: code=%d out=%q err=%q", code, out, stderr)
		}
		// Absolute selection is also accepted.
		args[5] = filepath.Join(dir, ".env.local")
		code, _, _ = runProcessEnvironment(t, dir, map[string]*string{"API_URL": &value, "OWNER_TOKEN": nil}, args...)
		if code != 0 {
			t.Fatalf("absolute source code=%d", code)
		}
	}
	if hits.Load() != 6 {
		t.Fatalf("hits=%d want=6", hits.Load())
	}
}
