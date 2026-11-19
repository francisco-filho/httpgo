package cli

import (
	"context"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestBodyHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			t.Fatal("help executed HTTP")
			return runner.Response{}, nil
		}), args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		for _, want := range []string{"--render-asserts", "Documentação:", "docs/subset.md"} {
			if !strings.Contains(out, want) {
				t.Fatalf("help lacks %q", want)
			}
		}
	}
}

func TestBodyCLI(t *testing.T) {
	for _, tc := range []struct {
		name, expression, body, contentType, diagnostic string
		code                                            int
	}{
		{"pass", `body contains "customer"`, "customer body-private", "text/plain", "", 0},
		{"fail", `body not contains "passwordHash"`, "passwordHash body-private", "text/plain", "assertion body not contains", 1},
		{"empty contains", `body contains ""`, "", "text/plain", "", 0},
		{"empty not", `body not contains ""`, "", "text/plain", "assertion body not contains", 1},
		{"invalid decode", `body not contains "passwordHash"`, "\xffbody-private", "text/plain; charset=utf-8", "erro de avaliação de body", 2},
		{"unknown charset", `body contains ""`, "body-private", "text/plain; charset=private-label", "charset de resposta desconhecido", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			file := writeScenario(t, "GET http://example.test/first\nHTTP *\n[Asserts]\n  "+tc.expression+"\nGET http://example.test/next")
			code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(tc.body), Headers: []query.HeaderValue{{Name: "Content-Type", Value: tc.contentType}}}, nil
			}), "test", file)
			wantCalls := 1
			if tc.code == 0 {
				wantCalls = 2
			}
			if code != tc.code || calls != wantCalls || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatalf("code=%d calls=%d out=%s err=%s", code, calls, out, stderr)
			}
			if tc.code != 0 && !strings.Contains(stderr, `":4:3:`) {
				t.Fatal(stderr)
			}
			for _, forbidden := range []string{"body-private", "private-label", "erro de transporte"} {
				if strings.Contains(out+stderr, forbidden) {
					t.Fatalf("unexpected %q in %s %s", forbidden, out, stderr)
				}
			}
			if tc.code == 1 && !strings.Contains(out, "requisições_falha_assertion=1 requisições_erro=0") {
				t.Fatal(out)
			}
		})
	}
}
