//go:build linux

package e2e

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
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
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// A real Linux pseudoterminal, without a runtime or test dependency on C.
func TestPlainOutputOnRealTTY(t *testing.T) {
	for _, noColor := range []string{"", "1"} {
		for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
			t.Run(strings.Join(args, " ")+"/NO_COLOR="+noColor, func(t *testing.T) {
				t.Setenv("NO_COLOR", noColor)
				master, slave := openPTY(t)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				command := exec.CommandContext(ctx, binary(t), args...)
				var stderr bytes.Buffer
				command.Stdout = slave
				command.Stderr = &stderr
				if err := command.Start(); err != nil {
					t.Fatal(err)
				}
				// Drain while the child writes: help may exceed the PTY buffer. Waiting
				// for process exit before reading deadlocks once that buffer fills.
				var output []byte
				var readErr error
				readDone := make(chan struct{})
				go func() { output, readErr = io.ReadAll(master); close(readDone) }()
				waitErr := command.Wait()
				closeErr := slave.Close()
				select {
				case <-readDone:
				case <-ctx.Done():
					_ = master.Close()
					t.Fatal("TTY reader did not finish")
				}
				if waitErr != nil {
					t.Fatal(waitErr)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				if readErr != nil && !errors.Is(readErr, syscall.EIO) {
					t.Fatal(readErr)
				}
				if stderr.Len() != 0 || bytes.Contains(output, []byte("\x1b")) {
					t.Fatalf("stdout=%q,stderr=%q", output, stderr.String())
				}
				plain := strings.ReplaceAll(string(output), "\r\n", "\n")
				assertCompactProcessHelp(t, plain)
				code, redirected, diagnostics := run(t, args...)
				if code != 0 || diagnostics != "" || plain != redirected {
					t.Fatalf("TTY differs from redirected output: tty=%q redirected=%q code=%d stderr=%q", plain, redirected, code, diagnostics)
				}
			})
		}
	}
}

// Each stream has its own PTY or pipe: terminal selection must not infer stderr
// capabilities from stdout. Readers drain concurrently and expose event barriers.
func openPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = master.Close() })
	var value uint32
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), 0x40045431, uintptr(unsafe.Pointer(&value))); errno != 0 {
		t.Fatal(errno)
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), 0x80045430, uintptr(unsafe.Pointer(&value))); errno != 0 {
		t.Fatal(errno)
	}
	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", value), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = slave.Close() })
	return master, slave
}

type terminalLog struct {
	mu      sync.Mutex
	data    bytes.Buffer
	changed chan struct{}
	done    chan struct{}
	err     error
}

func (l *terminalLog) text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.ReplaceAll(l.data.String(), "\r\n", "\n")
}

func (l *terminalLog) waitFor(t *testing.T, ctx context.Context, text string) {
	t.Helper()
	for {
		if strings.Contains(l.text(), text) {
			return
		}
		select {
		case <-l.changed:
		case <-l.done:
			t.Fatalf("stream ended before %q: %q", text, l.text())
		case <-ctx.Done():
			t.Fatalf("waiting for %q: %v; output=%q", text, ctx.Err(), l.text())
		}
	}
}

func processStream(t *testing.T, tty bool) (*os.File, *os.File, *terminalLog) {
	t.Helper()
	var reader, writer *os.File
	if tty {
		reader, writer = openPTY(t)
	} else {
		var err error
		reader, writer, err = os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	}
	return reader, writer, &terminalLog{changed: make(chan struct{}, 1), done: make(chan struct{})}
}

func (l *terminalLog) drain(reader *os.File) {
	defer close(l.done)
	data := make([]byte, 4096)
	for {
		n, err := reader.Read(data)
		if n > 0 {
			l.mu.Lock()
			_, _ = l.data.Write(data[:n])
			l.mu.Unlock()
			select {
			case l.changed <- struct{}{}:
			default:
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) {
				l.err = err
			}
			return
		}
	}
}

