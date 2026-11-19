package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
)

type faultyUpload struct {
	*observedUpload
	statErr, closeErr error
}

func (f *faultyUpload) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.File.Stat()
}
func (f *faultyUpload) Close() error { return errors.Join(f.observedUpload.Close(), f.closeErr) }

func TestMultipartResourceFailurePaths(t *testing.T) {
	for _, mode := range []string{"stat", "close", "execute", "report", "cancel-before", "cancel-open", "cancel-execute"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "upload")
			if err := os.WriteFile(path, []byte("payload-private"), 0600); err != nil {
				t.Fatal(err)
			}
			scenario, err := parser.Parse("safe.hurl", strings.NewReader("POST http://localhost\n[MultipartFormData]\na: file,upload;\nHTTP 201\nGET http://localhost/next"))
			if err != nil {
				t.Fatal(err)
			}
			scenario.OriginDir = dir
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var source *faultyUpload
			calls, reports := 0, 0
			failure := errors.New("private error must not reach diagnostics")
			r := New(executorFunc(func(ctx context.Context, req Request) (Response, error) {
				calls++
				if source.closed {
					t.Fatal("source closed before executor")
				}
				_, _ = io.Copy(io.Discard, req.Multipart.Source)
				if mode == "execute" {
					return Response{}, failure
				}
				if mode == "cancel-execute" {
					cancel()
					return Response{}, ctx.Err()
				}
				return Response{Status: 201}, nil
			}), func(entry EntryResult) error {
				reports++
				if !source.closed || entry.Entry.Multipart != nil {
					t.Fatal("source retained at callback")
				}
				if mode == "report" {
					return failure
				}
				return nil
			})
			r.openUpload = func(path string) (uploadFile, error) {
				file, err := os.Open(path)
				if err != nil {
					return nil, err
				}
				source = &faultyUpload{observedUpload: &observedUpload{File: file}}
				if mode == "stat" {
					source.statErr = failure
				}
				if mode == "close" {
					source.closeErr = failure
				}
				if mode == "cancel-open" {
					cancel()
				}
				return source, nil
			}
			if mode == "cancel-before" {
				cancel()
			}
			result := r.Run(ctx, scenario)
			wantCalls, wantCompleted := 1, 0
			if mode == "stat" || mode == "cancel-before" || mode == "cancel-open" {
				wantCalls = 0
			}
			if mode == "report" {
				wantCompleted = 1
				if !errors.Is(result.ReportErr, failure) {
					t.Fatalf("result=%+v", result)
				}
			} else if result.Err == nil {
				t.Fatalf("result=%+v", result)
			}
			if calls != wantCalls || result.Summary.Started != wantCalls || result.Summary.Completed != wantCompleted || result.Summary.NotStarted != 2-wantCalls {
				t.Fatalf("calls=%d reports=%d result=%+v", calls, reports, result)
			}
			if source != nil && !source.closed {
				t.Fatal("source not closed")
			}
			if mode == "close" && (!errors.Is(result.Err, failure) || result.ErrorPosition.Line != 2) {
				t.Fatalf("close failure=%+v", result)
			}
		})
	}
}

func TestMultipartMixedBodiesAndCaptures(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "upload"), []byte("{{literal}}"), 0600); err != nil {
		t.Fatal(err)
	}
	scenario, err := parser.Parse("safe.hurl", strings.NewReader("POST http://localhost/inline\n{}\nHTTP 201\nPOST http://localhost/upload\n[MultipartFormData]\nfile: file,upload;\nHTTP 201\n[Captures]\nid: jsonpath \"$.id\"\n[Asserts]\njsonpath \"$.id\" == 42\nPOST http://localhost/external\nfile,upload;\nHTTP 201\nPOST http://localhost/final\n{\"id\":{{id}}}\nHTTP 201"))
	if err != nil {
		t.Fatal(err)
	}
	scenario.OriginDir = dir
	calls := 0
	r := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
		calls++
		switch calls {
		case 1:
			if string(req.Body) != "{}" || req.Multipart != nil {
				t.Fatalf("req=%+v", req)
			}
		case 2:
			if req.Multipart == nil || !req.RetainBody {
				t.Fatalf("req=%+v", req)
			}
			return Response{Status: 201, Body: []byte(`{"id":42}`)}, nil
		case 3:
			if string(req.Body) != "{{literal}}" || req.Multipart != nil {
				t.Fatalf("req=%+v", req)
			}
		case 4:
			if string(req.Body) != `{"id":42}` {
				t.Fatalf("req=%+v", req)
			}
		}
		return Response{Status: 201}, nil
	}), nil)
	result := r.Run(context.Background(), scenario)
	if result.Err != nil || result.Diverged || calls != 4 || result.Summary.Validated != 4 {
		t.Fatalf("calls=%d result=%+v", calls, result)
	}
}
