package e2e

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRegexCaptureHelpAndPublishedExamples(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := run(t, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
		for _, term := range []string{`otp_code: regex "verificação é: ([0-9]{6})"`, `refresh_token: header "Set-Cookie" regex "refresh=([^;]+)"`, "primeiro grupo da primeira correspondência", "--secret otp_code", "sem interpolação", "grupo não participante"} {
			if !strings.Contains(out, term) {
				t.Errorf("help missing %q", term)
			}
		}
	}
	for _, scenario := range []string{"scenario_capture_otp.hurl", "scenario_capture_refresh.hurl"} {
		input, err := os.ReadFile("../../testdata/scenarios/" + scenario)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{"../../docs/captures.md", "../../docs/subset.md"} {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), strings.TrimSpace(string(input))) {
				t.Errorf("%s diverges from %s", file, scenario)
			}
		}
	}
}

func TestRegexCaptureStructuralCorpusByBinary(t *testing.T) {
	for _, file := range []string{"invalid_capture_regex_late.hurl", "invalid_capture_regex_escape.hurl", "unsupported_capture_regex_lookaround.hurl", "unsupported_capture_regex_backreference.hurl", "unsupported_capture_regex_group.hurl", "unsupported_capture_header_filter.hurl"} {
		t.Run(file, func(t *testing.T) {
			input, err := os.ReadFile("../../testdata/scenarios/" + file)
			if err != nil {
				t.Fatal(err)
			}
			for _, nl := range []string{"\n", "\r\n"} {
				var hits atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
				source := strings.ReplaceAll(string(input), "{{base_url}}", server.URL)
				// The existing negative pipeline fixture uses a literal URL.
				source = strings.ReplaceAll(source, "http://localhost", server.URL)
				source = strings.ReplaceAll(source, "\n", nl)
				code, out, stderr := run(t, "test", scenario(t, source))
				server.Close()
				category := "sintaxe inválida"
				if strings.HasPrefix(file, "unsupported") {
					category = "construção não suportada"
				}
				if code != 2 || hits.Load() != 0 || !strings.Contains(stderr, category) || strings.Contains(out+stderr, "private") {
					t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
				}
			}
		})
	}
}
