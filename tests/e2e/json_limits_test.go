package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

func TestJSONStructureDemonstrationByBinary(t *testing.T) {
	data, err := os.ReadFile("../../testdata/scenarios/scenario_assert_structure.hurl")
	if err != nil {
		t.Fatal(err)
	}
	for _, first := range []string{`{"id":null}`, `{}`} {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			_, _ = io.WriteString(w, `{"items":[`+first+`,{"id":2},{"id":3}],"total":3}`)
		}))
		code, out, stderr := run(t, "test", scenario(t, string(data)+"\nGET {{base_url}}/next"), "--var", "base_url="+server.URL)
		server.Close()
		if first == `{}` {
			if code != 1 || hits.Load() != 1 || !strings.Contains(stderr, "exists: esperado presente, recebido ausente") || !strings.Contains(stderr, ":7:1:") || !strings.Contains(out, "não_iniciadas=1") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		} else if code != 0 || hits.Load() != 2 || stderr != "" {
			t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
		}
	}
}

func TestJSONNumericBoundariesByBinary(t *testing.T) {
	for _, op := range []string{">", ">=", "<", "<="} {
		for _, body := range []string{"-1.251", "-1.25", "-1.249"} {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				_, _ = io.WriteString(w, body)
			}))
			code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"\nHTTP *\n[Asserts]\njsonpath \"$\" "+op+" -1.25\nGET "+server.URL+"/next"))
			server.Close()
			pass := op == ">" && body == "-1.249" || op == ">=" && body != "-1.251" || op == "<" && body == "-1.251" || op == "<=" && body != "-1.249"
			wantCode, wantHits := 1, int32(1)
			if pass {
				wantCode, wantHits = 0, 2
			}
			if code != wantCode || hits.Load() != wantHits || pass && stderr != "" || !pass && !strings.Contains(stderr, op+": esperado -1.25, recebido "+body) {
				t.Fatalf("%s %s: code=%d hits=%d out=%s err=%s", body, op, code, hits.Load(), out, stderr)
			}
		}
	}
}

func TestJSONNumericTypesAndEvaluationByBinary(t *testing.T) {
	for _, tc := range []struct {
		body, expression, diagnostic string
		code                         int
	}{
		{`9007199254740993`, `jsonpath "$" > 9007199254740992`, "", 0},
		{`-0e99999999999999999999`, `jsonpath "$" >= 0`, "", 0},
		{`1e99999999999999999999999`, `jsonpath "$" > 1e99999999999999999999998`, "", 0},
		{`1e-99999999999999999999999`, `jsonpath "$" > 0`, "", 0},
		{`1.0000000000000000000001`, `jsonpath "$" > 1`, "", 0},
		{`"1"`, `jsonpath "$" > 0`, "comparação ordenada exige número; recebido string", 1},
		{`true`, `jsonpath "$" > 0`, "recebido booleano", 1},
		{`null`, `jsonpath "$" > 0`, "recebido null", 1},
		{`[]`, `jsonpath "$" > 0`, "recebido array", 1},
		{`{}`, `jsonpath "$" > 0`, "recebido objeto", 1},
		{`{}`, `jsonpath "$.missing" > 0`, "recebido ausente", 1},
		{`{"x":null}`, `jsonpath "$.x.value" > 0`, "percurso JSONPath exige objeto", 2},
		{``, `jsonpath "$" exists`, "erro de avaliação JSON", 2},
		{`null null`, `jsonpath "$" >= 0`, "erro de avaliação JSON", 2},
		{`{"body-private":`, `jsonpath "$" count == 0`, "erro de avaliação JSON", 2},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nHTTP *\n[Asserts]\n  "+tc.expression+"\nGET "+server.URL+"/next"))
			wantHits := int32(1)
			if tc.code == 0 {
				wantHits = 2
			}
			if code != tc.code || hits.Load() != wantHits || !strings.Contains(stderr, tc.diagnostic) || strings.Contains(out+stderr, "body-private") || tc.code == 0 && stderr != "" {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}

func TestJSONNumericInvalidNeverSends(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	for _, expression := range []string{`jsonpath "$" > "1"`, `jsonpath "$" >= true`, `jsonpath "$" < null`, `jsonpath "$" <= []`, `jsonpath "$" > {}`, `jsonpath "$" > {{n}}`, `jsonpath "$" > 1 2`, `jsonpath "$" >`, `jsonpath "$[*]" > 0`, `jsonpath "$" != 1`} {
		code, out, stderr := run(t, "test", scenario(t, "GET "+server.URL+"/first\nGET "+server.URL+"/late\nHTTP *\n[Asserts]\n"+expression))
		if code != 2 || !prevalidationSummary(out) || !strings.Contains(stderr, ":5:") || hits.Load() != 0 {
			t.Fatalf("%s: code=%d hits=%d out=%s err=%s", expression, code, hits.Load(), out, stderr)
		}
	}
}
