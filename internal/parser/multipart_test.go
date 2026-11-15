package parser

import (
	"strings"
	"testing"
)

func TestMultipartModel(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH"} {
		t.Run(method, func(t *testing.T) {
			scenario, err := Parse("upload.hurl", strings.NewReader(method+" http://localhost/documents\n[MultipartFormData]\ndescription: Documento de teste\ndocument: file,../fixtures/olá mundo.pdf; application/pdf\nHTTP 201"))
			if err != nil {
				t.Fatal(err)
			}
			form := scenario.Entries[0].Multipart
			if form == nil || len(form.Parts) != 2 || form.Parts[0].Value != "Documento de teste" || form.Parts[1].File.Path != "../fixtures/olá mundo.pdf" || form.Parts[1].MIME != "application/pdf" || form.Parts[1].ValuePosition.Line != 4 {
				t.Fatalf("form=%+v", form)
			}
		})
	}
}

func TestMultipartRejectsStructure(t *testing.T) {
	for _, body := range []string{
		"[MultipartFormData]\na: text", "[MultipartFormData]\na: file,a;\nb: file,b;", "[MultipartFormData]\na: file,a;\na: text", "{}\n[MultipartFormData]\na: file,a;", "file,a;\n[MultipartFormData]\na: file,a;", "[MultipartFormData]\na: file,a;\n{}", "[MultipartFormData]\na: file,a;\n[MultipartFormData]", "HTTP 200\n[MultipartFormData]\na: file,a;",
	} {
		t.Run(body, func(t *testing.T) {
			scenario, err := Parse("bad.hurl", strings.NewReader("GET http://localhost/first\nPOST http://localhost/upload\n"+body))
			if err == nil || len(scenario.Entries) != 0 {
				t.Fatalf("scenario=%+v err=%v", scenario, err)
			}
		})
	}
}
