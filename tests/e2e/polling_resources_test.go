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

func TestPollingReopensPayloadByBinary(t *testing.T) {
	for _, mode := range []string{"external", "multipart"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "upload")
			if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
				t.Fatal(err)
			}
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				var body []byte
				if mode == "multipart" {
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
					body, err = io.ReadAll(part)
					if err != nil {
						t.Error(err)
					}
					_ = part.Close()
					if _, err = reader.NextPart(); err != io.EOF {
						t.Errorf("unexpected part: %v", err)
					}
				} else {
					var err error
					body, err = io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
				}
				want := "first"
				if n == 2 {
					want = "second"
				}
				if string(body) != want {
					t.Errorf("body=%q want=%q", body, want)
				}
				if n == 1 {
					if err := os.WriteFile(path, []byte("second"), 0600); err != nil {
						t.Error(err)
					}
					w.WriteHeader(503)
				} else {
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			payload := "file,upload;\n"
			if mode == "multipart" {
				payload = "[MultipartFormData]\nfile: file,upload;\n"
			}
			file := configFile(t, dir, "poll.hurl", "POST "+server.URL+"\n[Options]\nretry: 1\nretry-interval: 0\n"+payload+"HTTP 200\n")
			code, out, stderr := run(t, "test", file)
			if code != 0 || hits.Load() != 2 || stderr != "" || !strings.Contains(out, "[tentativas=2]") || !strings.Contains(out, "iniciadas=1 concluídas=1 validadas=1") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestPollingIncompleteBodyStopsByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(503)
		_, _ = io.WriteString(w, "short")
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"\n[Options]\nretry: 3\nretry-interval: 0\nHTTP 200\nGET "+server.URL+"/never\n"))
	if code != 2 || hits.Load() != 1 || !strings.Contains(out, "tentativas_iniciadas=1 respostas_completas=0") || !strings.Contains(out, "não_iniciadas=1") || stderr == "" {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}