type terminalProcess struct {
	command     *exec.Cmd
	ctx         context.Context
	out, stderr *terminalLog
}

func startTerminalProcess(t *testing.T, outTTY, errTTY, noColor bool, args ...string) *terminalProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	outRead, outWrite, out := processStream(t, outTTY)
	errRead, errWrite, stderr := processStream(t, errTTY)
	command := exec.CommandContext(ctx, binary(t), args...)
	command.Env = filteredEnvironment("NO_COLOR")
	if noColor {
		command.Env = append(command.Env, "NO_COLOR=")
	}
	command.Stdout, command.Stderr = outWrite, errWrite
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	go out.drain(outRead)
	go stderr.drain(errRead)
	_ = outWrite.Close()
	_ = errWrite.Close()
	t.Cleanup(func() {
		cancel()
		if command.ProcessState == nil {
			_ = command.Wait()
		}
		<-out.done
		<-stderr.done
	})
	return &terminalProcess{command: command, ctx: ctx, out: out, stderr: stderr}
}

func filteredEnvironment(name string) []string {
	var result []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, name+"=") {
			result = append(result, value)
		}
	}
	return result
}

func (p *terminalProcess) wait(t *testing.T) int {
	t.Helper()
	err := p.command.Wait()
	<-p.out.done
	<-p.stderr.done
	if p.ctx.Err() != nil || p.out.err != nil || p.stderr.err != nil {
		t.Fatalf("process=%v out=%v err=%v", p.ctx.Err(), p.out.err, p.stderr.err)
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatal(err)
		}
		return exit.ExitCode()
	}
	return 0
}

