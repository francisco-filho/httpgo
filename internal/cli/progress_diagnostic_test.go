package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
)

func TestCompactDiagnosticBlocks(t *testing.T) {
	pos := parser.Position{File: "must-not-repeat.httpgo", Line: 114, Column: 1}
	status := 200
	for _, test := range []struct {
		name         string
		entry        runner.EntryResult
		err, loadErr error
		want         string
	}{
		{name: "membership", entry: runner.EntryResult{JSONFailure: &runner.JSONFailure{Position: pos, Query: "$.orders[?(@.number==[literal oculto])].items[0].allergens", Operator: "contains", Expected: `"SOY"`, Actual: "tipo array; contagem=2", Present: true}}, want: "[assertion]\n  jsonpath \"$.orders[?(@.number==[literal oculto])].items[0].allergens\" contains\n     expected \"SOY\" => received tipo array\n"},
		{name: "json count", entry: runner.EntryResult{JSONFailure: &runner.JSONFailure{Position: pos, Query: "$.items", Operator: "==", Count: true, Expected: "3", Actual: "2", Present: true}}, want: "[assertion]\n  jsonpath \"$.items\" count ==\n     expected 3 => received 2\n"},
		{name: "json absent", entry: runner.EntryResult{JSONFailure: &runner.JSONFailure{Position: pos, Query: "$.item", Operator: "==", Expected: "1"}}, want: "[assertion]\n  jsonpath \"$.item\" ==\n     expected 1 => received ausente\n"},
		{name: "json concealed absence", entry: runner.EntryResult{JSONFailure: &runner.JSONFailure{Position: pos, Query: "$.item", Operator: "==", Expected: "[oculto]", Actual: "[oculto]"}}, want: "[assertion]\n  jsonpath \"$.item\" ==\n     expected [oculto] => received [oculto] (ausente)\n"},
		{name: "type predicate", entry: runner.EntryResult{JSONFailure: &runner.JSONFailure{Position: pos, Query: "$.item", Operator: "isString", Actual: "tipo number", Present: true}}, want: "[assertion]\n  jsonpath \"$.item\" isString\n     received tipo number; condição falsa\n"},
		{name: "json reason", entry: runner.EntryResult{JSONFailure: &runner.JSONFailure{Position: pos, Query: "$.item", Operator: "matches", Expected: `"pattern"`, Actual: "tipo array", Present: true, Detail: "matches exige string"}}, want: "[assertion]\n  jsonpath \"$.item\" matches\n     expected \"pattern\" => received tipo array; matches exige string\n"},
		{name: "header", entry: runner.EntryResult{HeaderFailure: &runner.HeaderFailure{Position: pos, Name: "X-Test", Operator: "==", Expected: "yes", Actual: "no", Present: true}}, want: "[assertion]\n  header \"X-Test\" ==\n     expected \"yes\" => received \"no\"\n"},
		{name: "header absent", entry: runner.EntryResult{HeaderFailure: &runner.HeaderFailure{Position: pos, Name: "X-Test", Operator: "exists"}}, want: "[assertion]\n  header \"X-Test\" exists\n     expected presente => received ausente\n"},
		{name: "header concealed absence", entry: runner.EntryResult{HeaderFailure: &runner.HeaderFailure{Position: pos, Name: "X-Test", Operator: "==", Expected: "[oculto]", Actual: "[oculto]"}}, want: "[assertion]\n  header \"X-Test\" ==\n     expected \"[oculto]\" => received [oculto] (ausente)\n"},
		{name: "body", entry: runner.EntryResult{BodyFailure: &runner.BodyFailure{Position: pos, Operator: "contains", Expected: "safe-literal"}}, want: "[assertion]\n  body contains\n     expected \"safe-literal\" => condição falsa\n"},
		{name: "regex", entry: runner.EntryResult{RegexFailure: &runner.RegexFailure{Position: pos, Pattern: "id=(.*)", Operator: "==", Expected: "safe-id", Present: true}}, want: "[assertion]\n  regex \"id=(.*)\" ==\n     expected \"safe-id\" => received grupo presente; condição falsa\n"},
		{name: "regex absent", entry: runner.EntryResult{RegexFailure: &runner.RegexFailure{Position: pos, Pattern: "id=(.*)", Operator: "==", Expected: "safe-id"}}, want: "[assertion]\n  regex \"id=(.*)\" ==\n     expected \"safe-id\" => received grupo ausente\n"},
		{name: "status", entry: runner.EntryResult{Entry: parser.Entry{ExpectedStatus: &status, ExpectedPosition: pos}, Status: 503}, want: "[status]\n  HTTP 200\n     expected 200 => received 503\n"},
		{name: "capture", err: &runner.CaptureError{Position: pos, Query: "$.items", Detail: "consulta não encontrou valor"}, want: "[captures]\n  jsonpath \"$.items\"\n     consulta não encontrou valor\n"},
		{name: "count capture", err: &runner.CaptureError{Position: pos, Query: "$.items", Count: true, Detail: "count exige array"}, want: "[captures]\n  jsonpath \"$.items\" count\n     count exige array\n"},
		{name: "capture without expression", err: &runner.CaptureError{Position: pos, Detail: "consulta não encontrou valor"}, want: "[captures]\n     consulta não encontrou valor\n"},
		{name: "json evaluation", err: &runner.JSONError{Position: pos, Query: "$.items", Detail: "consulta exige objeto"}, want: "[assertion]\n  jsonpath \"$.items\"\n     consulta exige objeto\n"},
		{name: "header evaluation", err: &runner.HeaderError{Position: pos, Name: "X-Test", Detail: "esperado textual inválido"}, want: "[assertion]\n  header \"X-Test\"\n     esperado textual inválido\n"},
		{name: "body evaluation", err: &runner.BodyError{Position: pos, Operator: "contains", Detail: "charset desconhecido"}, want: "[assertion]\n  body contains\n     charset desconhecido\n"},
		{name: "regex evaluation", err: &runner.RegexError{Position: pos, Operator: "matches", Detail: "padrão matches inválido"}, want: "[assertion]\n  regex matches\n     padrão matches inválido\n"},
		{name: "render", err: &runner.RenderError{Position: pos, Detail: "variável ausente"}, want: "[renderização]\n     erro de renderização: variável ausente\n"},
		{name: "external file", err: &runner.FileError{Position: pos, Detail: "arquivo ausente"}, want: "[renderização]\n     erro de arquivo externo: arquivo ausente\n"},
		{name: "transport", err: errors.New("raw-private-body-and-credentials"), want: "[transporte]\n     falha de transporte; verifique conexão, TLS e redirecionamentos\n"},
		{name: "cancel", err: context.Canceled, want: "[transporte]\n     requisição cancelada\n"},
		{name: "parse", loadErr: &parser.Error{Position: pos, Category: "sintaxe inválida", Detail: "resposta esperada inválida"}, want: "[parsing]\n     sintaxe inválida: resposta esperada inválida\n"},
		{name: "read", loadErr: &suite.Error{File: pos.File, Detail: "não foi possível abrir o cenário"}, want: "[parsing]\n     não foi possível abrir o cenário\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			c := &Command{err: &output}
			p := &fileProgress{command: c, done: 7}
			run := runner.Result{Entries: []runner.EntryResult{test.entry}, Diverged: test.err == nil, Err: test.err, ErrorPosition: pos}
			want := "  step 7 - Linha 114:1\n  " + test.want
			if test.loadErr != nil {
				want = "  Linha 114:1\n  " + test.want
				if test.name == "read" {
					want = "  " + test.want
				}
			}
			if err := p.diagnostic(run, test.loadErr); err != nil || output.String() != want {
				t.Fatalf("output=%q want=%q err=%v", output.String(), want, err)
			}
		})
	}
}

