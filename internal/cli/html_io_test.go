package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/suite"
)

func htmlCommand(t *testing.T, destination string) (*Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	command := NewRootCommand(&fakeExecutor{statuses: []int{503}}, BuildInfo{})
	command.reportNow = func() time.Time {
		return time.Date(2026, 11, 17, 16, 30, 45, 123999999, time.FixedZone("west", -3*60*60))
	}
	var out, stderr bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&stderr)
	command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP 200"), "--output=html", "--output-file", destination})
	return command, &out, &stderr
}

func TestHTMLTimestampCollisionAndPermissions(t *testing.T) {
	for _, name := range []string{"report.html", "report.with.dots.html", "report"} {
		t.Run(name, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "new", "nested", name)
			ext := filepath.Ext(base)
			prefix := strings.TrimSuffix(base, ext) + "-20261117T193045123Z"
			if err := os.MkdirAll(filepath.Dir(base), 0700); err != nil {
				t.Fatal(err)
			}
			previous := []string{base, prefix + ext, prefix + "-1" + ext, prefix + "-3" + ext}
			for _, path := range previous {
				if err := os.WriteFile(path, []byte("unchanged:"+path), 0600); err != nil {
					t.Fatal(err)
				}
			}
			command, out, stderr := htmlCommand(t, base)
			clockCalls := 0
			clock := command.reportNow
			command.reportNow = func() time.Time { clockCalls++; return clock() }
			code := command.ExecuteContext(context.Background())
			path := prefix + "-2" + ext
			if code != 1 || out.Len() != 0 || clockCalls != 1 || !strings.Contains(stderr.String(), fmt.Sprintf("gerado em %q", path)) {
				t.Fatalf("code=%d out=%q err=%q clocks=%d", code, out, stderr, clockCalls)
			}
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Contains(data, []byte("<!doctype html>")) {
				t.Fatalf("HTML=%q err=%v", data, err)
			}
			for _, path := range previous {
				data, err := os.ReadFile(path)
				if err != nil || string(data) != "unchanged:"+path {
					t.Fatalf("previous %s changed: %q %v", path, data, err)
				}
			}
			// A repeated invocation at the same instant must use the next gap.
			command.executor = &fakeExecutor{statuses: []int{200}}
			stderr.Reset()
			if code := command.ExecuteContext(context.Background()); code != 0 || !strings.Contains(stderr.String(), fmt.Sprintf("gerado em %q", prefix+"-4"+ext)) {
				t.Fatalf("second code=%d err=%q", code, stderr)
			}
		})
	}
}

func TestHTMLCreatesPrivateDestination(t *testing.T) {
	base := filepath.Join(t.TempDir(), "new", "nested", "report.html")
	command, _, _ := htmlCommand(t, base)
	if code := command.ExecuteContext(context.Background()); code != 1 {
		t.Fatal(code)
	}
	path, _ := readHTML(t, base)
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{{path, 0600}, {filepath.Dir(base), 0700}, {filepath.Dir(filepath.Dir(base)), 0700}} {
		info, err := os.Stat(item.path)
		if err != nil || info.Mode().Perm() & ^item.mode != 0 {
			t.Fatalf("path=%s info=%v err=%v", item.path, info, err)
		}
	}
}

func TestHTMLDestinationFailuresAndResourceClosure(t *testing.T) {
	for _, mode := range []string{"directory", "trailing separator", "parent file", "invalid", "open", "permission", "write", "short", "close", "write and close", "confirmation"} {
		t.Run(mode, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), "out.html")
			switch mode {
			case "directory":
				base = t.TempDir()
			case "trailing separator":
				base += string(filepath.Separator)
			case "parent file":
				base = filepath.Join(writeScenario(t, "GET http://example.test"), "out.html")
			case "invalid":
				base += "\x00"
			}
			command, out, stderr := htmlCommand(t, base)
			writer := &junitWriter{}
			opens := 0
			switch mode {
			case "open", "permission", "write", "short", "close", "write and close":
				command.openReport = func(string) (io.WriteCloser, error) {
					opens++
					if mode == "open" {
						return nil, errors.New("private open cause")
					}
					if mode == "permission" {
						return nil, os.ErrPermission
					}
					return writer, nil
				}
				writer.fail = mode == "write" || mode == "write and close"
				writer.short = mode == "short"
				if mode == "close" || mode == "write and close" {
					writer.closeErr = errors.New("private close cause")
				}
			case "confirmation":
				command.executor = &fakeExecutor{statuses: []int{200}}
				command.SetErr(&suiteWriter{short: true})
			}
			code := command.ExecuteContext(context.Background())
			if code != 2 || out.Len() != 0 || strings.Contains(stderr.String(), "gerado em") || strings.Contains(stderr.String(), "private") {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
			if mode != "confirmation" && !strings.Contains(stderr.String(), "não foi possível gravar relatório HTML") {
				t.Fatal(stderr.String())
			}
			if mode == "permission" && opens != 1 {
				t.Fatalf("permission retried %d times", opens)
			}
			if (writer.fail || writer.short || writer.closeErr != nil) && !writer.closed {
				t.Fatal("writer not closed")
			}
			if mode == "confirmation" {
				readHTML(t, base)
			}
		})
	}
}

func TestHTMLExclusiveOpenCollisionAndJoinedCauses(t *testing.T) {
	command, _, _ := htmlCommand(t, filepath.Join(t.TempDir(), "report.html"))
	calls := 0
	var paths []string
	writeCause, closeCause := errors.New("write"), errors.New("close")
	writer := &htmlFailWriter{writeErr: writeCause, closeErr: closeCause}
	command.openReport = func(path string) (io.WriteCloser, error) {
		paths = append(paths, path)
		calls++
		if calls == 1 {
			return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrExist}
		}
		return writer, nil
	}
	command.protection = &executionProtection{initial: func(s string) string { return s }}
	path, err := command.writeHTML(filepath.Join(t.TempDir(), "report.html"), suite.Result{}, nil)
	if path != "" || !errors.Is(err, writeCause) || !errors.Is(err, closeCause) || !writer.closed || calls != 2 || !strings.HasSuffix(paths[1], "-20261117T193045123Z-1.html") {
		t.Fatalf("path=%s err=%v closed=%t paths=%v", path, err, writer.closed, paths)
	}
}

type htmlFailWriter struct {
	writeErr, closeErr error
	closed             bool
}

func (w *htmlFailWriter) Write([]byte) (int, error) { return 0, w.writeErr }
func (w *htmlFailWriter) Close() error              { w.closed = true; return w.closeErr }
