package e2e

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestVariablesDemoTwoDestinations(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_products_variables.hurl")
	if err != nil {
		t.Fatal(err)
	}
	file := scenario(t, string(input))
	for range 2 {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.URL.Path != "/products" || r.Header.Get("Authorization") != "Bearer fictitious-token" {
				t.Errorf("request=%+v", r)
			}
			w.WriteHeader(200)
		}))
		t.Cleanup(server.Close) // Keep both listeners alive to ensure distinct destinations.
		code, out, stderr := run(t, "test", file, "--var", "api.baseUrl="+server.URL, "--var", "token=fictitious-token", "--secret", "token")
		if code != 0 || stderr != "" || hits.Load() != 1 || !strings.Contains(out, "validadas=1") || strings.Contains(out, "fictitious-token") {
			t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
		}
	}
}

func TestVariableTextAndSecretPathByBinary(t *testing.T) {
	const token = "fictitious%2F%20%25%3F%C3%A9"
	for _, status := range []int{200, 503} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/last" {
					w.WriteHeader(200)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if r.RequestURI != "/"+token+"?q=a/b" || r.Header.Get("X-Ordinary") != token || r.Header.Get("X-Text") != "a=b=c|42|true|{{literal}}|" || string(body) != `{ "static": false }` {
					t.Errorf("uri=%s headers=%v body=%q", r.RequestURI, r.Header, body)
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			text := "POST {{ base }}/{{token}}?q=a/b\nX-Ordinary: {{token}}\nX-Text: {{eq}}|{{number}}|{{bool}}|{{raw}}|{{empty}}\n{ \"static\": false }\nHTTP 200\nGET {{base}}/last"
			code, out, stderr := run(t, "test", scenario(t, text), "--secret=token", "--var", "token="+token, "--var", "base="+server.URL, "--var", "eq=a=b=c", "--var", "number=42", "--var", "bool=true", "--var", "raw={{literal}}", "--var", "empty=")
			wantCode, wantHits := 0, int32(2)
			if status == 503 {
				wantCode, wantHits = 1, 1
			}
			if code != wantCode || hits.Load() != wantHits || !strings.Contains(out, "resumo") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
			decoded, _ := url.PathUnescape(token)
			for _, secret := range []string{token, decoded, url.PathEscape(token)} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("leaked %q in %q %q", secret, out, stderr)
				}
			}
		})
	}
}

func TestVariablesInvalidInputBeforeNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	cases := []struct {
		suffix string
		flags  []string
	}{
		{suffix: "GET {{bad-name}}"}, {suffix: "GET {{ name"}, {suffix: "GET {{base}} trailing"}, {suffix: "POST {{base}}\n{\"{{token}}\":1}"}, {suffix: "GET {{base}}\nX-{{name}}: value"},
		{flags: []string{"--var", "noEquals"}}, {flags: []string{"--var", "bad-name=fictitious"}}, {flags: []string{"--var", "x=a", "--var", "x=a"}}, {flags: []string{"--var", "x=a", "--var", "x=b"}}, {flags: []string{"--var", "x=a", "--var", "x.y=b"}}, {flags: []string{"--var", "x.y=b", "--var", "x=a"}}, {flags: []string{"--var"}}, {flags: []string{"--secret"}}, {flags: []string{"--unknown=fictitious"}},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			text := "GET {{base}}/first\nHTTP 200\n" + tc.suffix
			args := []string{"test", scenario(t, text), "--var", "base=" + server.URL, "--var", "token=fictitious", "--secret", "token"}
			args = append(args, tc.flags...)
			code, out, stderr := run(t, args...)
			if code != 2 || out != "" || stderr == "" || hits.Load() != 0 || strings.Contains(stderr, "fictitious") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
		})
	}
}

func TestVariablesFinalValidationStopsAffectedAndLater(t *testing.T) {
	for _, tc := range []struct{ name, second, value string }{
		{"missing", "GET {{base}}/{{missing}}", "value=defined"},
		{"relative", "GET {{value}}", "value=/relative"},
		{"scheme", "GET {{value}}", "value=ftp://example.test"},
		{"host", "GET {{value}}", "value=http://[broken]"},
		{"port", "GET {{value}}", "value=http://example.test:65536"},
		{"encoding", "GET {{value}}", "value=http://example.test/%zz"},
		{"query encoding", "GET {{value}}", "value=http://example.test/?x=%zz"},
		{"space", "GET {{base}}/{{value}}", "value=not encoded"},
		{"path braces", "GET {{base}}/{{value}}", "value={unencoded}"},
		{"header CRLF", "GET {{base}}/affected\nX-Probe: {{value}}", "value=fictitious\r\nInjected: yes"},
		{"header control", "GET {{base}}/affected\nX-Probe: {{value}}", "value=fictitious\x7f"},
		{"Host", "GET {{base}}/affected\nHost: {{value}}", "value=user:fictitious@example.test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path != "/first" {
					t.Errorf("unexpected request %s", r.URL.Path)
				}
				w.WriteHeader(200)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/first\nHTTP 200\n"+tc.second+"\nGET {{base}}/last"), "--secret", "value", "--var", "base="+server.URL, "--var", tc.value)
			if code != 2 || hits.Load() != 1 || !strings.Contains(out, "iniciadas=1 concluídas=1 validadas=1") || !strings.Contains(out, "não_iniciadas=2") || !strings.Contains(stderr, "erro de renderização") || !strings.Contains(stderr, "input.hurl\":3:") && !strings.Contains(stderr, "input.hurl\":4:") || strings.Contains(out+stderr, "fictitious") {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
		})
	}
}

func TestVariablesEmptyAndMissingSecret(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	for _, tc := range []struct {
		source string
		flags  []string
		code   int
		hits   int32
	}{
		{source: "GET {{base}}/", flags: []string{"--secret", "future"}, code: 0, hits: 1},
		{source: "GET {{base}}/{{future}}", flags: []string{"--secret", "future"}, code: 2, hits: 0},
		{source: "GET {{base}}/{{future}}", flags: []string{"--secret", "future", "--var", "future="}, code: 0, hits: 1},
	} {
		hits.Store(0)
		args := append([]string{"test", scenario(t, tc.source), "--var", "base=" + server.URL}, tc.flags...)
		code, out, stderr := run(t, args...)
		if code != tc.code || hits.Load() != tc.hits || !strings.Contains(out, "resumo") {
			t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
		}
	}
}

func TestVariablesTLSFailureProtectsOutputs(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/{{token}}\nX-Ordinary: {{token}}\nHTTP 200\nGET {{base}}/last"), "--var", "base="+server.URL, "--var", "token=fictitious%2Ftoken", "--secret", "token")
	if code != 2 || hits.Load() != 0 || !strings.Contains(out, "erro de transporte") || strings.Contains(out+stderr, "fictitious") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}
