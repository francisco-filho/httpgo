package runner

import (
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

func TestExternalRenderFailuresUseOriginalPayloadPositions(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, tc := range []struct {
			name, source, assignment, detail string
			line, column                     int
		}{
			{"empty", "", "value=ok", "JSON", 1, 1},
			{"whitespace", " \t", "value=ok", "JSON", 1, 3},
			{"non JSON", "private-payload", "value=ok", "JSON", 1, 1},
			{"invalid UTF8", "\"\xff\"", "value=ok", "UTF-8", 1, 1},
			{"two values", "{} []", "value=ok", "após", 1, 3},
			{"Hurl suffix", "{}\nHTTP 200", "value=ok", "após", 1, 3},
			{"missing", "{\n \"escaped\":\"{{value}}\",\n \"next\":{{missing}}\n}", `value="quotes\"\\\n雪"`, "ausente", 3, 9},
			{"array in string", "{\n \"escaped\":\"{{value}}\"\n}", `value=[1,2]`, "array ou objeto", 2, 13},
			{"object in string", "{\n \"escaped\":\"{{value}}\"\n}", `value={"x":1}`, "array ou objeto", 2, 13},
			{"dynamic key", "{\n \"{{value}}\":1\n}", "value=ok", "chave JSON", 2, 3},
			{"malformed", "{\n \"escaped\":\"{{value}}\",\n \"next\":{{missing\n}", "value=ok", "fechamento ausente", 3, 9},
			{"opaque object", `{{product.name}}`, `product={"name":"Pizza"}`, "ausente", 1, 1},
		} {
			t.Run(tc.name+newline, func(t *testing.T) {
				source := strings.ReplaceAll(tc.source, "\n", newline)
				scenario, err := parser.Parse("safe.hurl", strings.NewReader("GET http://example.test/first\nHTTP 200\nPOST http://example.test/affected\nfile,payload.json;\nGET http://example.test/last"))
				if err != nil {
					t.Fatal(err)
				}
				renderer, err := template.New([]string{tc.assignment}, []string{"value"})
				if err != nil {
					t.Fatal(err)
				}
				calls, reads, reports := 0, 0, 0
				r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: 200}, nil }), renderer, func(entry EntryResult) error {
					reports++
					if entry.Entry.ExternalBody != nil || entry.Entry.Body != nil || entry.Entry.Multipart != nil {
						t.Fatalf("unsafe callback=%+v", entry)
					}
					return nil
				})
				r.readFile = func(context.Context, string) ([]byte, error) {
					reads++
					if calls != 1 || reports != 1 {
						t.Fatalf("premature read calls=%d reports=%d", calls, reports)
					}
					return []byte(source), nil
				}
				result := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: true})
				var failure *RenderError
				wantPos := parser.Position{File: "payload.json", Line: tc.line, Column: tc.column}
				wantRef := parser.Position{File: "safe.hurl", Line: 4, Column: 1}
				if !errors.As(result.Err, &failure) || failure.Reference == nil || *failure.Reference != wantRef || result.ErrorPosition != wantPos || failure.Position != wantPos || !strings.Contains(failure.Detail, tc.detail) || calls != 1 || reads != 1 || reports != 2 || result.Summary != (Summary{Total: 3, Started: 1, Completed: 1, Validated: 1, NotStarted: 2}) || result.Entries[0].Outcome != Passed || result.Entries[1].Outcome != RenderFailed || result.Entries[1].Entry.Position != wantPos {
					t.Fatalf("result=%+v failure=%+v calls=%d reads=%d reports=%d want=%+v", result, failure, calls, reads, reports, wantPos)
				}
				if strings.Contains(result.Err.Error(), "private-payload") || strings.Contains(result.Err.Error(), "quotes") {
					t.Fatalf("unsafe error=%v", result.Err)
				}
			})
		}
	}
}

func TestExternalDefensiveFinalJSONValidation(t *testing.T) {
	scenario, err := parser.Parse("safe.hurl", strings.NewReader("GET http://example.test/first\nHTTP 200\nPOST http://example.test/affected\nfile,payload.json;\nGET http://example.test/last"))
	if err != nil {
		t.Fatal(err)
	}
	renderer, _ := template.New(nil, nil)
	calls := 0
	r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: 200}, nil }), invalidJSONRenderer{renderer}, nil)
	r.readFile = func(context.Context, string) ([]byte, error) { return []byte(" \n{}\n"), nil }
	got := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: true})
	var failure *RenderError
	if !errors.As(got.Err, &failure) || failure.Position != (parser.Position{File: "payload.json", Line: 1, Column: 1}) || !strings.Contains(failure.Detail, "renderizado inválido") || calls != 1 || got.Summary.Started != 1 {
		t.Fatalf("got=%+v failure=%+v calls=%d", got, failure, calls)
	}
	if strings.Contains(got.Err.Error(), "secret") {
		t.Fatalf("unsafe error=%v", got.Err)
	}
}

