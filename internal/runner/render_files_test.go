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

func TestExternalJSONValuesAndStaticBytes(t *testing.T) {
	for _, tc := range []struct{ name, source, assignment, want string }{
		{"string", `{{product.value}}`, `product.value="quotes\"\\\n雪"`, `"quotes\"\\\n雪"`},
		{"number", `{{product.value}}`, `product.value=9007199254740993123456789`, `9007199254740993123456789`},
		{"boolean", `{{product.value}}`, `product.value=false`, `false`},
		{"null", `{{product.value}}`, `product.value=null`, `null`},
		{"array", `{{product.value}}`, `product.value=[9007199254740993,"{{literal}}"]`, `[9007199254740993,"{{literal}}"]`},
		{"object", `{"value":{{product.value}}}`, `product.value={"nested":[true,null]}`, `{"value":{"nested":[true,null]}}`},
		{"composition", `{"value":"prefix-{{product.value}}-suffix"}`, `product.value="quotes\"\\\n雪{{literal}}"`, `{"value":"prefix-quotes\"\\\n雪{{literal}}-suffix"}`},
		{"static LF", " \n{\n \"value\": [null, 12]\n}\t\n", "product.value=unused", " \n{\n \"value\": [null, 12]\n}\t\n"},
		{"static CRLF", "\r\n{\r\n \"value\": true\r\n}\r\n", "product.value=unused", "\r\n{\r\n \"value\": true\r\n}\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, absolute := range []bool{false, true} {
				dir := t.TempDir()
				path := filepath.Join(dir, " {{literal}} payload.data ")
				if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
					t.Fatal(err)
				}
				ref := filepath.Base(path)
				if absolute {
					ref = path
				}
				scenario, err := parser.Parse("source.hurl", strings.NewReader("POST http://example.test\nfile,"+ref+";\nHTTP 201"))
				if err != nil {
					t.Fatal(err)
				}
				scenario.OriginDir = dir
				renderer, err := template.New([]string{tc.assignment}, nil)
				if err != nil {
					t.Fatal(err)
				}
				calls := 0
				result := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
					calls++
					if !bytes.Equal(req.Body, []byte(tc.want)) || len(req.Headers) != 0 {
						t.Fatalf("body=%q headers=%v want=%q", req.Body, req.Headers, tc.want)
					}
					return Response{Status: 201}, nil
				}), renderer, nil).RunWithOptions(context.Background(), scenario, Options{RenderFiles: true})
				if result.Err != nil || result.Diverged || calls != 1 || result.Summary.Validated != 1 {
					t.Fatalf("result=%+v calls=%d", result, calls)
				}
			}
		})
	}
}
