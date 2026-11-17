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
	"syscall"
	"testing"
	"time"
)

func TestHTMLRealPermissionFailure(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	dir, err := os.MkdirTemp("", "httpgo-html-permissions-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := configFile(t, dir, "input.hurl", "GET "+server.URL+"\nHTTP 200")
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	denied := filepath.Join(dir, "denied")
	if err := os.Mkdir(denied, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0700) })
	executable := binary(t)
	if os.Geteuid() == 0 {
		data, err := os.ReadFile(executable)
		if err != nil {
			t.Fatal(err)
		}
		executable = filepath.Join(dir, "httpgo")
		if err := os.WriteFile(executable, data, 0755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "test", file, "--output=html", "--output-file", filepath.Join(denied, "report.html"))
	command.Dir = dir
	if os.Geteuid() == 0 {
		command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	}
	var out, stderr bytes.Buffer
	command.Stdout, command.Stderr = &out, &stderr
	err = command.Run()
	if err == nil || command.ProcessState == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "não foi possível gravar relatório HTML") || strings.Contains(stderr.String(), "gerado em") {
		t.Fatalf("err=%v state=%v hits=%d out=%q stderr=%q", err, command.ProcessState, hits.Load(), out.String(), stderr.String())
	}
}
