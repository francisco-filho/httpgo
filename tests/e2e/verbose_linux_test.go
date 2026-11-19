//go:build linux

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestVerboseRealTTYAndPipes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"final":true}`)
	}))
	defer server.Close()
	file := scenario(t, "POST "+server.URL+"\n{}\nHTTP 200")
	for _, level := range []string{"-v", "-vv"} {
		for _, outTTY := range []bool{false, true} {
			for _, errTTY := range []bool{false, true} {
				for _, noColor := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/out=%t/err=%t/noColor=%t", level, outTTY, errTTY, noColor), func(t *testing.T) {
						p := startTerminalProcess(t, outTTY, errTTY, noColor, "test", file, level)
						if code := p.wait(t); code != 1 {
							t.Fatalf("code=%d err=%q", code, p.stderr.text())
						}
						out, stderr := p.out.text(), p.stderr.text()
						color := outTTY && errTTY && !noColor
						if strings.Contains(out+stderr, "\r") || strings.Contains(out, "\x1b") || strings.Contains(stderr, "\x1b[2K") || strings.Contains(stderr, "\x1b") != color || strings.Contains(stderr, "\x1b[1m"+filepath.Base(file)+"\x1b[0m") != color || !strings.Contains(out, "Sucesso/erros: 0/1") || strings.Contains(out, "POST") || strings.Contains(stderr, "Testes executados:") || strings.Contains(stderr, "[response body]") != (level == "-vv") {
							t.Fatalf("out=%q err=%q", out, stderr)
						}
					})
				}
			}
		}
	}
}

func TestVerboseRealTTYPendingAndCancellation(t *testing.T) {
	for _, level := range []string{"-v", "-vv"} {
		for _, cancelled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cancel=%t", level, cancelled), func(t *testing.T) {
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if r.URL.Path == "/hold" {
						close(entered)
						select {
						case <-release:
						case <-r.Context().Done():
							return
						}
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"private-success":true}`)
				}))
				defer func() { once.Do(func() { close(release) }); server.Close() }()
				dir := t.TempDir()
				ttyScenario(t, dir, "a/same.httpgo", "GET "+server.URL+"/first\nHTTP *")
				ttyScenario(t, dir, "b/same.httpgo", "GET "+server.URL+"/hold\nHTTP 200")
				ttyScenario(t, dir, "c/last.hurl", "GET "+server.URL+"/last")
				p := startTerminalProcess(t, true, true, true, "test", dir, level)
				select {
				case <-entered:
				case <-p.ctx.Done():
					t.Fatal("HTTP not reached")
				}
				p.stderr.waitFor(t, p.ctx, "[OK] GET "+server.URL+"/first")
				if strings.Count(p.stderr.text(), "same.httpgo\n") != 2 || strings.Contains(p.stderr.text(), "/hold (") || p.out.text() != "" {
					t.Fatalf("premature result: out=%q err=%q", p.out.text(), p.stderr.text())
				}
				if cancelled {
					if err := p.command.Process.Signal(os.Interrupt); err != nil {
						t.Fatal(err)
					}
				} else {
					once.Do(func() { close(release) })
				}
				code := p.wait(t)
				out, stderr := p.out.text(), p.stderr.text()
				wantCode, wantCalls := 0, int32(3)
				if cancelled {
					wantCode, wantCalls = 2, 2
				}
				if code != wantCode || calls.Load() != wantCalls || strings.Contains(stderr, "\r") || strings.Contains(out+stderr, "private-success") {
					t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls.Load(), out, stderr)
				}
				if cancelled && (!strings.Contains(stderr, "last.hurl NÃO EXECUTADO") || !strings.Contains(stderr, "[transporte]") || !strings.Contains(out, "Não executados: 1")) {
					t.Fatalf("missing cancellation: %q %q", out, stderr)
				}
			})
		}
	}
}

func TestVerboseRealOutputFailure(t *testing.T) {
	for _, stream := range []string{"stderr", "stdout"} {
		t.Run(stream, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(503)
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			file := scenario(t, "POST "+server.URL+"\n{}\nHTTP 200\nGET "+server.URL+"/blocked")
			full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = full.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary(t), "test", file, "-vv")
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			wantCalls := int32(0)
			if stream == "stderr" {
				cmd.Stderr = full
			} else {
				cmd.Stdout = full
				wantCalls = 1
			}
			err = cmd.Run()
			if err == nil || ctx.Err() != nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 2 || calls.Load() != wantCalls {
				t.Fatalf("err=%v state=%v calls=%d out=%q stderr=%q", err, cmd.ProcessState, calls.Load(), out.String(), stderr.String())
			}
		})
	}
}
