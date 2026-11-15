package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

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
