package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestPollingHelpAndPublishedExample(t *testing.T) {
	assertDocumentContains(t, "docs/subset.md", "[Options]", "retry: 3", "retry-interval: 500", "default `1000`", "POST: efeitos podem se repetir", "sem snapshot", "não há timeout global", "status_comparados")
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jobs/123" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if hits.Add(1) < 3 {
			_, _ = io.WriteString(w, `{"status":"PENDING"}`)
		} else {
			_, _ = io.WriteString(w, `{"status":"READY"}`)
		}
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", "../../testdata/scenarios/scenario_polling.hurl", "--var", "base_url="+server.URL)
	if code != 0 || stderr != "" || hits.Load() != 3 || !strings.Contains(out, "[tentativas=3]") || !strings.Contains(out, "iniciadas=1 concluídas=1 validadas=1") || !strings.Contains(out, "tentativas_iniciadas=3 respostas_completas=3 status_comparados=3") {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}
