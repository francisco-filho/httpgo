package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestJSONLimitsSecretsByBinary(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, expression := range []string{`jsonpath "$.secret_key" > 987654321`, `jsonpath "$.items" count == 987654321`, `jsonpath "$.secret_key_missing" exists`} {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/login" {
					_, _ = io.WriteString(w, `{"token":{"confidential_name":"secret_key","confidential_number":987654321}}`)
					return
				}
				_, _ = io.WriteString(w, `{"secret_key":0,"items":["body-private"],"unrelated":"body-private"}`)
			}))
			source := "GET {{base}}/assert\nHTTP *\n[Asserts]\n" + expression
			if pending {
				source += "\n[Captures]\ntoken: jsonpath \"$.missing.bad\""
			} else {
				source = "GET {{base}}/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\n" + source
			}
			source += "\nGET {{base}}/next"
			// The query is sanitized for known secrets. Pending captures cannot
			// reveal unknown future contents, but must conceal both operands.
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--secret", "token")
			server.Close()
			wantHits := int32(2)
			if pending {
				wantHits = 1
			}
			if code != 1 || hits.Load() != wantHits || !strings.Contains(stderr, "[oculto]") || !strings.Contains(out, "não_iniciadas=1") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			for _, secret := range []string{"987654321", "body-private"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("leaked %s: %s %s", secret, out, stderr)
				}
			}
			if !pending && strings.Contains(out+stderr, "secret_key") {
				t.Fatalf("known secret query leaked: %s %s", out, stderr)
			}
		}
	}
}

func TestJSONLimitsFinalResponseByBinary(t *testing.T) {
	for _, incomplete := range []bool{false, true} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.URL.Path == "/start" {
				w.Header().Set("Location", "/final")
				w.WriteHeader(302)
				_, _ = io.WriteString(w, `{"total":-1,"items":[]}`)
				return
			}
			if incomplete {
				w.Header().Set("Content-Length", "100")
			}
			_, _ = io.WriteString(w, `{"total":1,"items":[null]}`)
		}))
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/start\nHTTP *\n[Asserts]\njsonpath \"$.total\" >= 1\njsonpath \"$.items\" count == 1\njsonpath \"$.items[0]\" exists\n[Captures]\nid: jsonpath \"$.total\"\nGET "+server.URL+"/{{id}}"))
		server.Close()
		wantCode, wantHits := 0, int32(3)
		if incomplete {
			wantCode, wantHits = 2, 2
		}
		if code != wantCode || hits.Load() != wantHits || !incomplete && stderr != "" || incomplete && (!strings.Contains(out, "erro de transporte") || !strings.Contains(stderr, "falha ao consumir ou fechar a resposta")) {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}
