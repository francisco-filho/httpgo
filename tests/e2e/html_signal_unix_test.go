//go:build linux || darwin

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

func TestHTMLRealCancellationStillGeneratesAvailableResults(t *testing.T) {
	started := make(chan struct{})
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/first" {
			w.WriteHeader(200)
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	configFile(t, dir, "01.hurl", "GET "+server.URL+"/first\nHTTP 200\nGET "+server.URL+"/never\nHTTP 200")
	configFile(t, dir, "02.hurl", "GET "+server.URL+"/never\nHTTP 200")
	base := filepath.Join(t.TempDir(), "report.html")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), "test", dir, "--output=html", "--output-file", base)
	var out, stderr bytes.Buffer
	command.Stdout, command.Stderr = &out, &stderr
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
		t.Fatalf("exited early: %v", waitErr)
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
	_, html := htmlArtifact(t, base)
	if waitErr == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 || out.Len() != 0 || !strings.Contains(stderr.String(), "cancelada") || !strings.Contains(stderr.String(), "relatório HTML gerado em") || !strings.Contains(html, "não iniciadas=1") || !strings.Contains(html, "<strong>não executado</strong>") {
		t.Fatalf("state=%v hits=%d out=%q err=%q HTML=%s", command.ProcessState, hits.Load(), out.String(), stderr.String(), html)
	}
}
