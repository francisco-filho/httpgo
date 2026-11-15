package runner

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
)

type failingFileReader struct{ *os.File }

func (f failingFileReader) Read(p []byte) (int, error) {
	n, _ := f.File.Read(p)
	return n, io.ErrUnexpectedEOF
}

func TestExternalDescriptorClosedBeforeExecutorOrFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "payload")
		if err := os.WriteFile(path, []byte("literal"), 0600); err != nil {
			t.Fatal(err)
		}
		scenario, err := parser.Parse("source.hurl", strings.NewReader("POST http://example.test\nfile,"+path+";"))
		if err != nil {
			t.Fatal(err)
		}
		var descriptor *os.File
		calls := 0
		r := New(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			if _, err := descriptor.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("descriptor open at send: %v", err)
			}
			return Response{Status: 200}, nil
		}), nil)
		r.readFile = func(ctx context.Context, path string) ([]byte, error) {
			var err error
			descriptor, err = os.Open(path)
			if err != nil {
				return nil, err
			}
			if fail {
				return readAndClosePayload(ctx, failingFileReader{descriptor})
			}
			return readAndClosePayload(ctx, descriptor)
		}
		got := r.Run(context.Background(), scenario)
		if _, err := descriptor.Stat(); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("descriptor retained: %v", err)
		}
		if fail && (calls != 0 || !errors.Is(got.Err, io.ErrUnexpectedEOF)) || !fail && (calls != 1 || got.Err != nil) {
			t.Fatalf("fail=%v calls=%d result=%+v", fail, calls, got)
		}
	}
}

func TestExternalCancellationNeverStartsPrematurely(t *testing.T) {
	for _, stage := range []string{"before", "read", "transport"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			scenario, err := parser.Parse("source.hurl", strings.NewReader("POST http://example.test\nfile,literal;\nGET http://example.test/next"))
			if err != nil {
				t.Fatal(err)
			}
			calls, reads := 0, 0
			r := New(executorFunc(func(ctx context.Context, _ Request) (Response, error) {
				calls++
				cancel()
				return Response{}, ctx.Err()
			}), nil)
			r.readFile = func(context.Context, string) ([]byte, error) {
				reads++
				if stage == "read" {
					cancel()
				}
				return []byte("literal"), nil
			}
			if stage == "before" {
				cancel()
			}
			got := r.Run(ctx, scenario)
			wantCalls, wantReads := 0, 1
			if stage == "before" {
				wantReads = 0
			}
			if stage == "transport" {
				wantCalls = 1
			}
			if !errors.Is(got.Err, context.Canceled) || calls != wantCalls || reads != wantReads || got.Summary.Started != wantCalls || got.Summary.Completed != 0 || got.Summary.NotStarted != 2-wantCalls {
				t.Fatalf("calls=%d reads=%d got=%+v", calls, reads, got)
			}
		})
	}
}

func TestPayloadCloseFailureAndCanceledRead(t *testing.T) {
	reader := &payloadReader{failure: io.EOF, closeFailure: io.ErrClosedPipe}
	body, err := readAndClosePayload(context.Background(), reader)
	if body != nil || !errors.Is(err, io.ErrClosedPipe) || !reader.closed {
		t.Fatalf("body=%q err=%v closed=%v", body, err, reader.closed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reader = &payloadReader{}
	body, err = readAndClosePayload(ctx, reader)
	if body != nil || !errors.Is(err, context.Canceled) || !reader.closed {
		t.Fatalf("body=%q err=%v closed=%v", body, err, reader.closed)
	}
}
