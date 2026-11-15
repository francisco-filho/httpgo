package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestExternalRenderedFailuresByBinary(t *testing.T) {
	for _, body := range []string{"", " \t", "not JSON", "\"\xff\"", "{} []", `{"x":"{{missing}}"}`, `{"x":{{unfinished}`, `{"x":"{{value}}"}`, `{"{{value}}":1}`} {
		t.Run(fmt.Sprintf("%q", body), func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
			defer server.Close()
			file := scenario(t, "GET "+server.URL+"/first\nHTTP 200\nPOST "+server.URL+"/second\nfile,payload.json;\nGET "+server.URL+"/last")
			if err := os.WriteFile(filepath.Join(filepath.Dir(file), "payload.json"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			code, out, stderr := run(t, "test", file, "--render-files", "--var", "value=[1,2]")
			if code != 2 || hits.Load() != 1 || !strings.Contains(stderr, `"payload.json":`) || !strings.Contains(stderr, "erro de renderização") || !strings.Contains(stderr, fmt.Sprintf("referência %q:4:1", file)) || !strings.Contains(out, "iniciadas=1 concluídas=1 validadas=1 sem_validação=0 não_iniciadas=2") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestExternalRenderedDemoByBinary(t *testing.T) {
	source := `{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}}}`
	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || r.Method != "POST" || r.URL.Path != "/products" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request=%v err=%v", r, err)
		}
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.WriteHeader(201)
	}))
	defer server.Close()
	file := scenario(t, "POST "+server.URL+"/products\nContent-Type: application/json\nfile,product.json;\nHTTP 201")
	if err := os.WriteFile(filepath.Join(filepath.Dir(file), "product.json"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"test", "--render-files", file, "--var", "product.name=Pizza", "--var", "product.price=45", "--var", "product.active=true"},
		{"test", file, "--var", "product.name=Suco", "--var", "product.price=12", "--var", "product.active=false", "--render-files"},
		{"test", file},
	} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
		if strings.Contains(out+stderr, source) {
			t.Fatalf("payload leaked: %q", out+stderr)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{`{"name":"Pizza","price":45,"active":true}`, `{"name":"Suco","price":12,"active":false}`, source}
	if len(bodies) != len(want) {
		t.Fatalf("bodies=%q", bodies)
	}
	for i := range want {
		if bodies[i] != want[i] {
			t.Fatalf("body[%d]=%q want=%q", i, bodies[i], want[i])
		}
	}
}
