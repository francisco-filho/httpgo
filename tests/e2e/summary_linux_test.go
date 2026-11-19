//go:build linux

package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTextSummaryRealAllModes(t *testing.T) {
	for _, mode := range []string{"legacy", "compact", "verbose", "bodies"} {
		t.Run(mode, func(t *testing.T) {
			var calls, polls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path == "/poll" && polls.Add(1) == 1 {
					w.WriteHeader(503)
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			ttyScenario(t, dir, "01.httpgo", strings.Repeat("GET "+server.URL+"/ok\nHTTP 200\n", 3))
			ttyScenario(t, dir, "02.httpgo", "GET "+server.URL+"/{{missing}}")
			ttyScenario(t, dir, "03.httpgo", "invalid syntax, not a step")
			ttyScenario(t, dir, "04.httpgo", "GET "+server.URL+"/poll\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200")
			args := []string{"test", dir}
			if mode == "verbose" {
				args = append(args, "-v")
			}
			if mode == "bodies" {
				args = append(args, "-vv")
			}
			p := startTerminalProcess(t, mode == "compact", mode == "compact", true, args...)
			if code := p.wait(t); code != 2 || calls.Load() != 5 || polls.Load() != 2 {
				t.Fatalf("code=%d sends=%d polls=%d err=%q", code, calls.Load(), polls.Load(), p.stderr.text())
			}
			out := p.out.text()
			for _, text := range []string{"Cenários executados: 4\n", "Sucesso/erros: 2/2\n", "Testes executados: 5\n", "Sucesso/erros: 4/1\n"} {
				if !strings.Contains(out, text) {
					t.Fatalf("missing %q in %q", text, out)
				}
			}
			durations := make(map[string]time.Duration)
			for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
				key, value, ok := strings.Cut(line, ": ")
				if ok && (key == "Tempo" || key == "Tempo médio por Cenário" || key == "Tempo médio por Teste") {
					duration, err := time.ParseDuration(value)
					if err != nil {
						t.Fatal(err)
					}
					durations[key] = duration
				}
			}
			if len(durations) != 3 || durations["Tempo médio por Cenário"] != durations["Tempo"]/4 || durations["Tempo médio por Teste"] != durations["Tempo"]/5 || !strings.HasSuffix(out, fmt.Sprintf("Tempo: %s\n", durations["Tempo"])) {
				t.Fatalf("wrong averages/order: %q", out)
			}
			details := p.stderr.text()
			if mode == "legacy" {
				details = out
			}
			if mode == "verbose" || mode == "bodies" {
				if strings.Count(details, "-----------------------------------------------\n") != 4 || strings.Contains(details, "Erro: 0") || !strings.Contains(details, "Sucesso: 3\n-----------------------------------------------\n") || !strings.Contains(details, "Sucesso: 0 - Erro: 1\n-----------------------------------------------\n") {
					t.Fatalf("wrong per-scenario balances: %q", details)
				}
			} else if strings.Contains(out+p.stderr.text(), "Sucesso: ") || strings.Contains(out+p.stderr.text(), "-----------------------------------------------") {
				t.Fatalf("verbose-only per-scenario balance in %s: out=%q err=%q", mode, out, p.stderr.text())
			}
		})
	}
}
