package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var suite struct {
	binary   string
	buildErr error
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "httpgo-e2e-")
	if err != nil {
		suite.buildErr = err
		m.Run()
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()
	suite.binary = filepath.Join(dir, "httpgo")
	if runtime.GOOS == "windows" {
		suite.binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-X main.version=e2e -X main.commit=testcommit -X main.date=2026-11-14", "-o", suite.binary, "./cmd/httpgo")
	command.Dir = filepath.Join("..", "..")
	command.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := command.CombinedOutput(); err != nil {
		suite.buildErr = fmt.Errorf("build: %w: %s", err, output)
	}
	m.Run()
}

func binary(t *testing.T) string {
	t.Helper()
	if suite.buildErr != nil {
		t.Fatal(suite.buildErr)
	}
	return suite.binary
}
func scenario(t *testing.T, text string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "input.hurl")
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), args...)
	var out, stderr bytes.Buffer
	command.Stdout = &out
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		t.Fatalf("binary exceeded harness deadline: %v", ctx.Err())
	}
	code := 0
	if err != nil {
		if command.ProcessState == nil {
			t.Fatal(err)
		}
		code = command.ProcessState.ExitCode()
	}
	return code, out.String(), stderr.String()
}

func TestGetSequence(t *testing.T) {
	for _, first := range []int{200, 503} {
		t.Run(strconv.Itoa(first), func(t *testing.T) {
			var mu sync.Mutex
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				paths = append(paths, r.URL.Path)
				mu.Unlock()
				if r.URL.Path == "/health" {
					w.WriteHeader(first)
				} else {
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			example, err := os.ReadFile("../../testdata/scenarios/scenario_get_status.hurl")
			if err != nil {
				t.Fatal(err)
			}
			file := scenario(t, strings.ReplaceAll(string(example), "http://localhost:8080", server.URL))
			code, out, stderr := run(t, "test", file)
			wantCode, wantCalls := 0, 3
			if first == 503 {
				wantCode, wantCalls = 1, 1
			}
			mu.Lock()
			defer mu.Unlock()
			if code != wantCode || len(paths) != wantCalls || paths[0] != "/health" || !strings.Contains(out, "resumo") {
				t.Fatalf("code=%d,out=%q,err=%q,paths=%v", code, out, stderr, paths)
			}
			if first == 200 {
				if stderr != "" || paths[1] != "/inexistente" || paths[2] != "/health" || !strings.Contains(out, "validadas=3") {
					t.Fatalf("out=%q,err=%q,paths=%v", out, stderr, paths)
				}
			} else if !strings.Contains(stderr, "esperado 200, recebido 503") || !strings.Contains(out, "não_iniciadas=2") {
				t.Fatalf("out=%q,err=%q", out, stderr)
			}
		})
	}
}

func TestUnvalidatedAndRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(500) }))
	defer server.Close()
	address := strings.Replace(server.URL, "http://", "http://username-secret:password-secret@", 1) + "/visible-path?token=query-secret&hidden-name=value-secret#fragment-secret"
	code, out, stderr := run(t, "test", scenario(t, "GET "+address))
	if code != 0 || stderr != "" || !strings.Contains(out, "sem validação de status") || !strings.Contains(out, "recebido 500") || !strings.Contains(out, "/visible-path?[oculta]") {
		t.Fatalf("code=%d,out=%q,err=%q", code, out, stderr)
	}
	assertNoSecrets(t, out+stderr)
}

func TestInputAndUsageNeverSend(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	for _, suffix := range []string{"HEAD " + server.URL, "GET http://username-secret:password-secret@bad%zz/?token=query-secret", "HTTP 999", "HTTP 200 # comment", "[Asserts]"} {
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"\n"+suffix))
		if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
			t.Fatalf("code=%d,out=%q,err=%q,hits=%d", code, out, stderr, hits.Load())
		}
		assertNoSecrets(t, out+stderr)
	}
	for _, text := range []string{"", "# comments only"} {
		code, out, stderr := run(t, "test", scenario(t, text))
		if code != 2 || out != "" || stderr == "" {
			t.Fatalf("code=%d,out=%q,err=%q", code, out, stderr)
		}
	}
	for _, args := range [][]string{nil, {"test"}, {"test", "a", "b"}, {"--unknown"}, {"test", "--unknown"}, {"test", filepath.Join(t.TempDir(), "missing.hurl")}, {"test", t.TempDir()}} {
		code, out, stderr := run(t, args...)
		if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
			t.Fatalf("args=%v,code=%d,out=%q,err=%q", args, code, out, stderr)
		}
	}
}

func TestHelpAndBuildMetadata(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}, {"--version"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || out == "" || stderr != "" || strings.Contains(out, "\x1b") {
			t.Fatalf("args=%v,code=%d,out=%q,err=%q", args, code, out, stderr)
		}
		if args[0] == "--version" {
			if !strings.Contains(out, "httpgo e2e commit=testcommit date=2026-11-14") {
				t.Fatalf("metadata=%q", out)
			}
		} else if !strings.Contains(out, "httpgo test health.hurl") || !strings.Contains(out, "não coloque segredos") {
			t.Fatalf("help=%q", out)
		}
	}
}

