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

func TestSuiteDeniedAccessUnprivilegedProcess(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	dir, err := os.MkdirTemp("", "httpgo-suite-access-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	good := configFile(t, dir, "good.hurl", "GET "+server.URL)
	if err := os.Chmod(good, 0644); err != nil {
		t.Fatal(err)
	}
	denied := configFile(t, dir, "denied.hurl", "GET "+server.URL)
	if err := os.Chmod(denied, 0000); err != nil {
		t.Fatal(err)
	}
	// Copy the built binary to an accessible directory when running as root;
	// the child is always genuinely unprivileged, never a chmod-only root test.
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
	blockedDir := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blockedDir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blockedDir, 0700) })
	fifo := filepath.Join(dir, "pipe.hurl")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{denied, blockedDir, dir, fifo} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		command := exec.CommandContext(ctx, executable, "test", good, path)
		command.Dir = dir
		if os.Geteuid() == 0 {
			command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		}
		var out, stderr bytes.Buffer
		command.Stdout = &out
		command.Stderr = &stderr
		err := command.Run()
		cancel()
		if err == nil || command.ProcessState == nil || command.ProcessState.ExitCode() != 2 || out.Len() != 0 || (path != fifo && !strings.Contains(stderr.String(), "permissão") || path == fifo && !strings.Contains(stderr.String(), "regular")) || hits.Load() != 0 {
			t.Fatalf("state=%v err=%v out=%q stderr=%q hits=%d", command.ProcessState, err, out.String(), stderr.String(), hits.Load())
		}
	}
}
