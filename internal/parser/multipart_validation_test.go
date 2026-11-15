package parser

import (
	"errors"
	"strings"
	"testing"
)

func TestMultipartMetadataAndContentType(t *testing.T) {
	for _, tc := range []struct {
		name, header, part string
		valid              bool
	}{
		{"omitted", "", "file: file,a.pdf;", true},
		{"explicit", "Content-Type: multipart/form-data\n", "file: file,a.pdf; application/pdf", true},
		{"variable", "Content-Type: {{kind}}\n", "file: file,a.pdf;", true},
		{"type", "Content-Type: application/json\n", "file: file,a.pdf;", false},
		{"boundary", "Content-Type: multipart/form-data; boundary=\"\"\n", "file: file,a.pdf;", false},
		{"duplicate", "Content-Type: multipart/form-data\ncontent-type: multipart/form-data\n", "file: file,a.pdf;", false},
		{"dynamic-name", "", "{{name}}: file,a.pdf;", false},
		{"tab-name", "", "bad\tname: file,a.pdf;", false},
		{"tab-filename", "", "file: file,a\tb.pdf;", false},
		{"cr", "", "file: file,a.pdf; application/pdf\rX-Injection: yes", false},
		{"control", "", "file: file,a.pdf; application/pdf\x7f", false},
		{"mime", "", "file: file,a.pdf; bad", false},
		{"mime-template", "", "file: file,a.pdf; {{kind}}", false},
		{"missing-delimiter", "", "file: file,a.pdf", false},
		{"empty-path", "", "file: file,;", false},
		{"comma-path", "", "file: file,a,b;", false},
		{"template-syntax", "", "text: {{broken\nfile: file,a;", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario, err := Parse("safe.hurl", strings.NewReader("GET http://localhost/first\nPOST http://localhost\n"+tc.header+"[MultipartFormData]\n"+tc.part+"\nHTTP 201"))
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var located *Error
			if !errors.As(err, &located) || located.Position.Line < 3 || len(scenario.Entries) != 0 || strings.Contains(err.Error(), "X-Injection") {
				t.Fatalf("scenario=%+v err=%v", scenario, err)
			}
		})
	}
}
