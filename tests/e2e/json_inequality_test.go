package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-415: real binary, secret capture, no dependent request after repetition.
func TestJSONInequalityRotation(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "scenarios", "scenario_json_inequality_rotation.hurl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, repeat := range []bool{false, true} {
		t.Run(map[bool]string{false: "rotated", true: "repeated"}[repeat], func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				token := "fake-old-private"
				switch r.URL.Path {
				case "/session/login":
				case "/session/renew":
					if !repeat {
						token = "fake-new-private"
					}
				case "/next":
					if r.Header.Get("Authorization") != "Bearer fake-new-private" {
						t.Error("wrong new token")
					}
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				_, _ = io.WriteString(w, `{"accessToken":"`+token+`"}`)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, string(data)), "--render-asserts", "--var", "base_url="+server.URL, "--secret", "old_token", "--secret", "new_token")
			wantCode, wantHits := 0, int32(3)
			if repeat {
				wantCode, wantHits = 1, 2
			}
			if code != wantCode || hits.Load() != wantHits || !repeat && stderr != "" || repeat && (!strings.Contains(stderr, "!=: esperado [oculto], recebido [oculto]") || !strings.Contains(out, "não_iniciadas=1")) {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			for _, secret := range []string{"fake-old-private", "fake-new-private"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatal("secret exposed")
				}
			}
		})
	}
}

// GO-416–423,427: CLI codes distinguish absence, equality and evaluation errors.
func TestJSONInequalityValues(t *testing.T) {
	for _, tc := range []struct {
		query, expected, body, diagnostic string
		code                              int
	}{
		{"$", `1`, `"1"`, "", 0}, {"$", `null`, `null`, "recebido null", 1},
		{"$", `""`, `null`, "", 0}, {"$", `1`, `null`, "", 0},
		{"$", `1`, `1e0`, "!=", 1}, {"$", `0`, `-0.0e99`, "!=", 1},
		{"$", `{"a":1,"b":2}`, `{"b":2,"a":1}`, "tipo objeto", 1},
		{"$", `[1,2]`, `[2,1]`, "", 0}, {"$", `9007199254740993`, `9007199254740992`, "", 0},
		{"$", `1e999999999999999999`, `2e999999999999999999`, "", 0},
		{"$.missing", `null`, `{}`, "recebido ausente", 1}, {"$[1]", `null`, `[]`, "recebido ausente", 1},
		{"$", `null`, `bad`, "erro de avaliação JSON", 2}, {"$.x.y", `1`, `{"x":1}`, "percurso", 2},
	} {
		t.Run(tc.query+tc.expected+tc.body, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); _, _ = io.WriteString(w, tc.body) }))
			defer server.Close()
			source := "GET " + server.URL + "\nHTTP 200\n[Asserts]\njsonpath \"" + tc.query + "\" != " + tc.expected + "\nGET " + server.URL + "/next"
			code, out, stderr := run(t, "test", scenario(t, source))
			wantHits := int32(1)
			if tc.code == 0 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits || !strings.Contains(stderr, tc.diagnostic) || tc.code == 0 && stderr != "" {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}