func TestTransportFailuresStopSequence(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		for _, mode := range []string{"redirect limit", "cross origin", "invalid location", "TLS", "connection", "incomplete", "no retry"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				var hits, next, targetHits atomic.Int32
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { targetHits.Add(1); w.WriteHeader(200) }))
				defer target.Close()
				source := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/next" {
						next.Add(1)
						w.WriteHeader(200)
						return
					}
					hits.Add(1)
					switch mode {
					case "redirect limit":
						n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/"))
						w.Header().Set("Location", fmt.Sprintf("/%d?token=query-secret", n+1))
						w.WriteHeader(302)
					case "cross origin":
						w.Header().Set("Location", strings.Replace(target.URL, "http://", "http://username-secret:password-secret@", 1)+"/?token=destination-secret")
						w.WriteHeader(302)
					case "invalid location":
						w.Header().Set("Location", "http://bad%zz/?token=destination-secret")
						w.WriteHeader(302)
					case "incomplete":
						w.Header().Set("Content-Length", "100")
						w.WriteHeader(200)
						_, _ = io.WriteString(w, "short")
					case "no retry":
						connection, _, err := w.(http.Hijacker).Hijack()
						if err != nil {
							t.Error(err)
							return
						}
						_ = connection.Close()
					default:
						w.WriteHeader(200)
					}
				}))
				source.Config.ErrorLog = log.New(io.Discard, "", 0)
				if mode == "TLS" {
					source.StartTLS()
				} else {
					source.Start()
				}
				defer source.Close()
				address := source.URL + "/0?token=query-secret"
				if mode == "connection" {
					listener, err := net.Listen("tcp", "127.0.0.1:0")
					if err != nil {
						t.Fatal(err)
					}
					address = "http://" + listener.Addr().String() + "/visible-path?token=query-secret"
					if err := listener.Close(); err != nil {
						t.Fatal(err)
					}
				}
				body := "\nAuthorization: auth-secret\n{\"password\":{{password}}}"
				if method == "GET" {
					body = ""
				}
				code, out, stderr := run(t, "test", scenario(t, method+" {{target}}"+body+"\nHTTP 200\nDELETE {{base}}/next\nHTTP 200"), "--var", "target="+address, "--var", "base="+source.URL, "--var", `password="body-secret"`, "--secret", "password")
				if code != 2 || !strings.Contains(out, "erro de transporte") || !strings.Contains(out, "concluídas=0") || !strings.Contains(out, "não_iniciadas=1") || stderr == "" || next.Load() != 0 || targetHits.Load() != 0 {
					t.Fatalf("code=%d,out=%q,err=%q,hits=%d,next=%d,target=%d", code, out, stderr, hits.Load(), next.Load(), targetHits.Load())
				}
				if mode == "redirect limit" && hits.Load() != 11 {
					t.Fatalf("hits=%d,want 11", hits.Load())
				}
				if mode == "no retry" && hits.Load() != 1 {
					t.Fatalf("retry: hits=%d", hits.Load())
				}
				assertNoSecrets(t, out+stderr)
			})
		}
	}

}
func TestPublicTimeoutIncludesBody(t *testing.T) {
	for _, method := range []string{"GET", "POST"} {
		t.Run(method, func(t *testing.T) {
			var next atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if r.URL.Path == "/next" {
					next.Add(1)
					return
				}
				w.Header().Set("X-Ready", "yes")
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/body")
					w.WriteHeader(307)
					return
				}
				w.WriteHeader(200)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer server.Close()
			started := time.Now()
			body, captures := "", "\n[Captures]\ntoken: jsonpath \"$\""
			if method == "POST" {
				body = "\nAuthorization: auth-secret\n{\"password\":{{password}}}"
				captures = ""
			}
			code, out, stderr := run(t, "test", scenario(t, method+" {{base}}/start"+body+"\nHTTP *\n[Asserts]\nheader \"X-Ready\" == \"yes\""+captures+"\nDELETE {{base}}/next"), "--var", "base="+server.URL, "--var", `password="body-secret"`, "--secret", "password")
			if code != 2 || !strings.Contains(stderr, "timeout") || !strings.Contains(out, "concluídas=0") || next.Load() != 0 || time.Since(started) < 10*time.Second {
				t.Fatalf("code=%d,out=%q,err=%q,next=%d,elapsed=%s", code, out, stderr, next.Load(), time.Since(started))
			}
			assertNoSecrets(t, out+stderr)
		})
	}
}

func assertNoSecrets(t *testing.T, text string) {
	t.Helper()
	for _, secret := range []string{"username-secret", "password-secret", "query-secret", "value-secret", "hidden-name", "fragment-secret", "destination-secret", "bad%zz", "auth-secret", "proxy-secret", "cookie-secret", "set-cookie-secret", "body-secret"} {
		if strings.Contains(text, secret) {
			t.Errorf("leaked %q in %q", secret, text)
		}
	}
}
