package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func TestFileFailuresPreservePreviousResults(t *testing.T) {
	for _, cause := range []error{fs.ErrNotExist, fs.ErrPermission, errPayloadDirectory, io.ErrUnexpectedEOF} {
		t.Run(cause.Error(), func(t *testing.T) {
			scenario, err := parser.Parse("safe.hurl", strings.NewReader("GET http://example.test/first\nHTTP 200\nPOST http://example.test/second\nAuthorization: credential-secret\nfile,private-path;\nHTTP 201\nGET http://example.test/last"))
			if err != nil {
				t.Fatal(err)
			}
			calls, reports := 0, 0
			runner := New(executorFunc(func(_ context.Context, _ Request) (Response, error) { calls++; return Response{Status: 200}, nil }), func(entry EntryResult) error {
				reports++
				if entry.Entry.ExternalBody != nil || entry.Entry.Body != nil || entry.Entry.Headers != nil || strings.Contains(fmt.Sprintf("%+v", entry), "credential-secret") {
					t.Fatalf("unsafe callback=%+v", entry)
				}
				return nil
			})
			runner.readFile = func(context.Context, string) ([]byte, error) {
				return []byte("partial payload-secret"), &os.PathError{Op: "read", Path: "private-path", Err: cause}
			}
			got := runner.Run(context.Background(), scenario)
			var failure *FileError
			var original *os.PathError
			if !errors.As(got.Err, &failure) || !errors.As(got.Err, &original) || !errors.Is(got.Err, cause) || calls != 1 || reports != 2 || len(got.Entries) != 2 || got.Entries[0].Outcome != Passed || got.Entries[1].Outcome != FileFailed || got.Summary != (Summary{Total: 3, Started: 1, Completed: 1, Validated: 1, NotStarted: 2}) || got.ErrorPosition != (parser.Position{File: "safe.hurl", Line: 5, Column: 1}) {
				t.Fatalf("calls=%d reports=%d got=%+v", calls, reports, got)
			}
			if strings.Contains(got.Err.Error(), "private-path") || strings.Contains(got.Err.Error(), "payload-secret") {
				t.Fatalf("unsafe error=%v", got.Err)
			}
		})
	}
}

type payloadReader struct {
	closed       bool
	failure      error
	closeFailure error
}

func (r *payloadReader) Read(p []byte) (int, error) {
	copy(p, "partial")
	return len("partial"), r.failure
}
func (r *payloadReader) Close() error { r.closed = true; return r.closeFailure }

func TestPayloadReadFailureClosesAndDiscardsPartialBytes(t *testing.T) {
	for _, closeFailure := range []error{nil, io.ErrClosedPipe} {
		reader := &payloadReader{failure: io.ErrUnexpectedEOF, closeFailure: closeFailure}
		body, err := readAndClosePayload(context.Background(), reader)
		if body != nil || !reader.closed || !errors.Is(err, io.ErrUnexpectedEOF) || closeFailure != nil && !errors.Is(err, closeFailure) {
			t.Fatalf("body=%q reader=%+v err=%v", body, reader, err)
		}
	}
}

func TestFileFailureReportErrorTakesPriority(t *testing.T) {
	scenario, err := parser.Parse("safe.hurl", strings.NewReader("POST http://example.test\nfile,missing;"))
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("write failed")
	runner := New(executorFunc(func(context.Context, Request) (Response, error) { t.Fatal("unexpected send"); return Response{}, nil }), func(EntryResult) error { return failure })
	runner.readFile = func(context.Context, string) ([]byte, error) { return nil, fs.ErrPermission }
	got := runner.Run(context.Background(), scenario)
	if !errors.Is(got.ReportErr, failure) || got.Summary.Started != 0 {
		t.Fatalf("got=%+v", got)
	}
}

func TestExternalBytes(t *testing.T) {
	for _, body := range [][]byte{nil, []byte(" \ttext\r\n\n"), {0, 255, 254, 1}, []byte(`{"x":"{{missing}}"}`)} {
		for _, absolute := range []bool{false, true} {
			dir := t.TempDir()
			path := filepath.Join(dir, " {{name}} payload ")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			reference := filepath.Base(path)
			if absolute {
				reference = path
			}
			scenario, err := parser.Parse("source.hurl", strings.NewReader("POST http://example.test\nfile,"+reference+";\nHTTP 201"))
			if err != nil {
				t.Fatal(err)
			}
			scenario.OriginDir = dir
			renderer, err := template.New([]string{"missing=replacement"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			got := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				if !bytes.Equal(req.Body, body) || len(req.Headers) != 0 {
					t.Fatalf("request=%+v want body=%q", req, body)
				}
				return Response{Status: 201}, nil
			}), renderer, nil).Run(context.Background(), scenario)
			if calls != 1 || got.Err != nil || got.Diverged {
				t.Fatalf("calls=%d got=%+v", calls, got)
			}
		}
	}
}
