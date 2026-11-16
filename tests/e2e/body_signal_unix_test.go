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

func TestBodyConsumptionRealCancellation(t *testing.T) {
	started := make(chan struct{})
	closed := make(chan struct{})
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("customer body-private"))
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
		close(closed)
	}))
	defer server.Close()
	file := scenario(t, "GET "+server.URL+"/start\nHTTP *\n[Asserts]\nbody not contains \"passwordHash\"\n[Captures]\ntoken: jsonpath \"$.missing\"\nGET "+server.URL+"/never")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), "test", file, "--secret", "token")
	var out, stderr bytes.Buffer
	command.Stdout, command.Stderr = &out, &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() { _ = command.Process.Kill() }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("exited early: %v", err)
	case <-ctx.Done():
		t.Fatal("response not started")
	}
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 || !strings.Contains(stderr.String(), "cancelada") || !strings.Contains(out.String(), "concluídas=0") || !strings.Contains(out.String(), "não_iniciadas=1") || strings.Contains(out.String()+stderr.String(), "body-private") {
			t.Fatalf("err=%v hits=%d out=%s stderr=%s", err, hits.Load(), out.String(), stderr.String())
		}
	case <-ctx.Done():
		t.Fatal("process did not cancel")
	}
	select {
	case <-closed:
	case <-ctx.Done():
		t.Fatal("response connection not closed")
	}
}