func TestExternalRenderedReadAndCloseFailures(t *testing.T) {
	for _, cause := range []error{fs.ErrNotExist, fs.ErrPermission, io.ErrUnexpectedEOF, io.ErrClosedPipe} {
		t.Run(cause.Error(), func(t *testing.T) {
			scenario, err := parser.Parse("safe.hurl", strings.NewReader("GET http://example.test/first\nHTTP 200\nPOST http://example.test/affected\nfile,private;\nGET http://example.test/last"))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			r := New(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: 200}, nil }), nil)
			r.readFile = func(context.Context, string) ([]byte, error) { return []byte("partial private payload"), cause }
			got := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: true})
			var failure *FileError
			if !errors.As(got.Err, &failure) || !errors.Is(got.Err, cause) || calls != 1 || got.Entries[1].Outcome != FileFailed || got.Summary.Started != 1 {
				t.Fatalf("got=%+v calls=%d", got, calls)
			}
		})
	}
}

func TestExternalRenderedDescriptorClosedBeforeSend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload.json")
	if err := os.WriteFile(path, []byte(`{"x":{{value}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	scenario, err := parser.Parse("safe.hurl", strings.NewReader("POST http://example.test\nfile,"+path+";"))
	if err != nil {
		t.Fatal(err)
	}
	renderer, _ := template.New([]string{"value=true"}, nil)
	var descriptor *os.File
	r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
		if _, err := descriptor.Stat(); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("descriptor open: %v", err)
		}
		if string(req.Body) != `{"x":true}` {
			t.Fatalf("body=%q", req.Body)
		}
		return Response{Status: 200}, nil
	}), renderer, nil)
	r.readFile = func(ctx context.Context, path string) ([]byte, error) {
		var err error
		descriptor, err = os.Open(path)
		if err != nil {
			return nil, err
		}
		return readAndClosePayload(ctx, descriptor)
	}
	got := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: true})
	if got.Err != nil || got.Summary.Started != 1 {
		t.Fatalf("got=%+v", got)
	}
}

type cancelJSONRenderer struct {
	*template.Context
	cancel context.CancelFunc
}

func (r cancelJSONRenderer) NewSession() template.Session {
	return cancelJSONRenderer{r.Context.NewSession().(*template.Context), r.cancel}
}
func (r cancelJSONRenderer) RenderJSON(source []byte) ([]byte, error) {
	r.cancel()
	return r.Context.RenderJSON(source)
}

func TestExternalCancellationAfterRendering(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	renderer, _ := template.New(nil, nil)
	scenario, err := parser.Parse("safe.hurl", strings.NewReader("POST http://example.test\nfile,payload;\nGET http://example.test/last"))
	if err != nil {
		t.Fatal(err)
	}
	r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { t.Fatal("unexpected send"); return Response{}, nil }), cancelJSONRenderer{renderer, cancel}, nil)
	r.readFile = func(context.Context, string) ([]byte, error) { return []byte("{}"), nil }
	got := r.RunWithOptions(ctx, scenario, Options{RenderFiles: true})
	if !errors.Is(got.Err, context.Canceled) || got.Summary.Started != 0 || got.Summary.NotStarted != 2 {
		t.Fatalf("got=%+v", got)
	}
}

func TestExternalRenderReporterFailure(t *testing.T) {
	scenario, err := parser.Parse("safe.hurl", strings.NewReader("POST http://example.test\nfile,payload;"))
	if err != nil {
		t.Fatal(err)
	}
	writeErr := fmt.Errorf("writer failed")
	r := New(executorFunc(func(context.Context, Request) (Response, error) { t.Fatal("unexpected send"); return Response{}, nil }), func(EntryResult) error { return writeErr })
	r.readFile = func(context.Context, string) ([]byte, error) { return []byte("{{missing}}"), nil }
	got := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: true})
	if !errors.Is(got.ReportErr, writeErr) || got.Err == nil || got.Summary.Started != 0 {
		t.Fatalf("got=%+v", got)
	}
}
