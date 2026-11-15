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
	"sync/atomic"
	"testing"
	"time"
)

func TestExternalOriginAcrossProcessDirectories(t *testing.T) {
	root := t.TempDir()
	scenarioDir := filepath.Join(root, "project", "tests")
	if err := os.MkdirAll(scenarioDir, 0700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(root, "outside payload")
	const body = "literal {{missing}}\r\n"
	if err := os.WriteFile(payload, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil || string(data) != body {
			t.Errorf("body=%q err=%v", data, err)
		}
		hits.Add(1)
		w.WriteHeader(201)
	}))
	defer server.Close()
	file := filepath.Join(scenarioDir, "source.hurl")
	for _, reference := range []string{filepath.Join("..", "..", "outside payload"), payload} {
		if err := os.WriteFile(file, []byte("POST "+server.URL+"\nfile,"+reference+";\nHTTP 201"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, cwd := range []string{root, t.TempDir()} {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			command := exec.CommandContext(ctx, binary(t), "test", file)
			command.Dir = cwd
			var out, stderr bytes.Buffer
			command.Stdout, command.Stderr = &out, &stderr
			err := command.Run()
			cancel()
			if err != nil || stderr.Len() != 0 || !strings.Contains(out.String(), "validadas=1") {
				t.Fatalf("cwd=%q reference=%q err=%v out=%q stderr=%q", cwd, reference, err, out.String(), stderr.String())
			}
		}
	}
	if hits.Load() != 4 {
		t.Fatalf("hits=%d", hits.Load())
	}
}

func TestExternalMixedSequenceByBinary(t *testing.T) {
	for _, mode := range []string{"success", "mismatch", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				index := hits.Add(1)
				want := []string{`{"static":true}`, "payload-secret {{missing}}", `{"value":42}`}[index-1]
				if string(data) != want {
					t.Errorf("body=%q want=%q", data, want)
				}
				if index == 2 && mode == "mismatch" {
					w.WriteHeader(503)
					return
				}
				if index == 2 && mode == "incomplete" {
					w.Header().Set("Content-Length", "100")
					_, _ = io.WriteString(w, "short")
					return
				}
				w.WriteHeader(201)
			}))
			defer server.Close()
			text := "POST " + server.URL + "\n{\"static\":true}\nHTTP 201\nPOST " + server.URL + "\nAuthorization: credential-secret\nfile,external;\nHTTP 201\nPOST " + server.URL + "\n{\"value\":{{value}}}\nHTTP 201"
			file := scenario(t, text)
			if err := os.WriteFile(filepath.Join(filepath.Dir(file), "external"), []byte("payload-secret {{missing}}"), 0600); err != nil {
				t.Fatal(err)
			}
			code, out, stderr := run(t, "test", file, "--var", "value=42")
			wantCode, wantHits := 0, int32(3)
			if mode == "mismatch" {
				wantCode, wantHits = 1, 2
			}
			if mode == "incomplete" {
				wantCode, wantHits = 2, 2
			}
			if code != wantCode || hits.Load() != wantHits || !strings.Contains(out, "iniciadas="+fmt.Sprint(wantHits)) {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			if mode != "success" && !strings.Contains(out, "não_iniciadas=1") {
				t.Fatalf("out=%q", out)
			}
			for _, secret := range []string{"credential-secret", "payload-secret"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("unsafe output=%q", out+stderr)
				}
			}
		})
	}
}

func TestExternalRedirectsByBinary(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var hits atomic.Int32
			body := []byte{0, 255, '{', '{', 'x', '}', '}'}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				wantMethod, wantBody := "POST", body
				if r.URL.Path == "/final" && status <= 303 {
					wantMethod, wantBody = "GET", nil
				}
				if err != nil || r.Method != wantMethod || !bytes.Equal(data, wantBody) {
					t.Errorf("method=%s body=%q err=%v", r.Method, data, err)
				}
				hits.Add(1)
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(status)
					return
				}
				w.WriteHeader(201)
			}))
			defer server.Close()
			file := scenario(t, "POST "+server.URL+"/start\nfile,external;\nHTTP 201")
			if err := os.WriteFile(filepath.Join(filepath.Dir(file), "external"), body, 0600); err != nil {
				t.Fatal(err)
			}
			code, out, stderr := run(t, "test", file)
			if code != 0 || stderr != "" || hits.Load() != 2 || !strings.Contains(out, "iniciadas=1 concluídas=1 validadas=1") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}
