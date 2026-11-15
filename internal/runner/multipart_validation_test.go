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

func TestMultipartTextAndMIME(t *testing.T) {
	for _, tc := range []struct{ filename, explicit, want string }{
		{"a.PDF", "", "application/pdf"}, {"a.txt", "", "text/plain"}, {"a.json", "", "application/json"}, {"a.unknown", "", "application/octet-stream"}, {"a.pdf", "text/plain", "text/plain"},
	} {
		t.Run(tc.filename+tc.explicit, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.filename), []byte("not inspected {{missing}}"), 0600); err != nil {
				t.Fatal(err)
			}
			scenario, err := parser.Parse("safe.hurl", strings.NewReader("POST http://localhost\nContent-Type: {{kind}}\n[MultipartFormData]\na: prefix-{{secret}}-suffix\nb: {{empty}}\nc: {{number}}/{{boolean}}/{{null}}\nd: {{literal}}\ne: {\"key\":true}\nf: file,"+tc.filename+"; "+tc.explicit+"\nHTTP 201"))
			if err != nil {
				t.Fatal(err)
			}
			scenario.OriginDir = dir
			renderer, err := template.New([]string{"kind=multipart/form-data", "secret=fictitious-token", "empty=", "number=42", "boolean=true", "null=null", "literal={{unresolved}}"}, []string{"secret"})
			if err != nil {
				t.Fatal(err)
			}
			r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
				parts := req.Multipart.Parts
				want := []string{"prefix-fictitious-token-suffix", "", "42/true/null", "{{unresolved}}", `{"key":true}`}
				for i, value := range want {
					if parts[i].Value != value {
						t.Errorf("part %d=%q want=%q", i, parts[i].Value, value)
					}
				}
				if parts[5].MIME != tc.want {
					t.Errorf("MIME=%q want=%q", parts[5].MIME, tc.want)
				}
				return Response{Status: 201}, nil
			}), renderer, func(entry EntryResult) error {
				if entry.Entry.Multipart != nil || strings.Contains(fmt.Sprintf("%+v", entry), "fictitious-token") {
					t.Fatalf("unsafe callback=%+v", entry)
				}
				return nil
			})
			result := r.Run(context.Background(), scenario)
			if result.Err != nil {
				t.Fatal(result.Err)
			}
		})
	}
}

func TestMultipartPreparationFailuresPreserveCounts(t *testing.T) {
	for _, mode := range []string{"missing", "directory", "permission", "stat", "variable", "array", "object", "content-type", "boundary"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "payload"), []byte("payload-private"), 0600); err != nil {
				t.Fatal(err)
			}
			scenario, err := parser.Parse("safe.hurl", strings.NewReader("GET http://localhost/first\nHTTP 200\nPOST http://localhost/second\nContent-Type: {{kind}}\n[MultipartFormData]\na: prefix-{{secret}}-suffix\nb: {{value}}\nfile: file,payload;\nGET http://localhost/last"))
			if err != nil {
				t.Fatal(err)
			}
			scenario.OriginDir = dir
			values := map[string]string{"secret": "token-private", "value": "text", "kind": "multipart/form-data"}
			switch mode {
			case "variable":
				delete(values, "value")
			case "array":
				values["value"] = "[]"
			case "object":
				values["value"] = "{}"
			case "content-type":
				values["kind"] = "application/json"
			case "boundary":
				values["kind"] = "multipart/form-data; boundary=x"
			}
			var assignments []string
			for key, value := range values {
				assignments = append(assignments, key+"="+value)
			}
			renderer, _ := template.New(assignments, []string{"secret"})
			calls, reports := 0, 0
			r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{Status: 200}, nil }), renderer, func(entry EntryResult) error {
				reports++
				if entry.Entry.Multipart != nil || strings.Contains(fmt.Sprintf("%+v", entry), "token-private") {
					t.Fatalf("unsafe callback=%+v", entry)
				}
				return nil
			})
			cause := map[string]error{"missing": fs.ErrNotExist, "directory": errPayloadDirectory, "permission": fs.ErrPermission, "stat": io.ErrUnexpectedEOF}[mode]
			if cause != nil {
				r.openUpload = func(string) (uploadFile, error) {
					return nil, &os.PathError{Op: "open", Path: "token-private", Err: cause}
				}
			}
			result := r.Run(context.Background(), scenario)
			if result.Err == nil || calls != 1 || reports != 2 || result.Summary != (Summary{Total: 3, Started: 1, Completed: 1, Validated: 1, NotStarted: 2}) || result.ErrorPosition.Line < 4 {
				t.Fatalf("calls=%d reports=%d result=%+v", calls, reports, result)
			}
			if cause != nil && !errors.Is(result.Err, cause) {
				t.Fatalf("cause lost: %v", result.Err)
			}
			if strings.Contains(result.Err.Error(), "token-private") || strings.Contains(result.Err.Error(), "payload-private") {
				t.Fatal("unsafe error")
			}
		})
	}
}
