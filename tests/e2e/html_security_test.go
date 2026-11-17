package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestHTMLRealPollingCapturesAndIsolationMatchOtherFormats(t *testing.T) {
	var mu sync.Mutex
	var requests []string
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Header.Get("Cookie") != "" {
			t.Error("cookie shared implicitly")
		}
		switch r.URL.Path {
		case "/login":
			w.Header().Set("Set-Cookie", "fiction-html-cookie")
			_, _ = fmt.Fprint(w, `{"id":"selected"}`)
		case "/poll":
			polls++
			if polls == 1 {
				w.WriteHeader(503)
			}
		case "/items/selected":
			if r.Header.Get("X-ID") != "selected" {
				t.Error("capture not sent intact")
			}
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	configFile(t, dir, "01.hurl", "GET "+server.URL+"/login\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"\nGET "+server.URL+"/poll\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200\nGET "+server.URL+"/items/{{id}}\nX-ID: {{id}}\nHTTP 200")
	configFile(t, dir, "02.hurl", "GET "+server.URL+"/items/{{id}}\nHTTP 200")
	configFile(t, dir, "03.hurl", "GET "+server.URL+"/last")
	want := []string{"GET /login", "GET /poll", "GET /poll", "GET /items/selected", "GET /last"}
	for _, format := range []string{"text", "junit", "html"} {
		mu.Lock()
		requests = nil
		polls = 0
		mu.Unlock()
		args := []string{"test", dir}
		base := filepath.Join(t.TempDir(), "report."+format)
		if format != "text" {
			args = append(args, "--output="+format, "--output-file", base)
		}
		code, out, stderr := run(t, args...)
		mu.Lock()
		got := append([]string(nil), requests...)
		mu.Unlock()
		if code != 2 || !reflect.DeepEqual(got, want) || !strings.Contains(stderr, "renderização") {
			t.Fatalf("format=%s code=%d requests=%v out=%q err=%q", format, code, got, out, stderr)
		}
		if format == "html" {
			_, html := htmlArtifact(t, base)
			for _, text := range []string{"Tentativas iniciadas: 5", "Iniciadas: 4", "sem validação de status", "tentativas=2"} {
				if !strings.Contains(html, text) {
					t.Fatalf("HTML missing %q: %s", text, html)
				}
			}
		}
	}
}

func TestHTMLRealSecretsAreNotInSerializedDocument(t *testing.T) {
	const token = "fiction-html-env-<&\"secret"
	const captured = "fiction-html-captured-secret"
	t.Setenv("HTTPGO_HTML_ENV", token)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("environment credentials changed")
		}
		if r.URL.Path == "/login" {
			_ = json.NewEncoder(w).Encode(map[string]string{"token": captured, "private": "FULL_BODY_SENTINEL"})
			return
		}
		if r.Header.Get("X-Derived") != "prefix-"+captured {
			t.Error("capture derivation changed")
		}
		w.Header().Set("X-Private", "FULL_HEADER_SENTINEL")
		_ = json.NewEncoder(w).Encode(map[string]string{"echo": captured})
	}))
	defer server.Close()
	root := t.TempDir()
	base := configFile(t, root, "base.yaml", "version: 1\nvariables:\n  token: ${HTTPGO_HTML_ENV}\nsecrets: [captured]")
	file := configFile(t, root, "ação & \"<script>\".hurl", "GET "+server.URL+"/login\nAuthorization: Bearer {{token}}\nHTTP 200\n[Captures]\ncaptured: jsonpath \"$.token\"\nGET "+server.URL+"/echo\nAuthorization: Bearer {{token}}\nX-Derived: prefix-{{captured}}\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"different\"")
	dest := filepath.Join(root, "report.html")
	code, out, stderr := run(t, "test", file, "--config", base, "--output=html", "--output-file", dest)
	_, html := htmlArtifact(t, dest)
	if code != 1 || out != "" || !strings.Contains(html, "[oculto]") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatalf("code=%d out=%q err=%q HTML=%s", code, out, stderr, html)
	}
	if strings.Contains(html, "<script>") {
		t.Fatal("executable markup in serialized HTML")
	}
	for _, value := range []string{"fiction-html-env-", captured, "FULL_BODY_SENTINEL", "FULL_HEADER_SENTINEL"} {
		if strings.Contains(out+stderr+html, value) {
			t.Fatalf("leak of %q in report or terminal", value)
		}
	}
}
