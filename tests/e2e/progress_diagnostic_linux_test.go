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

func TestCompactRealTTYMultilineDiagnostic(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		t.Run(fmt.Sprintf("NO_COLOR=%t", noColor), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"orders":[{"number":7,"items":[{"allergens":["private-body-allergen"]}]}]}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			var text strings.Builder
			for step := 1; step <= 7; step++ {
				fmt.Fprintf(&text, "GET %s/first/%d\nHTTP 200\n", server.URL, step)
			}
			text.WriteString("[Asserts]\n")
			for strings.Count(text.String(), "\n") < 113 {
				text.WriteByte('\n')
			}
			text.WriteString(`jsonpath "$.orders[?(@.number==7)].items[0].allergens" contains "SOY"` + "\n")
			for step := 8; step <= 15; step++ {
				fmt.Fprintf(&text, "GET %s/not-reached/%d\nHTTP 200\n", server.URL, step)
			}
			ttyScenario(t, dir, "27-phase-six-advanced-menu.hurl", text.String())
			ttyScenario(t, dir, "29-phase-seven-combos.hurl", strings.Repeat("GET "+server.URL+"/next\nHTTP 200\n", 13))
			p := startTerminalProcess(t, true, true, noColor, "test", dir)
			if code := p.wait(t); code != 1 || calls.Load() != 20 {
				t.Fatalf("code=%d sends=%d err=%q", code, calls.Load(), p.stderr.text())
			}
			diagnostic := p.stderr.text()
			message := `expected "SOY" => received tipo array`
			if !noColor {
				message = "\x1b[31m" + message + "\x1b[0m"
			}
			block := "  GET " + server.URL + "/first/7 -> step 7 - Linha 114:1\n  [assertion]\n  jsonpath \"$.orders[?(@.number==[literal oculto])].items[0].allergens\" contains\n     " + message + "\n"
			index := strings.Index(diagnostic, block)
			nextName := "29-phase-seven-combos.hurl"
			if !noColor {
				nextName = "\x1b[1m" + nextName + "\x1b[0m"
			}
			next := strings.Index(diagnostic, "- "+nextName)
			if index < 0 || next < index+len(block) || !strings.Contains(diagnostic, "13/13\n") || strings.Contains(diagnostic, "private-body-allergen") || strings.Contains(diagnostic, "@.number==7") || strings.Contains(diagnostic, "execução interrompida") {
				t.Fatalf("missing safe ordered block %q in %q", block, diagnostic)
			}
			if !strings.Contains(p.out.text(), "Sucesso/erros: 1/1\n") {
				t.Fatalf("summary=%q", p.out.text())
			}
		})
	}
}

// GO-597–GO-601: the binary starts each compact step failure with its endpoint.
func TestCompactRealTTYEndpointBlocks(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		t.Run(fmt.Sprintf("NO_COLOR=%t", noColor), func(t *testing.T) {
			const secret = "fiction-endpoint-tty"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/login":
					w.Header().Set("Content-Type", "application/json")
					_, _ = fmt.Fprintf(w, `{"token":%q}`, secret)
				case "/status":
					w.WriteHeader(500)
				}
			}))
			defer server.Close()
			dir := t.TempDir()
			ttyScenario(t, dir, "01-pass.httpgo", "GET "+server.URL+"/ok\nHTTP 200")
			ttyScenario(t, dir, "02-status.httpgo", "GET "+server.URL+"/status\nHTTP 200")
			ttyScenario(t, dir, "03-assert.httpgo", "GET "+server.URL+"/login\n[Captures]\ntoken: jsonpath \"$.token\"\nGET "+server.URL+"/{{token}}?q={{token}}\nHTTP 200\n[Asserts]\nbody contains \"expected\"")
			ttyScenario(t, dir, "04-prepare.httpgo", "GET "+server.URL+"/{{missing}}")
			ttyScenario(t, dir, "05-parse.httpgo", "GET "+server.URL+"/ok\n[Unsupported]")
			ttyScenario(t, dir, "06-after.httpgo", "GET "+server.URL+"/after\nHTTP 200")
			p := startTerminalProcess(t, true, true, noColor, "test", dir, "--secret", "token")
			if code := p.wait(t); code != 2 {
				t.Fatalf("code=%d err=%q", code, p.stderr.text())
			}
			diagnostic := p.stderr.text()
			statusBlock := "  GET " + server.URL + "/status -> step 1 - Linha 2:1\n  [status]\n  HTTP 200\n"
			assertBlock := "  GET " + server.URL + "/[oculto]?[oculta] -> step 2 - Linha 7:1\n  [assertion]\n  body contains\n"
			prepareBlock := "  GET [URL não renderizada] -> step 1 - Linha 1:28\n  [renderização]\n"
			for _, block := range []string{statusBlock, assertBlock, prepareBlock} {
				if !strings.Contains(diagnostic, block) {
					t.Fatalf("missing %q in %q", block, diagnostic)
				}
			}
			for _, message := range []string{"expected 200 => received 500", "erro de renderização: variável ausente"} {
				if !strings.Contains(diagnostic, message) {
					t.Fatalf("missing message %q in %q", message, diagnostic)
				}
			}
			// An approved step keeps its single progress line, without any request line.
			if !noColor && !strings.Contains(diagnostic, "OK\x1b[0m  1/1\n\r\x1b[2K- \x1b[1m02-status.httpgo") {
				t.Fatalf("approved step leaked: %q", diagnostic)
			}
			if noColor && !strings.Contains(diagnostic, "- 01-pass.httpgo  OK  1/1\n\r\x1b[2K- 02-status.httpgo") {
				t.Fatalf("approved step leaked: %q", diagnostic)
			}
			// A rejected file has no endpoint and no invented step.
			start := strings.Index(diagnostic, "05-parse.httpgo")
			if start < 0 {
				t.Fatalf("missing parse file: %q", diagnostic)
			}
			parseBlock := diagnostic[start:]
			if end := strings.Index(parseBlock, "06-after.httpgo"); end >= 0 {
				parseBlock = parseBlock[:end]
			}
			if !strings.Contains(parseBlock, "0/?\n") || !strings.Contains(parseBlock, "[parsing]") || strings.Contains(parseBlock, " -> step") {
				t.Fatalf("parse block invented endpoint: %q", parseBlock)
			}
			if strings.Contains(diagnostic, secret) || strings.Contains(diagnostic, "?q=") {
				t.Fatalf("secret leaked: %q", diagnostic)
			}
		})
	}
}

