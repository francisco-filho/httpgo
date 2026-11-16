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

func TestJSONDemonstrationByBinary(t *testing.T) {
	data, err := os.ReadFile("../../testdata/scenarios/scenario_assert_json.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, price := range []string{`4500`, `"4500"`} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			_, _ = fmt.Fprintf(w, `{"name":"Pizza","price":%s,"active":true}`, price)
		}))
		source := string(data) + "\nGET {{base_url}}/next"
		code, out, stderr := run(t, "test", scenario(t, source), "--var", "base_url="+server.URL)
		server.Close()
		wantCode, wantHits := 0, int32(2)
		if price == `"4500"` {
			wantCode, wantHits = 1, 1
		}
		if code != wantCode || hits.Load() != wantHits {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
		if code == 1 && (!strings.Contains(stderr, `:6:1:`) || !strings.Contains(stderr, `esperado 4500, recebido "4500"`) || !strings.Contains(out, "não_iniciadas=1")) {
			t.Fatalf("out=%s err=%s", out, stderr)
		}
	}
}

func TestJSONAuthenticatedProductsByBinary(t *testing.T) {
	data, err := os.ReadFile("../../testdata/scenarios/scenario_assert_json_auth_products.hurl")
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if r.URL.Path != "/auth/login" && r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("captured token not sent")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/auth/login":
			if r.Method != "POST" || string(body) != `{"username":"demo","password":"senha-ficticia"}` {
				t.Errorf("login: %s %s", r.Method, body)
			}
			_, _ = io.WriteString(w, `{"accessToken":"fixture-token"}`)
		case "/products":
			if r.Method != "POST" || string(body) != `{"name":"Pizza","price":4500,"active":true}` {
				t.Errorf("create: %s %s", r.Method, body)
			}
			w.WriteHeader(201)
			_, _ = io.WriteString(w, `{"id":9007199254740993}`)
		case "/products/9007199254740993":
			if r.Method != "GET" {
				t.Errorf("method=%s", r.Method)
			}
			w.Header().Set("X-Request-Id", "req-123")
			_, _ = io.WriteString(w, `{"id":9007199254740993,"name":"Pizza","price":4500.0,"active":true}`)
		case "/echo":
			if r.Method != "POST" || string(body) != `{"productId":9007199254740993}` {
				t.Errorf("echo: %s %s", r.Method, body)
			}
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, string(data)), "--var", "base_url="+server.URL, "--var", "username=demo", "--var", "password=senha-ficticia", "--secret", "password", "--secret", "accessToken")
	if code != 0 || stderr != "" || hits.Load() != 4 || !strings.Contains(out, "validadas=4") || strings.Contains(out+stderr, "fixture-token") || strings.Contains(out+stderr, "senha-ficticia") {
		t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
	}
}

func TestJSONEqualityAndEvaluationByBinary(t *testing.T) {
	for index, tc := range []struct {
		body, expression string
		code             int
	}{
		{`{"s":"","n":1e0,"b":false,"null":null,"a":[],"o":{}}`, "jsonpath \"$.s\" == \"\"\njsonpath \"$.n\" == 1.0\njsonpath \"$.b\" == false\njsonpath \"$.null\" == null\njsonpath \"$.a\" == []\njsonpath \"$.o\" == {}", 0},
		{`{"items":[{"x":{"b":[true,null],"a":1}}]}`, `jsonpath "$.items[0].x" == {"a":1.0,"b":[true,null]}`, 0},
		{`{"x":[9007199254740993]}`, `jsonpath "$" == {"x":[9007199254740992]}`, 1},
		{`[1,2]`, `jsonpath "$" == [2,1]`, 1},
		{`1e99999999999999999999`, `jsonpath "$" == 10e99999999999999999998`, 0},
		{`{"x":"{{defined}}","a":["{{missing}}","\u96ea"]}`, `jsonpath "$" == {"x":"{{defined}}","a":["{{missing}}","雪"]}`, 0},
		{`{}`, `jsonpath "$.missing" == null`, 1},
		{`[]`, `jsonpath "$[0]" == null`, 1},
		{`null`, `jsonpath "$.x" == null`, 2},
		{`true`, `jsonpath "$.x" == null`, 2},
		{`"text"`, `jsonpath "$[0]" == null`, 2},
		{`{}`, `jsonpath "$[0]" == null`, 2},
		{`[]`, `jsonpath "$.x" == null`, 2},
		{``, `jsonpath "$" == null`, 2},
		{`null null`, `jsonpath "$" == null`, 2},
		{`{"body-private":`, `jsonpath "$" == null`, 2},
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); _, _ = io.WriteString(w, tc.body) }))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/assert\nHTTP *\n[Asserts]\n"+tc.expression+"\nGET {{base}}/next"), "--var", "base="+server.URL, "--var", "defined=resolved")
			wantHits := int32(1)
			if tc.code == 0 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits || strings.Contains(out+stderr, "body-private") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			if tc.code == 2 && (!strings.Contains(stderr, "erro de avaliação JSON") || strings.Contains(stderr, "erro de transporte")) {
				t.Fatal(stderr)
			}
			if strings.Contains(tc.expression, "missing\"") && code == 1 && !strings.Contains(stderr, "recebido ausente") {
				t.Fatal(stderr)
			}
		})
	}
}

