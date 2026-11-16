package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func suiteDisplayPath(t *testing.T, path string) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	name, err := filepath.Rel(cwd, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(name)
}

func TestSuiteDiscoveryRealHTTP(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()
	dir := t.TempDir()
	a := configFile(t, dir, "01.hurl", "GET "+server.URL+"/a\nHTTP 200")
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	b := configFile(t, nested, "02.hurl", "GET "+server.URL+"/b\nHTTP 200")
	explicit := configFile(t, dir, "explicit", "GET "+server.URL+"/c\nHTTP 200")
	configFile(t, dir, "ignore.json", "ignored")
	for _, args := range [][]string{{"test", dir, explicit, b, a}, {"test", b, explicit, a, dir}} {
		mu.Lock()
		paths = nil
		mu.Unlock()
		code, out, stderr := run(t, args...)
		mu.Lock()
		got := append([]string(nil), paths...)
		mu.Unlock()
		if code != 0 || stderr != "" || !reflect.DeepEqual(got, []string{"/a", "/c", "/b"}) || strings.Count(out, "resumo ") != 3 {
			t.Fatalf("code=%d out=%q err=%q paths=%v", code, out, stderr, got)
		}
	}
	for _, bad := range []string{filepath.Join(dir, "absent"), t.TempDir()} {
		mu.Lock()
		paths = nil
		mu.Unlock()
		code, out, stderr := run(t, "test", dir, bad)
		mu.Lock()
		count := len(paths)
		mu.Unlock()
		if code != 2 || out != "" || stderr == "" || count != 0 {
			t.Fatalf("code=%d out=%q err=%q paths=%d", code, out, stderr, count)
		}
	}
}
