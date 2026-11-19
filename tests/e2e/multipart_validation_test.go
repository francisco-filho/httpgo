package e2e

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestMultipartOriginAndMIMEByProcess(t *testing.T) {
	root := t.TempDir()
	scenarioDir := filepath.Join(root, "project", "tests")
	if err := os.MkdirAll(scenarioDir, 0700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(root, "outside {{missing}}.PDF")
	if err := os.WriteFile(payload, []byte("\x00\xff{{unbound}}"), 0600); err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		data, err := io.ReadAll(part)
		if err != nil || part.FileName() != "outside {{missing}}.PDF" || part.Header.Get("Content-Type") != "application/pdf" || string(data) != "\x00\xff{{unbound}}" {
			t.Errorf("part=%v data=%q err=%v", part.Header, data, err)
		}
		hits.Add(1)
		w.WriteHeader(201)
	}))
	defer server.Close()
	file := filepath.Join(scenarioDir, "upload.hurl")
	for _, reference := range []string{filepath.Join("..", "..", "outside {{missing}}.PDF"), payload} {
		if err := os.WriteFile(file, []byte("POST "+server.URL+"\n[MultipartFormData]\ndocument: file,"+reference+";\nHTTP 201"), 0600); err != nil {
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
				t.Fatalf("cwd=%q err=%v out=%q stderr=%q", cwd, err, out.String(), stderr.String())
			}
		}
	}
	if hits.Load() != 4 {
		t.Fatalf("hits=%d", hits.Load())
	}
}

func TestMultipartInvalidInputsByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	for _, body := range []string{
		"[MultipartFormData]\na: text", "[MultipartFormData]\na: file,a;\nb: file,b;", "[MultipartFormData]\na: file,a;\na: text", "{}\n[MultipartFormData]\na: file,a;", "[MultipartFormData]\na: file,a;\n{\"other\":true}", "[MultipartFormData]\n{{name}}: file,a;", "Content-Type: application/json\n[MultipartFormData]\na: file,a;", "Content-Type: multipart/form-data; boundary=x\n[MultipartFormData]\na: file,a;", "[MultipartFormData]\na: file,a; invalid-mime",
	} {
		file := scenario(t, "GET "+server.URL+"/first\nPOST "+server.URL+"\n"+body)
		code, out, stderr := run(t, "test", file)
		if code != 2 || !prevalidationSummary(out) || stderr == "" || hits.Load() != 0 {
			t.Fatalf("body=%q code=%d hits=%d out=%q err=%q", body, code, hits.Load(), out, stderr)
		}
	}
}

func TestMultipartProtectionAndPreparationByBinary(t *testing.T) {
	for _, mode := range []string{"success", "assertion", "missing", "directory", "permission", "variable", "array", "object", "type", "boundary"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/first" {
					w.WriteHeader(200)
					return
				}
				if r.URL.Path != "/upload" {
					t.Errorf("unexpected path=%s", r.URL.Path)
					return
				}
				reader, err := r.MultipartReader()
				if err != nil {
					t.Error(err)
					return
				}
				part, err := reader.NextPart()
				if err != nil {
					t.Error(err)
					return
				}
				data, err := io.ReadAll(part)
				if err != nil {
					t.Error(err)
					return
				}
				if string(data) != "prefix-token-fictitious-suffix" {
					t.Errorf("text=%q", data)
				}
				_, _ = io.Copy(io.Discard, r.Body)
				w.WriteHeader(201)
				_ = json.NewEncoder(w).Encode(map[string]string{"echo": string(data)})
			}))
			defer server.Close()
			text := "GET " + server.URL + "/first\nHTTP 200\nPOST " + server.URL + "/upload\nContent-Type: {{kind}}\n[MultipartFormData]\ndescription: prefix-{{secret}}-suffix\nextra: {{value}}\ndocument: file,private;\nHTTP 201"
			if mode == "success" {
				text += "\n[Asserts]\njsonpath \"$.echo\" == \"prefix-token-fictitious-suffix\""
			}
			if mode == "assertion" {
				text += "\n[Asserts]\njsonpath \"$.echo\" == \"wrong\""
			}
			text += "\nGET " + server.URL + "/last"
			if mode == "success" {
				text = strings.TrimSuffix(text, "\nGET "+server.URL+"/last")
			}
			file := scenario(t, text)
			path := filepath.Join(filepath.Dir(file), "private")
			kind, value := "multipart/form-data", "text"
			if mode == "type" {
				kind = "application/json"
			}
			if mode == "boundary" {
				kind = "multipart/form-data; boundary=x"
			}
			if mode == "array" {
				value = "[]"
			}
			if mode == "object" {
				value = "{}"
			}
			if mode == "directory" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else if mode != "missing" {
				perm := os.FileMode(0600)
				if mode == "permission" {
					perm = 0000
				}
				if err := os.WriteFile(path, []byte("private-file-bytes"), perm); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"test", file, "--var", "secret=token-fictitious", "--secret", "secret", "--var", "kind=" + kind}
			if mode != "variable" {
				args = append(args, "--var", "value="+value)
			}
			code, out, stderr := run(t, args...)
			wantCode, wantHits := 2, int32(1)
			if mode == "success" {
				wantCode, wantHits = 0, 2
			}
			if mode == "assertion" {
				wantCode, wantHits = 1, 2
			}
			if code != wantCode || hits.Load() != wantHits {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			if mode == "permission" && !strings.Contains(stderr, "acesso negado") {
				t.Fatalf("stderr=%q", stderr)
			}
			for _, secret := range []string{"token-fictitious", "prefix-", "private-file-bytes"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("unsafe output=%q", out+stderr)
				}
			}
		})
	}
}
