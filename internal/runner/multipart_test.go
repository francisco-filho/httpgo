package runner

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
)

type observedUpload struct {
	*os.File
	reads  int
	closed bool
}

func (f *observedUpload) Read(p []byte) (int, error) { f.reads++; return f.File.Read(p) }
func (f *observedUpload) Close() error               { f.closed = true; return f.File.Close() }

func TestMultipartPreparationIsIncrementalAndOwned(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "olá mundo.pdf")
	if err := os.WriteFile(path, []byte("{{missing}}\x00\xff"), 0600); err != nil {
		t.Fatal(err)
	}
	scenario, err := parser.Parse("upload.hurl", strings.NewReader("POST http://localhost\n[MultipartFormData]\ndescription: text\ndocument: file,olá mundo.pdf; application/pdf\nHTTP 201"))
	if err != nil {
		t.Fatal(err)
	}
	scenario.OriginDir = dir
	var source *observedUpload
	calls, reports := 0, 0
	r := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
		calls++
		if source.reads != 0 || source.closed || req.Body != nil || req.Multipart == nil {
			t.Fatalf("source=%+v req=%+v", source, req)
		}
		body := req.Multipart
		data, err := io.ReadAll(body.Source)
		if err != nil || string(data) != "{{missing}}\x00\xff" || body.Parts[1].Filename != "olá mundo.pdf" || body.Parts[1].MIME != "application/pdf" {
			t.Fatalf("body=%+v data=%q err=%v", body, data, err)
		}
		return Response{Status: 201}, nil
	}), func(result EntryResult) error {
		reports++
		if !source.closed || result.Entry.Multipart != nil {
			t.Fatalf("source=%+v result=%+v", source, result)
		}
		return nil
	})
	r.openUpload = func(path string) (uploadFile, error) {
		file, err := os.Open(path)
		source = &observedUpload{File: file}
		return source, err
	}
	result := r.Run(context.Background(), scenario)
	if result.Err != nil || calls != 1 || reports != 1 || result.Summary.Completed != 1 {
		t.Fatalf("result=%+v calls=%d reports=%d", result, calls, reports)
	}
}
