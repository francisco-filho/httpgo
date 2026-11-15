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

func TestMultipartDocumentedScenario(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_multipart.hurl")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := os.ReadFile("../../testdata/payloads/document.pdf")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range []string{"scenarios", "payloads"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "payloads", "document.pdf"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "scenarios", "documents.hurl")
	if err := os.WriteFile(file, input, 0600); err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		names, values := []string{"description", "category", "document"}, []string{"Documento de teste", "reports", string(payload)}
		for i, name := range names {
			part, err := reader.NextPart()
			if err != nil {
				t.Error(err)
				return
			}
			data, err := io.ReadAll(part)
			if err != nil || part.FormName() != name || string(data) != values[i] {
				t.Errorf("part=%v data=%q err=%v", part.Header, data, err)
			}
			if i == 2 && (part.FileName() != "document.pdf" || part.Header.Get("Content-Type") != "application/pdf") {
				t.Errorf("metadata=%v", part.Header)
			}
		}
		if r.Header.Get("Authorization") != "Bearer ficticio" {
			t.Errorf("authorization=%q", r.Header.Get("Authorization"))
		}
		hits.Add(1)
		w.WriteHeader(201)
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", file, "--var", "base_url="+server.URL, "--var", "token=ficticio", "--secret", "token", "--var", "category=reports")
	if code != 0 || stderr != "" || hits.Load() != 1 || !strings.Contains(out, "validadas=1") || strings.Contains(out+stderr, "ficticio") {
		t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
	}
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr = run(t, args...)
		for _, text := range []string{"[MultipartFormData]", "document: file,../fixtures/document.pdf; application/pdf", "basename", "application/octet-stream", "413 esperado passa", "Sem --render-files"} {
			if code != 0 || stderr != "" || !strings.Contains(out, text) {
				t.Fatalf("help missing %q code=%d out=%q err=%q", text, code, out, stderr)
			}
		}
	}
}
