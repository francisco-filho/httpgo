package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMultipartByBinary(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH"} {
		for _, text := range []string{"", "description: Documento\n"} {
			for _, payload := range []string{"", "\x00\xff{{missing}}"} {
				t.Run(fmt.Sprintf("%s/fields=%t/bytes=%d", method, text != "", len(payload)), func(t *testing.T) {
					var hits atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						reader, err := r.MultipartReader()
						if err != nil {
							t.Error(err)
							return
						}
						for {
							part, err := reader.NextPart()
							if err == io.EOF {
								break
							}
							if err != nil {
								t.Error(err)
								return
							}
							data, err := io.ReadAll(part)
							if err != nil {
								t.Error(err)
							}
							if part.FormName() == "document" && (part.FileName() != "olá mundo.pdf" || string(data) != payload || part.Header.Get("Content-Type") != "application/pdf") {
								t.Errorf("part=%v data=%q", part.Header, data)
							}
							if part.FormName() == "description" && string(data) != "Documento" {
								t.Errorf("text=%q", data)
							}
						}
						if r.Method != method {
							t.Errorf("method=%s", r.Method)
						}
						hits.Add(1)
						w.WriteHeader(201)
					}))
					defer server.Close()
					file := scenario(t, method+" "+server.URL+"\n[MultipartFormData]\n"+text+"document: file,olá mundo.pdf; application/pdf\nHTTP 201")
					if err := os.WriteFile(filepath.Join(filepath.Dir(file), "olá mundo.pdf"), []byte(payload), 0600); err != nil {
						t.Fatal(err)
					}
					code, out, stderr := run(t, "test", file)
					if code != 0 || stderr != "" || hits.Load() != 1 || !strings.Contains(out, "validadas=1") || strings.Contains(out, "{{missing}}") {
						t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
					}
				})
			}
		}
	}
}
