package e2e

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCaptureRenewalHelpAndLateValidation(t *testing.T) {
	code, out, stderr := run(t, "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("help=%d %s %s", code, out, stderr)
	}
	for _, term := range []string{"US-34", "capturas provisórias", "confirmação", "mesmo invertendo as seções", "Consultas de captura usam somente", "sem nova flag", "query variable"} {
		if !strings.Contains(out, term) {
			t.Fatalf("help missing %q", term)
		}
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, line := range []string{`variable "id" == "new"`, `jsonpath "$.id" == {{broken`, `jsonpath "$.id" unsupported true`} {
		source := "GET " + server.URL + "/early\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"\nGET " + server.URL + "/late\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"\n[Asserts]\n" + line
		code, out, stderr := run(t, "test", scenario(t, source), "--render-asserts")
		if code != 2 || hits.Load() != 0 || !prevalidationSummary(out) || !strings.Contains(stderr, ":10:") {
			t.Fatalf("late %q code=%d hits=%d stdout=%s stderr=%s", line, code, hits.Load(), out, stderr)
		}
	}
}
