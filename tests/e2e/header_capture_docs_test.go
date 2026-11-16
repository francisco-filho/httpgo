package e2e

import (
	"os"
	"strings"
	"testing"
)

func TestHeaderCaptureHelpAndPublishedExample(t *testing.T) {
	code, out, stderr := run(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
	for _, term := range []string{`catalog_etag: header "ETag"`, `If-None-Match: {{catalog_etag}}`, "primeira ocorrência", "Ausente: erro de captura 2", "capturar Set-Cookie não cria Cookie"} {
		if !strings.Contains(out, term) {
			t.Errorf("help missing %q", term)
		}
	}
	input, err := os.ReadFile("../../testdata/scenarios/scenario_capture_cache.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../../README.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
			t.Errorf("%s diverges from published cache scenario", file)
		}
	}
}