func TestJSONPrecedenceByBinary(t *testing.T) {
	for _, tc := range []struct {
		expectation, expression, body, diagnostic string
		status, code                              int
	}{
		{"HTTP 200", `jsonpath "$" == null`, `invalid`, "status divergente", 503, 1},
		{"HTTP *", "header \"X\" exists\njsonpath \"$\" == null", `invalid`, "assertion header", 200, 1},
		{"HTTP *", "jsonpath \"$\" == null\nheader \"X\" exists", `invalid`, "erro de avaliação JSON", 200, 2},
		{"HTTP *", "jsonpath \"$\" == 2\njsonpath \"$.bad\" == null", `1`, "assertion jsonpath", 200, 1},
		{"HTTP *", `jsonpath "$" == 1`, `1`, "erro de captura", 200, 2},
	} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, tc.body)
		}))
		source := "GET {{base}}/first\n" + tc.expectation + "\n[Asserts]\n" + tc.expression + "\n[Captures]\nx: jsonpath \"$.missing\"\nGET {{base}}/next"
		code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL)
		server.Close()
		if code != tc.code || hits.Load() != 1 || !strings.Contains(stderr, tc.diagnostic) || !strings.Contains(out, "não_iniciadas=1") {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}

func TestJSONInvalidLateNeverSendsByBinary(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, suffix := range []string{"[Asserts]\njsonpath \"$\" == null", "HTTP *\n[Asserts]\njsonpath \"$\" == {{value}}", "HTTP *\n[Asserts]\njsonpath \"$\" == 1 2"} {
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nHTTP 200\nGET "+server.URL+"/late\n"+suffix))
		if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
	for _, name := range []string{"invalid_assert_json_literal.hurl", "unsupported_assert_json_operator.hurl", "unsupported_assert_json_filter.hurl", "unsupported_assert_query.hurl", "unsupported_assert_json_dynamic_query.hurl", "invalid_structure_count_negative.hurl", "invalid_structure_count_fractional.hurl", "invalid_structure_exists_operand.hurl", "invalid_structure_numeric_expected.hurl", "unsupported_capture_count.hurl"} {
		data, err := os.ReadFile("../../testdata/scenarios/" + name)
		if err != nil {
			t.Fatal(err)
		}
		code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(string(data), "http://localhost:8080", server.URL)))
		if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
			t.Fatalf("%s code=%d hits=%d out=%s err=%s", name, code, hits.Load(), out, stderr)
		}
	}
}

func TestJSONSecretsByBinary(t *testing.T) {
	for _, pending := range []bool{false, true} {
		for _, expression := range []string{`jsonpath "$.value" == {"part":"expected-private"}`, `header "X" == "expected-private"`} {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if hits.Add(1) == 1 && !pending {
					_, _ = io.WriteString(w, `{"token":"actual-private"}`)
					return
				}
				w.Header().Set("X", "actual-private")
				_, _ = io.WriteString(w, `{"value":{"part":"actual-private"},"unrelated":"body-private"}`)
			}))
			source := "GET {{base}}/assert\nHTTP *\n[Asserts]\n" + expression + "\nGET {{base}}/next"
			if pending {
				source = strings.Replace(source, "\n[Asserts]", "\n[Captures]\ntoken: jsonpath \"$.missing.bad\"\n[Asserts]", 1)
			} else {
				source = "GET {{base}}/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\n" + source
			}
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--var", "initial=expected-private", "--secret", "initial", "--secret", "token")
			server.Close()
			wantHits := int32(2)
			if pending {
				wantHits = 1
			}
			if code != 1 || hits.Load() != wantHits || !strings.Contains(stderr, "[oculto]") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
			for _, secret := range []string{"actual-private", "expected-private", "body-private"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("leaked %s: %s %s", secret, out, stderr)
				}
			}
		}
	}
}

func TestJSONFinalResponseAndIncompleteBodyByBinary(t *testing.T) {
	for _, mode := range []string{"final", "intermediate", "incomplete"} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			if r.URL.Path == "/start" {
				w.Header().Set("Location", "/final")
				w.WriteHeader(302)
				_, _ = io.WriteString(w, `{"x":"intermediate"}`)
				return
			}
			if mode == "incomplete" {
				w.Header().Set("Content-Length", "100")
			}
			_, _ = io.WriteString(w, `{"x":"final"}`)
		}))
		expected := `"final"`
		codeWant, hitsWant := 0, int32(3)
		if mode == "intermediate" {
			expected = `"intermediate"`
			codeWant, hitsWant = 1, 2
		}
		if mode == "incomplete" {
			codeWant, hitsWant = 2, 2
		}
		code, out, stderr := run(t, "test", scenario(t, "GET {{base}}/start\nHTTP *\n[Asserts]\njsonpath \"$.x\" == "+expected+"\nGET {{base}}/next"), "--var", "base="+server.URL)
		server.Close()
		if code != codeWant || hits.Load() != hitsWant || mode == "incomplete" && !strings.Contains(out, "concluídas=0") {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}
