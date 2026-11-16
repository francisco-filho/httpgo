package suite

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func fixture(t *testing.T, dir, name, text string) string {
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

func TestDiscoverNormalizedUniqueOrder(t *testing.T) {
	dir := t.TempDir()
	a := fixture(t, dir, "01.hurl", "GET http://example.test/a")
	b := fixture(t, dir, "nested/02.hurl", "GET http://example.test/b")
	c := fixture(t, dir, "explicit", "GET http://example.test/c")
	fixture(t, dir, "ignore.txt", "not a scenario")
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, a)
	if err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][]string{{dir, b, a, c, relative, dir + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".." + string(filepath.Separator) + "01.hurl"}, {c, a, filepath.Join(dir, "nested"), dir}} {
		files, err := New(nil).Discover(context.Background(), paths)
		if err != nil {
			t.Fatal(err)
		}
		want := []File{{a, displayName(cwd, a)}, {c, displayName(cwd, c)}, {b, displayName(cwd, b)}}
		// Lexicographic names, not argument order or traversal order.
		if !reflect.DeepEqual(files, want) {
			t.Fatalf("files=%v want=%v", files, want)
		}
	}
}

func TestDiscoverEquivalenceUsesFilesystem(t *testing.T) {
	dir := t.TempDir()
	lower := fixture(t, dir, "a.hurl", "GET http://example.test")
	upper := fixture(t, dir, "A.hurl", "GET http://example.test")
	link := filepath.Join(dir, "hard.hurl")
	if err := os.Link(lower, link); err != nil {
		t.Fatal(err)
	}
	lowInfo, err := os.Stat(lower)
	if err != nil {
		t.Fatal(err)
	}
	upInfo, err := os.Stat(upper)
	if err != nil {
		t.Fatal(err)
	}
	files, err := New(nil).Discover(context.Background(), []string{dir, upper, lower, link})
	if err != nil {
		t.Fatal(err)
	}
	want := 2
	if os.SameFile(lowInfo, upInfo) {
		want = 1
	}
	if len(files) != want {
		t.Fatalf("files=%v want count=%d", files, want)
	}
}

func TestDiscoverLinks(t *testing.T) {
	dir := t.TempDir()
	good := fixture(t, dir, "good.hurl", "GET http://example.test")
	for name, target := range map[string]string{"link.hurl": good, "loop": dir, "broken.hurl": filepath.Join(dir, "absent")} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	files, err := New(nil).Discover(context.Background(), []string{dir})
	if err != nil || len(files) != 1 || files[0].Path != good {
		t.Fatalf("files=%v err=%v", files, err)
	}
	for _, name := range []string{"link.hurl", "loop/good.hurl", "loop/../good.hurl", "broken.hurl"} {
		files, err := New(nil).Discover(context.Background(), []string{good, dir + string(filepath.Separator) + name})
		if err == nil || files != nil {
			t.Fatalf("path=%s files=%v err=%v", name, files, err)
		}
	}
}

func TestDiscoverErrorsDiscardWholePlan(t *testing.T) {
	dir := t.TempDir()
	good := fixture(t, dir, "good.hurl", "GET http://example.test")
	empty := t.TempDir()
	fixture(t, empty, "readme.txt", "ignored")
	for _, bad := range []string{filepath.Join(dir, "missing"), empty, good + string(filepath.Separator) + "child", good + string(filepath.Separator), good + string(filepath.Separator) + "."} {
		files, err := New(nil).Discover(context.Background(), []string{good, bad})
		if err == nil || files != nil {
			t.Fatalf("bad=%s files=%v err=%v", bad, files, err)
		}
	}
	for _, mode := range []string{"open", "close", "directory", "stat"} {
		t.Run(mode, func(t *testing.T) {
			s := New(nil)
			cause := errors.New("private-fs-cause")
			switch mode {
			case "open":
				s.open = func(string) (scenarioFile, error) { return nil, cause }
			case "close":
				s.open = func(string) (scenarioFile, error) {
					return &faultFile{Reader: strings.NewReader(""), closeErr: cause}, nil
				}
			case "directory":
				s.readDir = func(string) ([]os.DirEntry, error) { return nil, cause }
			case "stat":
				s.lstat = func(string) (fs.FileInfo, error) { return nil, cause }
			}
			files, err := s.Discover(context.Background(), []string{dir})
			if files != nil || !errors.Is(err, cause) || strings.Contains(err.Error(), cause.Error()) {
				t.Fatalf("files=%v err=%v", files, err)
			}
		})
	}
}

type faultFile struct {
	io.Reader
	info              fs.FileInfo
	statErr, closeErr error
	closed            bool
}

func (f *faultFile) Stat() (fs.FileInfo, error) { return f.info, f.statErr }
func (f *faultFile) Close() error               { f.closed = true; return f.closeErr }

type executor struct{ calls int }

func (e *executor) Execute(context.Context, runner.Request) (runner.Response, error) {
	e.calls++
	return runner.Response{Status: 200}, nil
}

type readFailure struct{ cause error }

func (r readFailure) Read([]byte) (int, error) { return 0, r.cause }

func TestLoadFailureNeverExecutesPartialScenario(t *testing.T) {
	path := fixture(t, t.TempDir(), "input.hurl", "GET http://example.test")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("private-load-cause")
	for _, mode := range []string{"open", "stat", "read", "close", "parse", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			e := &executor{}
			s := New(runner.New(e, nil))
			file := &faultFile{Reader: strings.NewReader("GET http://example.test"), info: info}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s.open = func(string) (scenarioFile, error) {
				if mode == "open" {
					return nil, cause
				}
				if mode == "cancel" {
					cancel()
				}
				return file, nil
			}
			switch mode {
			case "stat":
				file.statErr = cause
			case "read":
				file.Reader = io.MultiReader(file.Reader, readFailure{cause})
			case "close":
				file.closeErr = cause
			case "parse":
				file.Reader = strings.NewReader("GET http://example.test\nHEAD http://example.test")
			}
			result, err := s.RunFile(ctx, File{path, "input.hurl"}, runner.Options{})
			if err == nil || e.calls != 0 || len(result.Entries) != 0 || mode != "open" && !file.closed {
				t.Fatalf("result=%+v err=%v calls=%d closed=%v", result, err, e.calls, file.closed)
			}
			if mode != "parse" && mode != "cancel" && !errors.Is(err, cause) {
				t.Fatalf("cause lost: %v", err)
			}
			if strings.Contains(err.Error(), cause.Error()) {
				t.Fatalf("raw cause leaked: %v", err)
			}
		})
	}
}
