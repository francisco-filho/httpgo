//go:build linux || darwin

package e2e

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealCtrlC(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			started := make(chan struct{})
			var next atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if r.URL.Path == "/next" {
					next.Add(1)
					return
				}
				close(started)
				<-r.Context().Done()
			}))
			defer server.Close()
			body := ""
			if method != "GET" {
				body = "\nAuthorization: auth-secret\n{\"password\":\"body-secret\"}"
			}
			file := scenario(t, method+" {{base}}/wait?token=query-secret"+body+"\nHTTP 200\nDELETE {{base}}/next")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary(t), "test", file, "--var", "base="+server.URL)
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
				t.Fatal("process did not handle Ctrl+C")
			}
			if waitErr == nil || command.ProcessState.ExitCode() != 2 || !strings.Contains(stderr.String(), "cancelada") || !strings.Contains(out.String(), "não_iniciadas=1") || next.Load() != 0 {
				t.Fatalf("exit=%v,out=%q,err=%q,next=%d", command.ProcessState, out.String(), stderr.String(), next.Load())
			}
			assertNoSecrets(t, out.String()+stderr.String())
		})
	}
}
