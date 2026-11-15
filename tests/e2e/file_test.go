package e2e

import (
	"bytes"
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

func TestExternalBodiesByBinary(t *testing.T) {
	for i, body := range [][]byte{nil, []byte(" \ttext\r\n\n"), {0, 255, 254, 1}, []byte(`{"x":"{{missing}}"}`)} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil || !bytes.Equal(data, body) || r.Method != "POST" || r.Header.Get("Content-Type") != "" {
					t.Errorf("body=%q err=%v headers=%v", data, err, r.Header)
				}
				hits.Add(1)
				w.WriteHeader(201)
			}))
			defer server.Close()
			file := scenario(t, "POST "+server.URL+"\nfile, {{missing}} payload ;\nHTTP 201")
			if err := os.WriteFile(filepath.Join(filepath.Dir(file), " {{missing}} payload "), body, 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{nil, {"--var", "missing=replacement"}} {
				code, out, stderr := run(t, append([]string{"test", file}, args...)...)
				if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
					t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
				}
			}
			if hits.Load() != 2 {
				t.Fatalf("hits=%d", hits.Load())
			}
		})
	}
}

func TestMalformedExternalPreventsAllNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	for _, suffix := range []string{"file,;", "file,a", "file,a,b;", "file,a;b;", "file,a;\nfile,b;", "{}\nfile,a;", "file,a;\n{}", "file,a;\nX-Test: value"} {
		file := scenario(t, "GET "+server.URL+"/first\nHTTP 200\nPOST "+server.URL+"\n"+suffix)
		code, out, stderr := run(t, "test", file)
		if code != 2 || out != "" || !strings.Contains(stderr, fmt.Sprintf("%q:", file)) || hits.Load() != 0 {
			t.Fatalf("suffix=%q code=%d out=%q err=%q hits=%d", suffix, code, out, stderr, hits.Load())
		}
	}
}
