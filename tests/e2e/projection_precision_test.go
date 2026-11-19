package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// GO-445: the original demo must distinguish adjacent integers above 2^53.
func TestProjectionPrecisionDemo(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "distinct", true: "repeated"}[changed], func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Method == "POST" {
					body, _ := io.ReadAll(r.Body)
					if string(body) != `[9007199254740992,9007199254740993]` {
						t.Errorf("capture=%s", body)
					}
					w.WriteHeader(204)
					return
				}
				second := "9007199254740993"
				if changed {
					second = "9007199254740992"
				}
				_, _ = io.WriteString(w, `{"products":[{"id":9007199254740992},{"id":`+second+`}]}`)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", filepath.Join("..", "..", "testdata", "scenarios", "scenario_projection_precision.hurl"), "--var", "base_url="+server.URL)
			wantCode, wantHits := 0, int32(2)
			if changed {
				wantCode, wantHits = 1, 1
			}
			if code != wantCode || hits.Load() != wantHits || !changed && stderr != "" || changed && !strings.Contains(stderr, ":6:1:") {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}

// GO-447: capture serialization and equality remain exact at every magnitude.
func TestProjectionExactNumberCapture(t *testing.T) {
	for _, tc := range []struct {
		numbers, expected string
		code              int
	}{
		{`[9223372036854775808,9223372036854775809]`, `[9223372036854775808,9223372036854775809]`, 0},
		{`[1.0000000000000000000001,1.0000000000000000000002]`, `[1.0000000000000000000001,1.0000000000000000000002]`, 0},
		{`[1.00,1e0,-0.00]`, `[1,1.0,0]`, 0},
		{`[1e99999999999999999999999,1e-99999999999999999999999]`, `[10e99999999999999999999998,10e-100000000000000000000000]`, 0},
		{`[1e-99999999999999999999999,2e-99999999999999999999999]`, `[1e-99999999999999999999999,1e-99999999999999999999999]`, 1},
	} {
		t.Run(tc.numbers, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Method == "POST" {
					body, _ := io.ReadAll(r.Body)
					if string(body) != tc.numbers {
						t.Errorf("capture=%s want=%s", body, tc.numbers)
					}
					w.WriteHeader(204)
					return
				}
				_, _ = io.WriteString(w, tc.numbers)
			}))
			defer server.Close()
			source := "GET " + server.URL + "\nHTTP 200\n[Asserts]\njsonpath \"$[*]\" == " + tc.expected + "\n[Captures]\nnumbers: jsonpath \"$[*]\"\nPOST " + server.URL + "\n{{numbers}}\nHTTP 204"
			code, out, stderr := run(t, "test", scenario(t, source))
			want := int32(1)
			if tc.code == 0 {
				want = 2
			}
			if code != tc.code || hits.Load() != want || tc.code == 0 && stderr != "" {
				t.Fatalf("code=%d hits=%d out=%s err=%s", code, hits.Load(), out, stderr)
			}
		})
	}
}
