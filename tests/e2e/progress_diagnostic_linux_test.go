//go:build linux

package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCompactRealTTYMultilineDiagnostic(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		t.Run(fmt.Sprintf("NO_COLOR=%t", noColor), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprint(w, `{"orders":[{"number":7,"items":[{"allergens":["private-body-allergen"]}]}]}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			var text strings.Builder
			for step := 1; step <= 7; step++ {
				fmt.Fprintf(&text, "GET %s/first/%d\nHTTP 200\n", server.URL, step)
			}
			text.WriteString("[Asserts]\n")
			for strings.Count(text.String(), "\n") < 113 {
				text.WriteByte('\n')
			}
			text.WriteString(`jsonpath "$.orders[?(@.number==7)].items[0].allergens" contains "SOY"` + "\n")
			for step := 8; step <= 15; step++ {
				fmt.Fprintf(&text, "GET %s/not-reached/%d\nHTTP 200\n", server.URL, step)
			}
			ttyScenario(t, dir, "27-phase-six-advanced-menu.hurl", text.String())
			ttyScenario(t, dir, "29-phase-seven-combos.hurl", strings.Repeat("GET "+server.URL+"/next\nHTTP 200\n", 13))
			p := startTerminalProcess(t, true, true, noColor, "test", dir)
			if code := p.wait(t); code != 1 || calls.Load() != 20 {
				t.Fatalf("code=%d sends=%d err=%q", code, calls.Load(), p.stderr.text())
			}
			diagnostic := p.stderr.text()
			message := `expected "SOY" => recebido tipo array`
			if !noColor {
				message = "\x1b[31m" + message + "\x1b[0m"
			}
			block := "  step 7 - Linha 114:1\n  [assertion]\n  jsonpath \"$.orders[?(@.number==[literal oculto])].items[0].allergens\" contains\n  " + message + "\n"
			index := strings.Index(diagnostic, block)
			next := strings.Index(diagnostic, "- 29-phase-seven-combos.hurl")
			if index < 0 || next < index+len(block) || !strings.Contains(diagnostic, "13/13\n") || strings.Contains(diagnostic, "private-body-allergen") || strings.Contains(diagnostic, "@.number==7") || strings.Contains(diagnostic, "execução interrompida") {
				t.Fatalf("missing safe ordered block %q in %q", block, diagnostic)
			}
			if !strings.Contains(p.out.text(), "Sucesso/erros: 1/1\n") {
				t.Fatalf("summary=%q", p.out.text())
			}
		})
	}
}
