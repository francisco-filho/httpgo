package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWildcardSequence(t *testing.T) {
	var hits atomic.Int32
	statuses := []int{200, 404, 503}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(statuses[hits.Add(1)-1]) }))
	defer server.Close()
	input, err := os.ReadFile("../../testdata/scenarios/scenario_wildcard.hurl")
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(string(input), "http://localhost:8080", server.URL)))
	if code != 0 || stderr != "" || hits.Load() != 3 || strings.Count(out, "status aceito por HTTP *") != 3 || !strings.Contains(out, "validadas=0 sem_validação=3") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}
