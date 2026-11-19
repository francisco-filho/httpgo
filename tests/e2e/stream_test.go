package e2e

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestResultsStreamWhileNextRequestIsPending(t *testing.T) {
	for _, tc := range []struct {
		name               string
		code               int
		outcome, completed string
	}{
		{"success", 0, "status validado", "concluídas=2"},
		{"mismatch", 1, "status divergente", "concluídas=2"},
		{"incomplete body", 2, "erro de transporte", "concluídas=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			secondStarted, releaseSecond := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/first" {
					w.Header().Set("X-Ready", "yes")
					w.WriteHeader(200)
					_, _ = io.WriteString(w, `{"token":"stream-token-ficticio"}`)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil || r.Method != "POST" || string(body) != `{"typed":[true,null,9007199254740993123456789]}` || r.Header.Get("Authorization") != "Bearer stream-token-ficticio" {
					t.Errorf("method=%s body=%s err=%v", r.Method, body, err)
				}
				close(secondStarted)
				select {
				case <-releaseSecond:
				case <-r.Context().Done():
					return
				}
				switch tc.name {
				case "mismatch":
					w.WriteHeader(503)
				case "incomplete body":
					w.Header().Set("Content-Length", "100")
					w.WriteHeader(200)
					_, _ = io.WriteString(w, "short")
				default:
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			address := strings.Replace(server.URL, "http://", "http://username-secret:password-secret@", 1)
			file := scenario(t, "GET {{first}}/first?token=query-secret#fragment-secret\nHTTP 200\n[Asserts]\njsonpath \"$.token\" == \"stream-token-ficticio\"\n[Captures]\ntoken: jsonpath \"$.token\"\nPOST {{base}}/second\nAuthorization: Bearer {{token}}\n{\"typed\":{{payload}}}\nHTTP 404")
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			reader, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = reader.Close(); _ = writer.Close() }()
			command := exec.CommandContext(ctx, binary(t), "test", file, "--var", "first="+address, "--var", "base="+server.URL, "--var", "payload=[true,null,9007199254740993123456789]", "--secret", "token")
			var stderr bytes.Buffer
			command.Stdout, command.Stderr = writer, &stderr
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			_ = writer.Close()
			lines, readDone, done := make(chan string, 4), make(chan struct{}), make(chan struct{})
			var readErr, waitErr error
			go func() {
				scanner := bufio.NewScanner(reader)
				for scanner.Scan() {
					lines <- scanner.Text()
				}
				readErr = scanner.Err()
				close(lines)
				close(readDone)
			}()
			go func() { waitErr = command.Wait(); close(done) }()
			defer func() { _ = command.Process.Kill(); <-done; <-readDone }()
			select {
			case <-secondStarted:
			case <-done:
				t.Fatal("process exited before the second request")
			case <-ctx.Done():
				t.Fatal("second request did not start")
			}
			var first string
			select {
			case first = <-lines:
				if !strings.HasPrefix(first, "GET ") || !strings.Contains(first, "/first?[oculta]: status validado") {
					t.Fatalf("first result = %q", first)
				}
			case <-ctx.Done():
				t.Fatal("first result was not visible while the second request was pending")
			}
			assertNoSecrets(t, first)
			select {
			case line := <-lines:
				t.Fatalf("output before the pending request finished: %q", line)
			default:
			}
			close(releaseSecond)
			// Drain concurrently with the child: the shared summary is multiline
			// and must not depend on a fixed channel buffer to let Wait complete.
			var remaining []string
			for line := range lines {
				remaining = append(remaining, line)
			}
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("process did not finish after releasing the second response")
			}
			select {
			case <-readDone:
			case <-ctx.Done():
				t.Fatal("stdout did not close")
			}
			if readErr != nil || command.ProcessState.ExitCode() != tc.code || (waitErr == nil) != (tc.code == 0) {
				t.Fatalf("exit=%v,wait=%v,read=%v,stderr=%q", command.ProcessState, waitErr, readErr, stderr.String())
			}
			if len(remaining) != 11 || remaining[2] != "Cenários executados: 1" || remaining[6] != "Testes executados: 2" || !strings.HasPrefix(remaining[10], "Tempo:") || !strings.Contains(remaining[0], "/second: "+tc.outcome) || !strings.HasPrefix(remaining[1], "resumo ") || !strings.Contains(remaining[1], tc.completed) {
				t.Fatalf("remaining output = %q", remaining)
			}
			prefix := "POST "
			if tc.code != 0 {
				prefix = fmt.Sprintf("%q:7:1: POST ", suiteDisplayPath(t, file))
			}
			if !strings.HasPrefix(remaining[0], prefix) {
				t.Fatalf("second result = %q; want prefix %q", remaining[0], prefix)
			}
			if tc.code == 0 && stderr.Len() != 0 || tc.code != 0 && stderr.Len() == 0 {
				t.Fatalf("stderr = %q", stderr.String())
			}
			assertNoSecrets(t, first+strings.Join(remaining, "\n")+stderr.String())
			if strings.Contains(first+strings.Join(remaining, "\n")+stderr.String(), "stream-token-ficticio") {
				t.Fatal("capture leaked")
			}
		})
	}
}