func ttyScenario(t *testing.T, dir, name, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCompactRealTTYPendingAndOutcomes(t *testing.T) {
	for _, mode := range []string{"pass", "fourth failure", "retry pass", "retry exhausted", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var unblock sync.Once
			defer unblock.Do(func() { close(release) })
			var held atomic.Bool
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/hold" {
					if held.CompareAndSwap(false, true) {
						close(entered)
						select {
						case <-release:
						case <-r.Context().Done():
							return
						}
					}
					if mode == "retry exhausted" || (mode == "retry pass" && calls.Load() == 2) {
						w.WriteHeader(503)
						return
					}
				}
				if r.URL.Path == "/4" && mode == "fourth failure" {
					w.WriteHeader(503)
				}
			}))
			defer func() {
				unblock.Do(func() { close(release) })
				server.Close()
			}()
			dir := t.TempDir()
			ttyScenario(t, dir, "a/same.httpgo", "GET "+server.URL+"/first")
			text := "GET " + server.URL + "/hold\nHTTP 200\n"
			total := 10
			if strings.HasPrefix(mode, "retry") {
				text = "GET " + server.URL + "/hold\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200\n"
				total = 1
			} else {
				for i := 2; i <= 10; i++ {
					text += fmt.Sprintf("GET %s/%d\nHTTP *\n", server.URL, i)
				}
				if mode == "fourth failure" {
					text = strings.Replace(text, "/4\nHTTP *", "/4\nHTTP 200", 1)
				}
			}
			ttyScenario(t, dir, "b/same.httpgo", text)
			ttyScenario(t, dir, "c/last.hurl", "GET "+server.URL+"/last")
			args := []string{"test", dir}
			if mode == "fourth failure" || mode == "retry exhausted" {
				args = append(args, "--fail-fast")
			}
			p := startTerminalProcess(t, true, true, true, args...)
			select {
			case <-entered:
			case <-p.ctx.Done():
				t.Fatal("request never reached server")
			}
			p.stderr.waitFor(t, p.ctx, fmt.Sprintf("0/%d", total))
			pending := p.stderr.text()
			if !strings.Contains(pending, "OK  1/1\n") || p.out.text() != "" || strings.Contains(pending, "- b/") {
				t.Fatalf("pending out=%q err=%q", p.out.text(), pending)
			}
			if mode == "cancel" {
				if err := p.command.Process.Signal(os.Interrupt); err != nil {
					t.Fatal(err)
				}
			} else {
				unblock.Do(func() { close(release) })
			}
			code := p.wait(t)
			wantCode := 0
			wantCalls := int32(12)
			wantFinal := "OK  10/10"
			wantCount := "Cenários executados: 3\n"
			wantTests := 12
			wantStates := "Sucesso/erros: 3/0\n"
			switch mode {
			case "fourth failure":
				wantCode = 1
				wantCalls = 5
				wantFinal = "ERRO  4/10\n  GET " + server.URL + "/4 -> step 4 - Linha"
				wantTests = 5
				wantCount = "Cenários executados: 2\n"
				wantStates = "Sucesso/erros: 1/1\nNão executados: 1\n"
			case "retry pass":
				wantTests = 3
				wantCalls = 4
				wantFinal = "OK  1/1"
			case "retry exhausted":
				wantCode = 1
				wantCalls = 3
				wantFinal = "ERRO  1/1\n  GET " + server.URL + "/hold -> step 1 - Linha"
				wantTests = 2
				wantCount = "Cenários executados: 2\n"
				wantStates = "Sucesso/erros: 1/1\nNão executados: 1\n"
			case "cancel":
				wantCode = 2
				wantCalls = 2
				wantFinal = "ERRO  1/10\n  GET " + server.URL + "/hold -> step 1 - Linha"
				wantTests = 2
				wantCount = "Cenários executados: 2\n"
				wantStates = "Sucesso/erros: 1/1\nNão executados: 1\n"
			}
			out, diagnostic := p.out.text(), p.stderr.text()
			if code != wantCode || calls.Load() != wantCalls || !strings.Contains(diagnostic, wantFinal) || !strings.Contains(out, wantCount) || !strings.Contains(out, fmt.Sprintf("Testes executados: %d\n", wantTests)) || !strings.Contains(out, wantStates) {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls.Load(), out, diagnostic)
			}
			if strings.Contains(diagnostic, "\x1b[32m") || strings.Contains(out, "\x1b") || strings.Contains(diagnostic, "status validado") || strings.Count(diagnostic, "- same.httpgo  OK  1/1\n") < 1 {
				t.Fatalf("unexpected output: out=%q err=%q", out, diagnostic)
			}
			if wantCode != 0 && !strings.Contains(diagnostic, "NÃO EXECUTADO  0/?") {
				t.Fatalf("missing skipped: %q", diagnostic)
			}
		})
	}
}

func TestCompactRealTTYSelection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	file := scenario(t, "GET "+server.URL+"\nHTTP 200")
	for _, outTTY := range []bool{false, true} {
		for _, errTTY := range []bool{false, true} {
			for _, noColor := range []bool{false, true} {
				t.Run(fmt.Sprintf("out=%t/err=%t/noColor=%t", outTTY, errTTY, noColor), func(t *testing.T) {
					p := startTerminalProcess(t, outTTY, errTTY, noColor, "test", file)
					if code := p.wait(t); code != 0 {
						t.Fatalf("code=%d err=%q", code, p.stderr.text())
					}
					out, diagnostic := p.out.text(), p.stderr.text()
					if outTTY && errTTY {
						if !strings.HasPrefix(out, "Cenários executados: 1\n") || !strings.Contains(out, "Testes executados: 1\n") || strings.Contains(out+diagnostic, "Sucesso: ") || strings.Contains(out+diagnostic, "-----------------------------------------------") || !strings.Contains(diagnostic, "\x1b[2K") || strings.Contains(diagnostic, "\x1b[32m") == noColor {
							t.Fatalf("out=%q err=%q", out, diagnostic)
						}
					} else if !strings.Contains(out, "GET ") || !strings.Contains(out, "Cenários executados: 1\n") || !strings.Contains(out, "Testes executados: 1\n") || diagnostic != "" || strings.Contains(out, "\x1b") || strings.Contains(out, "Sucesso: ") || strings.Contains(out, "-----------------------------------------------") {
						t.Fatalf("legacy out=%q err=%q", out, diagnostic)
					}
				})
			}
		}
	}
}

