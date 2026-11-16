package cli

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestJSONAssertionHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"test", "--help"}} {
		code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("code=%d err=%s", code, stderr)
		}
		for _, want := range []string{`jsonpath "$.name" == "Pizza"`, `jsonpath "$.price" == 4500`, `jsonpath "$.active" == true`, "precisão exata", "sem float64", "ordem escrita", "erro de avaliação JSON (2)", "Captura secreta pendente", "sem avaliar/publicar", "count aceita ==, >, >=, < e <=", "não marcados", `jsonpath "$.items" count == 3`, `jsonpath "$.total" >= 1`, `jsonpath "$.items[0].id" exists`, `jsonpath "$.name" matches "^Pizza"`, "capturas, filtros ou JSONPath ampliado"} {
			if !strings.Contains(out, want) {
				t.Fatalf("help lacks %q", want)
			}
		}
		if strings.Contains(out, "assertions JSON ou outros operadores") {
			t.Fatal("stale unsupported JSON help")
		}
	}
}

func TestJSONAssertionCLI(t *testing.T) {
	for index, tc := range []struct {
		query, expected, body, diagnostic string
		code                              int
	}{
		{"$.x", `null`, `{"x":null}`, "", 0},
		{"$.x", `4500`, `{"x":"4500"}`, `esperado 4500, recebido "4500"`, 1},
		{"$.x", `null`, `{}`, `recebido ausente`, 1},
		{"$[1]", `null`, `[null]`, `recebido ausente`, 1},
		{"$.x", `""`, `{"x":null}`, `esperado "", recebido null`, 1},
		{"$.x.y", `null`, `{"x":1}`, `percurso JSONPath exige objeto`, 2},
		{"$", `null`, `{"unused":"body-private"`, `resposta não contém um único valor JSON válido`, 2},
		{"$", `null`, `null null`, `resposta não contém um único valor JSON válido`, 2},
	} {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			calls := 0
			file := writeScenario(t, "GET http://example.test/first\nHTTP *\n[Asserts]\n  jsonpath \""+tc.query+"\" == "+tc.expected+"\nGET http://example.test/next")
			code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(tc.body)}, nil
			}), "test", file)
			wantCalls := 1
			if tc.code == 0 {
				wantCalls = 2
			}
			if code != tc.code || calls != wantCalls || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out, stderr)
			}
			if tc.code != 0 && (!strings.Contains(stderr, `":4:3:`) || !strings.Contains(stderr, tc.query)) {
				t.Fatalf("missing query/position: %s", stderr)
			}
			if strings.Contains(out+stderr, "body-private") || strings.Contains(out+stderr, "erro de transporte") || strings.Contains(out+stderr, "erro de captura") {
				t.Fatalf("unsafe/wrong category: %s %s", out, stderr)
			}
			if tc.code != 0 && !strings.Contains(out, "validadas=0 sem_validação=1 não_iniciadas=1") {
				t.Fatal(out)
			}
		})
	}
}

func TestJSONAndHeaderPendingSecretCLI(t *testing.T) {
	for _, line := range []string{`header "X" == "expected-private"`, `jsonpath "$.value" == "expected-private"`} {
		file := writeScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\n"+line+"\n[Captures]\ntoken: jsonpath \"$.missing.bad\"\nGET http://example.test/next")
		code, out, stderr := execute(t, context.Background(), executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
			return runner.Response{Status: 200, Body: []byte(`{"value":"actual-private","unused":"body-private"}`), Headers: []query.HeaderValue{{Name: "X", Value: "actual-private"}}}, nil
		}), "test", file, "--secret", "token")
		if code != 1 || !strings.Contains(stderr, "[oculto]") {
			t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
		}
		for _, secret := range []string{"expected-private", "actual-private", "body-private"} {
			if strings.Contains(out+stderr, secret) {
				t.Fatalf("leaked %s", secret)
			}
		}
	}
}

func TestJSONWriteFailureHasPriority(t *testing.T) {
	for _, body := range []string{`1`, `invalid`} {
		for _, short := range []bool{false, true} {
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 200, Body: []byte(body)}, nil
			}), BuildInfo{})
			command.SetArgs([]string{"test", writeScenario(t, "GET http://example.test\nHTTP *\n[Asserts]\njsonpath \"$\" == 2\nGET http://example.test/next")})
			var stderr bytes.Buffer
			command.SetErr(&stderr)
			if short {
				command.SetOut(shortWriter{})
			} else {
				command.SetOut(&failAfterWriter{cause: errors.New("write-private")})
			}
			if code := command.ExecuteContext(context.Background()); code != 2 || calls != 1 || !strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(stderr.String(), "write-private") {
				t.Fatalf("code=%d calls=%d stderr=%s", code, calls, stderr.String())
			}
		}
	}
}
