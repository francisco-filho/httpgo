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

func TestJUnitRealWriteAndPermissionFailures(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(503)
	}))
	defer server.Close()
	dir, err := os.MkdirTemp("", "httpgo-junit-permissions-")
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
	denied := configFile(t, dir, "denied.xml", "unchanged")
	if err := os.Chmod(denied, 0400); err != nil {
		t.Fatal(err)
	}
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
	for _, destination := range []string{denied, "/dev/full"} {
		hits.Store(0)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, executable, "test", file, "--output=junit", "--output-file", destination)
		command.Dir = dir
		if os.Geteuid() == 0 {
			command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
		}
		var out, stderr bytes.Buffer
		command.Stdout, command.Stderr = &out, &stderr
		err := command.Run()
		cancel()
		if err == nil || command.ProcessState == nil || command.ProcessState.ExitCode() != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "não foi possível gravar relatório JUnit") || strings.Contains(stderr.String(), "publicado") || hits.Load() != 1 {
			t.Fatalf("destination=%q state=%v err=%v hits=%d out=%q stderr=%q", destination, command.ProcessState, err, hits.Load(), out.String(), stderr.String())
		}
	}
	data, err := os.ReadFile(denied)
	if err != nil || string(data) != "unchanged" {
		t.Fatalf("unwritable destination changed: data=%q err=%v", data, err)
	}
}