func TestCompactDiagnosticColorsAndUnknownLocation(t *testing.T) {
	for _, color := range []bool{false, true} {
		t.Run(fmt.Sprint(color), func(t *testing.T) {
			var output bytes.Buffer
			c := &Command{err: &output, color: color}
			p := &fileProgress{command: c}
			if err := p.diagnostic(runner.Result{Err: context.Canceled}, nil); err != nil {
				t.Fatal(err)
			}
			message := "requisição cancelada"
			if color {
				message = "\x1b[31m" + message + "\x1b[0m"
			}
			want := "  [transporte]\n     " + message + "\n"
			if output.String() != want {
				t.Fatalf("output=%q want=%q", output.String(), want)
			}
			output.Reset()
			if err := p.diagnostic(runner.Result{}, nil); err != nil || output.Len() != 0 {
				t.Fatalf("success diagnostic=%q err=%v", output.String(), err)
			}
		})
	}
}

func TestCompactEvaluationExpressionUsesOnlySafeMetadata(t *testing.T) {
	pos := parser.Position{Line: 4, Column: 1}
	for _, json := range []bool{false, true} {
		t.Run(fmt.Sprint(json), func(t *testing.T) {
			declaration := parser.Assertion{Header: &parser.HeaderAssertion{Position: pos, Name: "X", Operator: "matches", Expected: "must-not-appear"}}
			var cause error = &runner.HeaderError{Position: pos, Name: "X", Detail: "esperado textual inválido"}
			expression := `header "X" matches`
			if json {
				declaration = parser.Assertion{JSON: &parser.JSONAssertion{QueryPosition: pos, Query: "$.items", Count: true, Operator: "=="}}
				cause = &runner.JSONError{Position: pos, Query: "$.items", Detail: "count exige array"}
				expression = `jsonpath "$.items" count ==`
			}
			var output bytes.Buffer
			p := &fileProgress{command: &Command{err: &output}, done: 1}
			run := runner.Result{Err: fmt.Errorf("safe wrapper: %w", cause), Entries: []runner.EntryResult{{Entry: parser.Entry{Assertions: []parser.Assertion{declaration}}}}}
			if err := p.diagnostic(run, nil); err != nil || !strings.Contains(output.String(), expression+"\n") || strings.Contains(output.String(), "must-not-appear") {
				t.Fatalf("output=%q err=%v", output.String(), err)
			}
		})
	}
}

