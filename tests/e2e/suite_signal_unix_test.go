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

func TestSuiteRealCancellationStopsLaterFiles(t *testing.T) {
	started := make(chan struct{})
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/first" {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
			return
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	dir := t.TempDir()
	configFile(t, dir, "01.hurl", "GET "+server.URL+"/first\nHTTP 200")
	configFile(t, dir, "02.hurl", "GET "+server.URL+"/never\nHTTP 200")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), "test", dir)
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
		t.Fatalf("exited: %v", waitErr)
	case <-ctx.Done():
		t.Fatal("no request")
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("did not cancel")
	}
	if waitErr == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 || !strings.Contains(out.String(), "não_executados=1") || !strings.Contains(stderr.String(), "cancelada") {
		t.Fatalf("state=%v hits=%d out=%q stderr=%q", command.ProcessState, hits.Load(), out.String(), stderr.String())
	}
}
