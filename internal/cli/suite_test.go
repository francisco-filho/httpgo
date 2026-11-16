package cli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func suiteFile(t *testing.T, dir, name, text string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func shownPath(t *testing.T, path string) string {
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

func TestSuiteDiscoveryAndConfiguration(t *testing.T) {
	dir := t.TempDir()
	a := suiteFile(t, dir, "01.hurl", "GET {{base}}/a\nHTTP 200")
	b := suiteFile(t, dir, "nested/02.hurl", "GET {{base}}/b\nHTTP 200")
	c := suiteFile(t, dir, "explicit", "GET {{base}}/c\nHTTP 200")
	suiteFile(t, dir, "payload.txt", "ignored")
	for _, paths := range [][]string{{dir, b, a, c}, {c, a, dir, b}} {
		fake := &fakeExecutor{statuses: []int{200, 200, 200}}
		args := append([]string{"test", "--var", "base=http://example.test"}, paths...)
		code, out, stderr := execute(t, context.Background(), fake, args...)
		if code != 0 || stderr != "" || !reflect.DeepEqual(fake.calls, []string{"http://example.test/a", "http://example.test/c", "http://example.test/b"}) || strings.Count(out, "resumo ") != 3 {
			t.Fatalf("code=%d out=%q err=%q calls=%v", code, out, stderr, fake.calls)
		}
	}
	for _, mode := range []string{"missing", "empty", "link", "config", "environment", "reference"} {
		t.Run(mode, func(t *testing.T) {
			fake := &fakeExecutor{}
			args := []string{"test", dir}
			switch mode {
			case "missing":
				args = append(args, filepath.Join(dir, "missing.hurl"))
			case "empty":
				args = append(args, t.TempDir())
			case "link":
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(dir, link); err != nil {
					t.Fatal(err)
				}
				args = append(args, link)
			case "config":
				args = append(args, "--config", suiteFile(t, t.TempDir(), "bad.yaml", "version: 999"))
			case "environment":
				args = append(args, "--env", "../bad")
			case "reference":
				t.Setenv("HTTPGO_SUITE_PRESENT", "fictitious-known-secret")
				t.Setenv("HTTPGO_SUITE_ABSENT", "")
				if err := os.Unsetenv("HTTPGO_SUITE_ABSENT"); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--config", suiteFile(t, t.TempDir(), "base.yaml", "version: 1\nvariables: {known: '${HTTPGO_SUITE_PRESENT}', absent: '${HTTPGO_SUITE_ABSENT}'}"))
			}
			code, out, stderr := execute(t, context.Background(), fake, args...)
			if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 || strings.Contains(stderr, "fictitious-known-secret") {
				t.Fatalf("code=%d out=%q err=%q calls=%v", code, out, stderr, fake.calls)
			}
		})
	}
}
