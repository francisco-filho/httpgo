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

func writeReadmeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadmeFirstRunAndRequestData(t *testing.T) {
	for _, tc := range []struct {
		name, marker, method, path, body string
		status, code                     int
		variables                        bool
	}{
		{name: "first success", marker: "conteúdo completo:", method: "GET", path: "/health", status: 200},
		{name: "first divergence", marker: "conteúdo completo:", method: "GET", path: "/health", status: 200, code: 1},
		{name: "JSON and headers", marker: "Salve `json.hurl`:", method: "POST", path: "/products", body: `{"name":"Pizza","price":4500}`, status: 201},
		{name: "typed variables", marker: "Salve `products.hurl`:", method: "POST", path: "/products", body: `{"name":"Pizza","price":4500,"active":true,"code":"001"}`, status: 201, variables: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				data, err := io.ReadAll(r.Body)
				if err != nil || r.Method != tc.method || r.URL.Path != tc.path || string(data) != tc.body {
					t.Errorf("request=%s %s body=%q want=%q err=%v", r.Method, r.URL.Path, data, tc.body, err)
				}
				if tc.method == "POST" && r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Content-Type=%q", r.Header.Get("Content-Type"))
				}
				if tc.name == "JSON and headers" && r.Header.Get("X-Client") != "guia-httpgo" {
					t.Errorf("X-Client=%q", r.Header.Get("X-Client"))
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			input := strings.ReplaceAll(readmeBlock(t, tc.marker, "hurl"), "http://localhost:8080", server.URL)
			if tc.code == 1 {
				input = strings.Replace(input, "HTTP 200", "HTTP 201", 1)
			}
			args := []string{"test", scenario(t, input), "--var", "base_url=" + server.URL}
			if tc.variables {
				args = append(args, "--var", "api.baseUrl="+server.URL, "--var", "product.name=Pizza", "--var", "product.price=4500", "--var", "product.active=true", "--var", `product.code="001"`)
			}
			code, out, stderr := run(t, args...)
			if code != tc.code || calls.Load() != 1 || !strings.Contains(out, "resumo") || (code == 0 && stderr != "") || (code == 1 && !strings.Contains(stderr, "status divergente")) {
				t.Fatalf("code=%d want=%d calls=%d out=%q stderr=%q", code, tc.code, calls.Load(), out, stderr)
			}
		})
	}
}

func TestReadmeExternalBodyAndMultipart(t *testing.T) {
	for _, multipart := range []bool{false, true} {
		t.Run(map[bool]string{false: "external JSON", true: "multipart"}[multipart], func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != "POST" {
					t.Errorf("method=%s", r.Method)
				}
				if multipart {
					if r.URL.Path != "/documents" {
						t.Errorf("path=%s", r.URL.Path)
					}
					if err := r.ParseMultipartForm(1 << 20); err != nil {
						t.Error(err)
						return
					}
					defer func() { _ = r.MultipartForm.RemoveAll() }()
					file, header, err := r.FormFile("document")
					if err != nil {
						t.Error(err)
						return
					}
					defer func() { _ = file.Close() }()
					data, err := io.ReadAll(file)
					if err != nil || string(data) != "Documento fictício\n" || header.Filename != "document.txt" || header.Header.Get("Content-Type") != "text/plain" || r.FormValue("description") != "Documento de teste" || r.FormValue("category") != "reports" {
						t.Errorf("multipart header=%v body=%q form=%v err=%v", header, data, r.MultipartForm.Value, err)
					}
				} else {
					data, err := io.ReadAll(r.Body)
					if err != nil || string(data) != "{\"name\":\"Pizza\",\"price\":4500}\n" || r.URL.Path != "/products" || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("external body=%q path=%s err=%v", data, r.URL.Path, err)
					}
				}
				w.WriteHeader(201)
			}))
			defer server.Close()
			dir := t.TempDir()
			marker, name, address := "`tests/arquivo.hurl` com:", "tests/arquivo.hurl", "http://localhost:8080"
			writeReadmeFile(t, dir, "payloads/product.json", "{\"name\":\"Pizza\",\"price\":4500}\n")
			if multipart {
				marker, name, address = "salve `tests/upload.hurl`:", "tests/upload.hurl", "http://localhost:8081"
				writeReadmeFile(t, dir, "fixtures/document.txt", "Documento fictício\n")
			}
			file := writeReadmeFile(t, dir, name, strings.ReplaceAll(readmeBlock(t, marker, "hurl"), address, server.URL))
			code, out, stderr := run(t, "test", file)
			if code != 0 || calls.Load() != 1 || stderr != "" {
				t.Fatalf("code=%d calls=%d out=%q stderr=%q", code, calls.Load(), out, stderr)
			}
		})
	}
}