// GO-601, GO-603: the endpoint belongs only to compact text on a real TTY.
func TestCompactEndpointOnlyInTextCompact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer server.Close()
	file := scenario(t, "GET "+server.URL+"/fail\nHTTP 200")
	endpoint := "  GET " + server.URL + "/fail -> step 1 - Linha 2:1\n"

	t.Run("compact", func(t *testing.T) {
		p := startTerminalProcess(t, true, true, true, "test", file)
		if code := p.wait(t); code != 1 || !strings.Contains(p.stderr.text(), endpoint) {
			t.Fatalf("code=%d err=%q", code, p.stderr.text())
		}
	})
	t.Run("redirected", func(t *testing.T) {
		code, out, stderr := run(t, "test", file)
		if code != 1 || !strings.Contains(out, "GET "+server.URL+"/fail: status divergente") || !strings.Contains(stderr, "status divergente: esperado 200, recebido 503") || strings.Contains(out+stderr, " -> step") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	})
	for _, verbosity := range []string{"-v", "-vv"} {
		t.Run("verbose"+verbosity, func(t *testing.T) {
			code, out, stderr := run(t, "test", file, verbosity)
			if code != 1 || !strings.Contains(stderr, "[ERRO] GET "+server.URL+"/fail") || !strings.Contains(stderr, "  step 1 - Linha 2:1\n  [status]") {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
			if strings.Contains(stderr, " -> step") || strings.Count(stderr, server.URL+"/fail") != 1 {
				t.Fatalf("duplicated endpoint: %q", stderr)
			}
		})
	}
	for _, format := range []string{"junit", "html"} {
		t.Run(format, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report."+format)
			code, out, stderr := run(t, "test", file, "--output", format, "--output-file", path)
			if code != 1 {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
			data := ""
			if format == "html" {
				_, data = htmlArtifact(t, path)
			} else {
				report, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				data = string(report)
			}
			if strings.Contains(data, " -> step") || !strings.Contains(data, "status divergente") {
				t.Fatalf("report changed: %q", data)
			}
		})
	}
}

// GO-604: a compact write failure exits 2 and never reaches the next file.
func TestCompactRealTTYEndpointWriteFailure(t *testing.T) {
	var hits atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/hold" {
			once.Do(func() { close(entered) })
			<-release
		}
		w.WriteHeader(503)
	}))
	defer server.Close()
	defer func() {
		once.Do(func() { close(entered) })
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	dir := t.TempDir()
	ttyScenario(t, dir, "01-hold.httpgo", "GET "+server.URL+"/hold\nHTTP 200")
	ttyScenario(t, dir, "02-never.httpgo", "GET "+server.URL+"/never\nHTTP 200")
	outRead, outWrite := openPTY(t)
	errRead, errWrite := openPTY(t)
	defer func() { _ = outRead.Close(); _ = errRead.Close(); _ = outWrite.Close(); _ = errWrite.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), "test", dir)
	command.Env = append(filteredEnvironment("NO_COLOR"), "NO_COLOR=")
	command.Stdout, command.Stderr = outWrite, errWrite
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = outWrite.Close()
	_ = errWrite.Close()
	// stderr is deliberately left unread: no pending read keeps the master fd
	// alive, so closing it fails every subsequent write from the child.
	var out bytes.Buffer
	outDone := make(chan struct{})
	go func() { defer close(outDone); _, _ = io.Copy(&out, outRead) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatalf("request never reached server: %v", ctx.Err())
	}
	// Break the stderr PTY while the step is pending: the presentation fails
	// before the diagnostic write and the suite must stop with code 2.
	if err := errRead.Close(); err != nil {
		t.Fatal(err)
	}
	close(release)
	waitErr := command.Wait()
	<-outDone
	if ctx.Err() != nil || waitErr == nil || command.ProcessState == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 {
		t.Fatalf("state=%v err=%v hits=%d out=%q", command.ProcessState, waitErr, hits.Load(), out.String())
	}
	if strings.Contains(out.String(), "Testes executados: 2") {
		t.Fatalf("suite continued: %q", out.String())
	}
}
