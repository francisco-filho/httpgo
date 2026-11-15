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

func TestExternalDemoAndHelpByBinary(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_external_body.hurl")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile("../../testdata/payloads/product.json")
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || !bytes.Equal(body, payload) || r.URL.Path != "/products" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("body=%q err=%v request=%v", body, err, r)
		}
		hits.Add(1)
		w.WriteHeader(201)
	}))
	defer server.Close()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "tests"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "payloads"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "payloads", "product.json"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "tests", "products.hurl")
	if err := os.WriteFile(file, []byte(strings.ReplaceAll(string(input), "http://localhost:8080", server.URL)), 0600); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := run(t, "test", file)
	if code != 0 || stderr != "" || hits.Load() != 1 || !strings.Contains(out, "validadas=1") {
		t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
	}
	code, out, stderr = run(t, "test", "--help")
	if code != 0 || stderr != "" || !strings.Contains(out, "file,../payloads/product.json;") || !strings.Contains(out, "Sem sandbox") {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
}

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

func TestExternalReadFailureByBinary(t *testing.T) {
	for _, mode := range []string{"missing", "directory", "permission"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
			defer server.Close()
			file := scenario(t, "GET "+server.URL+"/first\nHTTP 200\nPOST "+server.URL+"/second\nAuthorization: credential-secret\nfile,credential-secret;\nGET "+server.URL+"/last")
			payload := filepath.Join(filepath.Dir(file), "credential-secret")
			want := "arquivo inexistente"
			switch mode {
			case "directory":
				if err := os.Mkdir(payload, 0700); err != nil {
					t.Fatal(err)
				}
				want = "diretório"
			case "permission":
				if err := os.WriteFile(payload, []byte("payload-secret"), 0000); err != nil {
					t.Fatal(err)
				}
				want = "acesso negado"
			}
			code, out, stderr := run(t, "test", file)
			if code != 2 || hits.Load() != 1 || !strings.Contains(stderr, want) || !strings.Contains(stderr, fmt.Sprintf("%q:5:1:", file)) || !strings.Contains(out, "iniciadas=1 concluídas=1 validadas=1 sem_validação=0 não_iniciadas=2") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			for _, secret := range []string{"credential-secret", "payload-secret"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("unsafe output=%q", out+stderr)
				}
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