// GO-597, GO-598, GO-600: the compact block starts with the failed endpoint.
func TestCompactDiagnosticEndpoint(t *testing.T) {
	pos := parser.Position{Line: 19, Column: 1}
	status := 200
	for _, test := range []struct {
		name     string
		entry    runner.EntryResult
		err      error
		diverged bool
		method   string
		want     string
	}{
		{
			name: "status", diverged: true, method: "POST",
			entry: runner.EntryResult{Entry: parser.Entry{Method: "POST", ExpectedStatus: &status, ExpectedPosition: pos}, Status: 500, DisplayURL: "http://host/catalogo/ativos"},
			want:  "[status]\n  HTTP 200\n     expected 200 => received 500\n",
		},
		{
			name: "json assertion", diverged: true, method: "GET",
			entry: runner.EntryResult{Entry: parser.Entry{Method: "GET"}, JSONFailure: &runner.JSONFailure{Position: pos, Query: "$.items", Operator: "==", Expected: "3", Actual: "2", Present: true}, DisplayURL: "http://host/items"},
			want:  "[assertion]\n  jsonpath \"$.items\" ==\n     expected 3 => received 2\n",
		},
		{
			name: "header assertion", diverged: true, method: "PUT",
			entry: runner.EntryResult{Entry: parser.Entry{Method: "PUT"}, HeaderFailure: &runner.HeaderFailure{Position: pos, Name: "X-Test", Operator: "==", Expected: "yes", Actual: "no", Present: true}, DisplayURL: "http://host/header"},
			want:  "[assertion]\n  header \"X-Test\" ==\n     expected \"yes\" => received \"no\"\n",
		},
		{
			name: "body assertion", diverged: true, method: "PATCH",
			entry: runner.EntryResult{Entry: parser.Entry{Method: "PATCH"}, BodyFailure: &runner.BodyFailure{Position: pos, Operator: "contains", Expected: "safe-literal"}, DisplayURL: "http://host/body"},
			want:  "[assertion]\n  body contains\n     expected \"safe-literal\" => condição falsa\n",
		},
		{
			name: "regex assertion", diverged: true, method: "DELETE",
			entry: runner.EntryResult{Entry: parser.Entry{Method: "DELETE"}, RegexFailure: &runner.RegexFailure{Position: pos, Pattern: "id=(.*)", Operator: "==", Expected: "safe-id", Present: true}, DisplayURL: "http://host/regex"},
			want:  "[assertion]\n  regex \"id=(.*)\" ==\n     expected \"safe-id\" => received grupo presente; condição falsa\n",
		},
		{
			name: "transport", method: "GET", err: errors.New("private transport cause"),
			entry: runner.EntryResult{Entry: parser.Entry{Method: "GET", Position: pos}, Outcome: runner.Failed, DisplayURL: "http://host/transport"},
			want:  "[transporte]\n     falha de transporte; verifique conexão, TLS e redirecionamentos\n",
		},
		{
			name: "render", method: "GET", err: &runner.RenderError{Position: pos, Detail: "variável ausente"},
			entry: runner.EntryResult{Entry: parser.Entry{Method: "GET", Position: pos}, Outcome: runner.RenderFailed, DisplayURL: "[URL não renderizada]"},
			want:  "[renderização]\n     erro de renderização: variável ausente\n",
		},
		{
			name: "external file", method: "POST", err: &runner.FileError{Position: pos, Detail: "arquivo ausente"},
			entry: runner.EntryResult{Entry: parser.Entry{Method: "POST", Position: pos}, Outcome: runner.FileFailed, DisplayURL: "http://host/rendered"},
			want:  "[renderização]\n     erro de arquivo externo: arquivo ausente\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			p := &fileProgress{command: &Command{err: &output}, done: 2}
			run := runner.Result{Entries: []runner.EntryResult{test.entry}, Diverged: test.diverged, Err: test.err, ErrorPosition: pos}
			if err := p.diagnostic(run, nil); err != nil {
				t.Fatal(err)
			}
			url := test.entry.DisplayURL
			if test.err != nil && test.name != "transport" {
				url = "[URL não renderizada]"
			}
			want := "  " + test.method + " " + url + " -> step 2 - Linha 19:1\n  " + test.want
			if output.String() != want {
				t.Fatalf("output=%q want=%q", output.String(), want)
			}
		})
	}
}