func TestReadmeConfigCredentialsSuiteAndReports(t *testing.T) {
	const secret = "token-de-demonstracao"
	t.Setenv("OWNER_TOKEN", secret)
	var calls, authenticated atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != "GET" || (r.URL.Path != "/health" && r.URL.Path != "/products/1") {
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") == "Bearer "+secret {
			authenticated.Add(1)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	replaceAddress := func(s string) string {
		s = strings.ReplaceAll(s, "http://localhost:8080", server.URL)
		return strings.ReplaceAll(s, "http://127.0.0.1:8080", server.URL)
	}
	first := readmeBlock(t, "conteúdo completo:", "hurl")
	file := writeReadmeFile(t, dir, "exemplo.hurl", first)
	config := writeReadmeFile(t, dir, "guia.yaml", replaceAddress(readmeBlock(t, "salve `guia.yaml`:", "yaml")))
	writeReadmeFile(t, dir, "environments/local.yaml", replaceAddress(readmeBlock(t, "Crie `environments/local.yaml`:", "yaml")))
	credentials := writeReadmeFile(t, dir, "credenciais.yaml", replaceAddress(readmeBlock(t, "Salve `credenciais.yaml`:", "yaml")))
	credentialFile := writeReadmeFile(t, dir, "credenciais.hurl", readmeBlock(t, "Salve `credenciais.hurl`:", "hurl"))
	writeReadmeFile(t, dir, "suite/01-health.hurl", first)
	writeReadmeFile(t, dir, "suite/02-produto.hurl", readmeBlock(t, "Salve `suite/02-produto.hurl` com:", "hurl"))
	for _, extra := range [][]string{nil, {"--env", "local"}, {"--env", "local", "--var", "base_url=" + server.URL}} {
		args := append([]string{"test", file, "--config", config}, extra...)
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("args=%v code=%d out=%q stderr=%q", args, code, out, stderr)
		}
	}
	code, out, stderr := run(t, "test", credentialFile, "--config", credentials)
	if code != 0 || stderr != "" || authenticated.Load() != 1 || strings.Contains(out+stderr, secret) {
		t.Fatalf("credentials code=%d authenticated=%d out=%q stderr=%q", code, authenticated.Load(), out, stderr)
	}
	if err := os.Unsetenv("OWNER_TOKEN"); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	code, out, stderr = run(t, "test", credentialFile, "--config", credentials)
	if code != 2 || calls.Load() != before || !strings.Contains(stderr, "OWNER_TOKEN") {
		t.Fatalf("missing credential code=%d calls=%d out=%q stderr=%q", code, calls.Load()-before, out, stderr)
	}
	for _, format := range []string{"text", "junit", "html"} {
		t.Run(format, func(t *testing.T) {
			args := []string{"test", filepath.Join(dir, "suite"), "--var", "base_url=" + server.URL}
			destination := filepath.Join(dir, "target", "report.html")
			if format == "junit" {
				destination = filepath.Join(dir, "target", "test-results.xml")
			}
			if format != "text" {
				args = append(args, "--output", format, "--output-file", destination)
			}
			if format == "html" {
				args = append(args, "--fail-fast")
			}
			before := calls.Load()
			code, out, stderr := run(t, args...)
			if code != 0 || calls.Load()-before != 2 || (format == "text" && !strings.Contains(out, "resumo")) || (format != "text" && out != "") {
				t.Fatalf("code=%d calls=%d out=%q stderr=%q", code, calls.Load()-before, out, stderr)
			}
			if format == "junit" {
				doc, raw := junitXML(t, destination)
				if doc.Tests != 2 || doc.Failures != 0 || doc.Errors != 0 || doc.Skipped != 0 {
					t.Fatalf("unexpected report: %s", raw)
				}
			}
			if format == "html" {
				_, html := htmlArtifact(t, destination)
				if !strings.Contains(stderr, "gerado em") || !strings.Contains(html, "01-health.hurl") || !strings.Contains(html, "02-produto.hurl") {
					t.Fatalf("stderr=%q HTML=%s", stderr, html)
				}
			}
		})
	}
}