func TestCompactRealTTYLoadErrorsAndProtection(t *testing.T) {
	const secret = "fiction-tty-token"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/login" {
			_, _ = fmt.Fprintf(w, `{"token":%q}`, secret)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	ttyScenario(t, dir, "01.httpgo", "GET "+server.URL+"/login\n[Captures]\ntoken: jsonpath \"$.token\"")
	ttyScenario(t, dir, "02-"+secret+".httpgo", "GET "+server.URL+"\nHTTP 200\n[Asserts]\nbody contains \""+secret+"\"")
	ttyScenario(t, dir, "03.httpgo", "GET "+server.URL+"/not-sent\n[Unsupported]")
	ttyScenario(t, dir, "04.httpgo", "GET "+server.URL+"/{{missing}}")
	p := startTerminalProcess(t, true, true, true, "test", dir, "--secret", "token")
	code := p.wait(t)
	out, diagnostic := p.out.text(), p.stderr.text()
	if code != 2 || calls.Load() != 2 || !strings.Contains(out, "Cenários executados: 4\n") || !strings.Contains(out, "Testes executados: 3\n") || !strings.Contains(out, "Sucesso/erros: 1/3\n") || !strings.Contains(out, "Sucesso/erros: 1/2\n") || !strings.Contains(diagnostic, "ERRO  0/?\n") || !strings.Contains(diagnostic, "step 1 - Linha") || !strings.Contains(diagnostic, "[oculto]") || strings.Contains(out+diagnostic, secret) || strings.Contains(out, "Não executados:") {
		t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls.Load(), out, diagnostic)
	}
}

func TestCompactRealTTYReportsAndStaticCommands(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	file := scenario(t, "GET "+server.URL+"\nHTTP 200")
	for _, mode := range []string{"junit", "html", "--help", "--version"} {
		t.Run(mode, func(t *testing.T) {
			args := []string{mode}
			destination := filepath.Join(t.TempDir(), "report."+mode)
			if mode == "junit" || mode == "html" {
				args = []string{"test", file, "--output", mode, "--output-file", destination}
			}
			p := startTerminalProcess(t, true, true, false, args...)
			if code := p.wait(t); code != 0 {
				t.Fatalf("code=%d err=%q", code, p.stderr.text())
			}
			out, diagnostic := p.out.text(), p.stderr.text()
			if strings.Contains(out+diagnostic, "\x1b") || strings.Contains(out+diagnostic, "Testes executados:") {
				t.Fatalf("out=%q err=%q", out, diagnostic)
			}
			if mode == "junit" || mode == "html" {
				if mode == "html" {
					matches, err := filepath.Glob(strings.TrimSuffix(destination, ".html") + "-*.html")
					if err != nil || len(matches) != 1 {
						t.Fatalf("reports=%v err=%v", matches, err)
					}
					destination = matches[0]
				}
				data, err := os.ReadFile(destination)
				if err != nil {
					t.Fatal(err)
				}
				if out != "" {
					t.Fatalf("report stdout=%q", out)
				}
				if mode == "junit" {
					var report struct {
						XMLName xml.Name `xml:"testsuite"`
						Tests   int      `xml:"tests,attr"`
					}
					if err := xml.Unmarshal(data, &report); err != nil || report.Tests != 1 {
						t.Fatalf("invalid JUnit: %s; err=%v", data, err)
					}
				} else if !bytes.Contains(data, []byte("<html")) {
					t.Fatalf("invalid HTML: %s", data)
				}
			}
		})
	}
}
