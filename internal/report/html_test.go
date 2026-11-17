package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
)

func TestHTMLGoldenOfflineAndEscaping(t *testing.T) {
	document := HTMLDocument{
		Summary: suite.Summary{Files: 4, Passed: 1, Failed: 1, Errors: 1, Skipped: 1, UnknownFiles: 2, Duration: time.Second, Requests: runner.Summary{Total: 3, Started: 2, Completed: 2, Validated: 1, NotStarted: 1}},
		Files: []HTMLFile{
			{Result: suite.FileResult{Name: `01-ação "<script>alert('x')</script>.hurl`, State: suite.Passed, Known: true, Requests: runner.Summary{Total: 1, Started: 1}, Duration: time.Millisecond}, Entries: []HTMLEntry{{Number: 1, Method: "GET", URL: "http://example.test/<img>?[oculta]", Outcome: "sem validação de status", Duration: time.Millisecond, Diagnostic: `aspas " & Unicode: café <script>alert(1)</script>`}}},
			{Result: suite.FileResult{Name: "02.hurl", State: suite.Failed, Known: true, Requests: runner.Summary{Total: 2, Started: 1, NotStarted: 1}}, Diagnostic: `"02.hurl":4:1: status divergente: esperado 200, recebido 503`},
			{Result: suite.FileResult{Name: "03.hurl", State: suite.Errored}, Diagnostic: "sintaxe inválida"},
			{Result: suite.FileResult{Name: "04.hurl", State: suite.Skipped}},
		},
	}
	var out bytes.Buffer
	if err := WriteHTML(&out, document); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, forbidden := range []string{"<script", "<img", "<link", "src=", "href=", "url(", "@import", "template.HTML", "prefers-color-scheme"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("active/external content %q in HTML", forbidden)
		}
	}
	for _, want := range []string{"<style>", "background: #fff", "<table>", "<pre>", "&lt;script&gt;", "café", "total=desconhecido", "não iniciadas=1", "ausência de assertion não é validação"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in HTML", want)
		}
	}
	path := filepath.Join("..", "..", "testdata", "report", "html.html")
	if *update {
		if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Fatal("HTML differs from golden (-update to regenerate)")
	}
}
