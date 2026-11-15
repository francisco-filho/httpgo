package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMultipartStatusAndDisconnectByBinary(t *testing.T) {
	for _, mode := range []string{"413", "mismatch", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			var hits, next atomic.Int32
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/next" {
					next.Add(1)
					w.WriteHeader(200)
					return
				}
				if mode == "disconnect" {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = conn.Close()
					return
				}
				w.Header().Set("Content-Length", "0")
				w.WriteHeader(413)
				w.(http.Flusher).Flush()
				<-release
			}))
			defer server.Close()
			defer close(release)
			expected := "413"
			if mode == "mismatch" {
				expected = "201"
			}
			file := scenario(t, "POST "+server.URL+"/upload\n[MultipartFormData]\nfile: file,large;\nHTTP "+expected+"\nGET "+server.URL+"/next\nHTTP 200")
			payload, err := os.Create(filepath.Join(filepath.Dir(file), "large"))
			if err != nil {
				t.Fatal(err)
			}
			truncateErr, closeErr := payload.Truncate(128<<20), payload.Close()
			if truncateErr != nil || closeErr != nil {
				t.Fatalf("truncate=%v close=%v", truncateErr, closeErr)
			}
			code, out, stderr := run(t, "test", file)
			wantCode, wantNext := 0, int32(1)
			if mode == "mismatch" {
				wantCode, wantNext = 1, 0
			}
			if mode == "disconnect" {
				wantCode, wantNext = 2, 0
			}
			if code != wantCode || next.Load() != wantNext || hits.Load() != 1+wantNext {
				t.Fatalf("code=%d hits=%d next=%d out=%q err=%q", code, hits.Load(), next.Load(), out, stderr)
			}
		})
	}
}

func TestMultipartTimeoutByBinary(t *testing.T) {
	var next atomic.Int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/next" {
			next.Add(1)
			return
		}
		var first [1]byte
		_, _ = r.Body.Read(first[:])
		<-release
	}))
	defer server.Close()
	defer close(release)
	file := scenario(t, "POST "+server.URL+"/upload\n[MultipartFormData]\nfile: file,large;\nHTTP 201\nGET "+server.URL+"/next")
	payload, err := os.Create(filepath.Join(filepath.Dir(file), "large"))
	if err != nil {
		t.Fatal(err)
	}
	truncateErr, closeErr := payload.Truncate(128<<20), payload.Close()
	if truncateErr != nil || closeErr != nil {
		t.Fatalf("truncate=%v close=%v", truncateErr, closeErr)
	}
	code, out, stderr := run(t, "test", file)
	if code != 2 || next.Load() != 0 || !strings.Contains(stderr, "timeout") || !strings.Contains(out, "iniciadas=1 concluídas=0") || !strings.Contains(out, "não_iniciadas=1") {
		t.Fatalf("code=%d next=%d out=%q err=%q", code, next.Load(), out, stderr)
	}
}

func TestMultipartMixedProcess(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index := hits.Add(1)
		if index == 2 {
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
			if err != nil || string(data) != "literal {{missing}}" {
				t.Errorf("data=%q err=%v", data, err)
			}
		} else {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
			}
			want := map[int32]string{1: "{}", 3: "literal {{missing}}", 4: `{"id":42}`}[index]
			if string(data) != want {
				t.Errorf("step=%d data=%q want=%q", index, data, want)
			}
		}
		w.WriteHeader(201)
		_, _ = io.WriteString(w, `{"id":42}`)
	}))
	defer server.Close()
	file := scenario(t, "POST "+server.URL+"\n{}\nHTTP 201\nPOST "+server.URL+"\n[MultipartFormData]\nfile: file,payload;\nHTTP 201\n[Captures]\nid: jsonpath \"$.id\"\n[Asserts]\njsonpath \"$.id\" == 42\nPOST "+server.URL+"\nfile,payload;\nHTTP 201\nPOST "+server.URL+"\n{\"id\":{{id}}}\nHTTP 201")
	if err := os.WriteFile(filepath.Join(filepath.Dir(file), "payload"), []byte("literal {{missing}}"), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := run(t, "test", file)
	if code != 0 || stderr != "" || hits.Load() != 4 || !strings.Contains(out, "validadas=4") || strings.Contains(out, "literal {{missing}}") {
		t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
	}
}
