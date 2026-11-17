package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestHeaderAssertionsByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		code             int
		diagnostic       string
	}{
		{"equal", `header "x-test" == "First,whole"`, 0, ""},
		{"equal fails", `header "X-Test" == "second"`, 1, `esperado "second", recebido "First,whole"`},
		{"equal case", `header "X-Test" == "first,whole"`, 1, `recebido "First,whole"`},
		{"contains", `header "X-Test" contains "st,wh"`, 0, ""},
		{"contains fails", `header "X-Test" contains "FIRST"`, 1, `recebido "First,whole"`},
		{"exists", `header "x-empty" exists`, 0, ""},
		{"exists fails", `header "Missing" exists`, 1, "recebido ausente"},
		{"absent equal empty", `header "Missing" == ""`, 1, `esperado "", recebido ausente`},
		{"absent contains empty", `header "Missing" contains ""`, 1, "recebido ausente"},
		{"empty equal", `header "X-Empty" == ""`, 0, ""},
		{"empty fails", `header "X-Empty" == "nonempty"`, 1, `recebido ""`},
		{"empty contains empty", `header "X-Empty" contains ""`, 0, ""},
		{"value contains empty", `header "X-Test" contains ""`, 0, ""},
		{"literal", `header "X-Literal" == "{{name}}"`, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.Header().Add("X-Test", "First,whole")
				w.Header().Add("X-Test", "second")
				w.Header().Set("X-Empty", "")
				w.Header().Set("X-Literal", "{{name}}")
			}))
			defer server.Close()
			file := scenario(t, "GET {{base}}/prior\nHTTP 200\nGET {{base}}/assert\nHTTP *\n[Asserts]\n"+tc.expression+"\nGET {{base}}/next")
			code, out, stderr := run(t, "test", file, "--var", "base="+server.URL, "--var", "name=resolved")
			wantHits := int32(3)
			if tc.code == 1 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits || !strings.Contains(stderr, tc.diagnostic) || !strings.Contains(out, "validadas=1") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			if code == 1 && (!strings.Contains(stderr, ":6:1: assertion header") || !strings.Contains(out, "não_iniciadas=1")) {
				t.Fatalf("out=%q err=%q", out, stderr)
			}
		})
	}
}

func TestHeaderDemonstrationByBinary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Request-Id", "req-123")
	}))
	defer server.Close()
	input, err := os.ReadFile("../../testdata/scenarios/scenario_assert_headers.hurl")
	if err != nil {
		t.Fatal(err)
	}
	code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(string(input), "http://localhost:8080", server.URL)))
	if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
}

func TestHeaderCapturePrecedenceByBinary(t *testing.T) {
	for _, tc := range []struct {
		name, expectation, body, header string
		status, code, hits              int
		diagnostic                      string
	}{
		{"status first", "HTTP 200", "invalid", "bad", 503, 1, 1, "status divergente"},
		{"header before body", "HTTP 200", `{"id":9007199254740993}`, "bad", 200, 1, 1, "assertion header"},
		{"wildcard header before body", "HTTP *", `{"id":9007199254740993}`, "bad", 503, 1, 1, "assertion header"},
		{"capture after headers", "HTTP 200", "invalid", "ok", 200, 2, 1, "erro de captura"},
		{"capture success", "HTTP 200", `{"id":9007199254740993}`, "ok", 200, 0, 2, ""},
		{"wildcard capture 200", "HTTP *", `{"id":9007199254740993}`, "ok", 200, 0, 2, ""},
		{"wildcard capture 404", "HTTP *", `{"id":9007199254740993}`, "ok", 404, 0, 2, ""},
		{"wildcard capture 503", "HTTP *", `{"id":9007199254740993}`, "ok", 503, 0, 2, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path != "/first" && r.URL.Path != "/9007199254740993" {
					t.Errorf("path=%q", r.URL.Path)
				}
				w.Header().Set("X-Test", tc.header)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			source := "GET {{base}}/first\n" + tc.expectation + "\n[Captures]\nid: jsonpath \"$.id\"\n[Asserts]\nheader \"X-Test\" == \"ok\"\nGET {{base}}/{{id}}"
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL)
			if code != tc.code || int(hits.Load()) != tc.hits || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestHeaderSensitiveValuesByBinary(t *testing.T) {
	for _, name := range []string{"aUtHoRiZaTiOn", "Proxy-Authorization", "Cookie", "sEt-CoOkIe", "X-Common"} {
		t.Run(name, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				w.Header().Set(name, "actual-private")
				w.Header().Set("Set-Cookie", "actual-private")
			}))
			defer server.Close()
			source := fmt.Sprintf("GET {{base}}/actual-private\nHTTP *\n[Asserts]\nheader %q == \"expected-private\"\nGET {{base}}/next", name)
			args := []string{"test", scenario(t, source), "--var", "base=" + server.URL}
			if name == "X-Common" {
				args = append(args, "--var", "token=expected-private", "--secret", "token")
			}
			code, out, stderr := run(t, args...)
			if code != 1 || hits.Load() != 1 || strings.Contains(out+stderr, "actual-private") || strings.Contains(out+stderr, "expected-private") || !strings.Contains(stderr, "[oculto]") {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
		})
	}
}

func TestInvalidHeaderAssertionsNeverSendByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, suffix := range []string{"[Asserts]", "HTTP *\n[Asserts]", "HTTP 200\n[Asserts]\nheader \"Authorization\" == private-value", "HTTP *\n[Asserts]\njsonpath \"$.private-value\" == \"x\"", "HTTP 200\n[Asserts]\nheader \"X\" startsWith \"private-value\"", "HTTP *\n[Asserts]\nheader \"X\" count == 1"} {
		source := "GET " + server.URL + "/prior\nHTTP 200\nGET " + server.URL + "/late\n" + suffix
		code, out, stderr := run(t, "test", scenario(t, source))
		if code != 2 || !prevalidationSummary(out) || hits.Load() != 0 || stderr == "" || strings.Contains(stderr, "private-value") {
			t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
		}
	}
}

func TestHeaderFinalResponseAndIncompleteBodyByBinary(t *testing.T) {
	for _, mode := range []string{"final success", "intermediate only", "incomplete"} {
		t.Run(mode, func(t *testing.T) {
			var hits, next atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/start":
					w.Header().Set("Location", "/final")
					w.Header().Set("X-Intermediate", "ok")
					w.WriteHeader(302)
				case "/final":
					w.Header().Set("X-Final", "ok")
					if mode == "incomplete" {
						w.Header().Set("Content-Length", "100")
					}
					_, _ = io.WriteString(w, "short")
				case "/next":
					next.Add(1)
				}
			}))
			defer server.Close()
			expression := `header "X-Final" == "ok"`
			wantCode := 0
			if mode == "intermediate only" {
				expression = `header "X-Intermediate" exists`
				wantCode = 1
			}
			if mode == "incomplete" {
				wantCode = 2
			}
			code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/start\nHTTP *\n[Asserts]\n"+expression+"\nGET {{base}}/next"), "--var", "base="+server.URL)
			wantHits, wantNext := int32(2), int32(0)
			if wantCode == 0 {
				wantHits, wantNext = 3, 1
			}
			if code != wantCode || hits.Load() != wantHits || next.Load() != wantNext {
				t.Fatalf("code=%d hits=%d next=%d out=%q err=%q", code, hits.Load(), next.Load(), out, stderr)
			}
			if mode == "incomplete" && !strings.Contains(out, "concluídas=0") {
				t.Fatalf("out=%q", out)
			}
		})
	}
}
