//go:build linux || darwin

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

func TestConfigRealCancellation(t *testing.T) {
	started := make(chan struct{})
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	dir := t.TempDir()
	configFile(t, dir, ".httpgo.yaml", "version: 1\ndefaults: {timeout: 60}\nvariables: {token: fake-cancel-token}\nsecrets: [token]")
	file := configFile(t, dir, "cancel.hurl", "GET "+server.URL+"/{{token}}\nHTTP 200\nGET "+server.URL+"/never\nHTTP 200")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), "test", file)
	command.Dir = dir
	var out, stderr bytes.Buffer
	command.Stdout = &out
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var waitErr error
	go func() { waitErr = command.Wait(); close(done) }()
	defer func() { _ = command.Process.Kill(); <-done }()
	select {
	case <-started:
	case <-done:
		t.Fatalf("exited before request: %v", waitErr)
	case <-ctx.Done():
		t.Fatal("request did not start")
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("process did not cancel")
	}
	if waitErr == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 || !strings.Contains(stderr.String(), "cancelada") || !strings.Contains(out.String(), "concluídas=0") || !strings.Contains(out.String(), "não_iniciadas=1") {
		t.Fatalf("state=%v out=%q err=%q hits=%d", command.ProcessState, out.String(), stderr.String(), hits.Load())
	}
	if strings.Contains(out.String()+stderr.String(), "fake-cancel-token") {
		t.Fatal("secret leaked")
	}
}
