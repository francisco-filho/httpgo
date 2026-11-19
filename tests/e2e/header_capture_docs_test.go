package e2e

import (
	"os"
	"strings"
	"testing"
)

func TestHeaderCaptureHelpAndPublishedExample(t *testing.T) {
	assertDocumentContains(t, "docs/subset.md", `catalog_etag: header "ETag"`, `If-None-Match: {{catalog_etag}}`, "a primeira ocorrência", "Header ausente é erro de captura (2)", "não cria `Cookie`")
	input, err := os.ReadFile("../../testdata/scenarios/scenario_capture_cache.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../../docs/captures.md", "../../docs/subset.md"} {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
			t.Errorf("%s diverges from published cache scenario", file)
		}
	}
}
