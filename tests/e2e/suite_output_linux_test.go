package e2e

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

func TestSuiteRealOutputFailureStopsNextFile(t *testing.T) {
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				if stream == "stderr" {
					w.WriteHeader(503)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			dir := t.TempDir()
			configFile(t, dir, "01.hurl", "GET "+server.URL+"/first\nHTTP 200")
			configFile(t, dir, "02.hurl", "GET "+server.URL+"/never\nHTTP 200")
			full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = full.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary(t), "test", dir)
			var out, stderr bytes.Buffer
			command.Stdout = &out
			command.Stderr = &stderr
			if stream == "stdout" {
				command.Stdout = full
			} else {
				command.Stderr = full
			}
			err = command.Run()
			if err == nil || command.ProcessState == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 {
				t.Fatalf("state=%v err=%v hits=%d out=%q stderr=%q", command.ProcessState, err, hits.Load(), out.String(), stderr.String())
			}
		})
	}
}
