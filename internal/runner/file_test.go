package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

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