// GO-601: a rejected file has no step and no invented endpoint.
func TestCompactDiagnosticLoadFailureHasNoEndpoint(t *testing.T) {
	pos := parser.Position{File: "scenario.httpgo", Line: 3, Column: 1}
	var output bytes.Buffer
	p := &fileProgress{command: &Command{err: &output}, done: 4}
	entry := runner.EntryResult{Entry: parser.Entry{Method: "GET"}, DisplayURL: "http://host/must-not-appear"}
	run := runner.Result{Entries: []runner.EntryResult{entry}, Diverged: true}
	if err := p.diagnostic(run, &suite.Error{File: pos.File, Detail: "não foi possível abrir o cenário"}); err != nil {
		t.Fatal(err)
	}
	want := "  [parsing]\n     não foi possível abrir o cenário\n"
	if output.String() != want {
		t.Fatalf("output=%q want=%q", output.String(), want)
	}
}

// GO-599, GO-602: the endpoint keeps the protected form of the verbose view.
func TestCompactEndpointProtectsURL(t *testing.T) {
	for _, noColor := range []string{"", "1"} {
		t.Run("NO_COLOR="+noColor, func(t *testing.T) {
			t.Setenv("NO_COLOR", noColor)
			const secret = "fiction-endpoint-token"
			file := writeScenario(t, "GET http://example.test/login\n[Captures]\ntoken: jsonpath \"$.token\"\nGET http://user:pass@example.test/{{token}}?q={{token}}\nHTTP 200")
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				if calls == 1 {
					return runner.Response{Status: 200, Body: []byte(`{"token":"` + secret + `"}`)}, nil
				}
				return runner.Response{Status: 500}, nil
			}), BuildInfo{})
			command.isTerminal = func(io.Writer) bool { return true }
			var out, stderr bytes.Buffer
			command.SetOut(&out)
			command.SetErr(&stderr)
			command.SetArgs([]string{"test", file, "--secret", "token"})
			if code := command.ExecuteContext(context.Background()); code != 1 {
				t.Fatalf("code=%d err=%q", code, stderr.String())
			}
			want := "  GET http://example.test/[oculto]?[oculta] -> step 2 - Linha 5:1\n"
			if calls != 2 || !strings.Contains(stderr.String(), want) {
				t.Fatalf("calls=%d err=%q want=%q", calls, stderr.String(), want)
			}
			for _, unsafe := range []string{secret, "user", "pass", "?q="} {
				if strings.Contains(out.String()+stderr.String(), unsafe) {
					t.Fatalf("leaked %q: %q", unsafe, out.String()+stderr.String())
				}
			}
		})
	}
}

// GO-604: an endpoint write failure stops the suite without reusing the writer.
func TestCompactEndpointWriteFailureStopsSuite(t *testing.T) {
	for _, short := range []bool{false, true} {
		t.Run(fmt.Sprintf("short=%v", short), func(t *testing.T) {
			dir := t.TempDir()
			suiteFile(t, dir, "01.httpgo", "GET http://example.test/first\nHTTP 200")
			suiteFile(t, dir, "02.httpgo", "GET http://example.test/never\nHTTP 200")
			calls := 0
			command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
				calls++
				return runner.Response{Status: 503}, nil
			}), BuildInfo{})
			command.isTerminal = func(io.Writer) bool { return true }
			// Loading, total, step and final line succeed; the endpoint block fails.
			writer := &suiteWriter{allowed: 4, short: short}
			command.SetOut(io.Discard)
			command.SetErr(writer)
			command.SetArgs([]string{"test", dir})
			if code := command.ExecuteContext(context.Background()); code != 2 || calls != 1 || writer.calls != 5 {
				t.Fatalf("code=%d calls=%d writes=%d err=%q", code, calls, writer.calls, writer.data.String())
			}
			if strings.Count(writer.data.String(), "step 1 - Linha") > 1 {
				t.Fatalf("repeated diagnostic: %q", writer.data.String())
			}
		})
	}
}
