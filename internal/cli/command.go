// Package cli owns argument handling and safe, plain-text presentation.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
	"capimgrosso.com/httpgo/internal/transport"
)

type BuildInfo struct{ Version, Commit, Date string }

type Command struct {
	executor runner.HTTPExecutor
	build    BuildInfo
	args     []string
	in       io.Reader
	out, err io.Writer
}

func NewRootCommand(executor runner.HTTPExecutor, build BuildInfo) *Command {
	return &Command{executor: executor, build: build, in: strings.NewReader(""), out: io.Discard, err: io.Discard}
}

func (c *Command) SetArgs(args []string) { c.args = args }
func (c *Command) SetIn(in io.Reader)    { c.in = in }
func (c *Command) SetOut(out io.Writer)  { c.out = out }
func (c *Command) SetErr(err io.Writer)  { c.err = err }

func (c *Command) ExecuteContext(ctx context.Context) int {
	local := *c
	protect := argumentProtection(c.args)
	local.out = protectedWriter{c.out, protect}
	local.err = protectedWriter{c.err, protect}
	return local.executeContext(ctx)
}

func (c *Command) executeContext(ctx context.Context) int {
	if len(c.args) == 1 && c.args[0] == "--version" {
		return c.writeText(fmt.Sprintf("httpgo %s commit=%s date=%s\n", c.build.Version, c.build.Commit, c.build.Date))
	}
	if (len(c.args) == 1 && c.args[0] == "--help") || (len(c.args) == 2 && c.args[0] == "test" && c.args[1] == "--help") {
		return c.writeText(help())
	}
	args, err := parseArguments(c.args)
	if err != nil {
		return c.fail(err.Error())
	}
	renderer, err := template.New(args.variables, args.secrets)
	if err != nil {
		return c.fail(err.Error())
	}
	file := args.file
	input, err := os.Open(file)
	if err != nil {
		return c.fail(fmt.Sprintf("%q: não foi possível abrir o arquivo; verifique o caminho e a permissão de leitura", file))
	}
	info, statErr := input.Stat()
	if statErr != nil || !info.Mode().IsRegular() {
		_ = input.Close()
		return c.fail(fmt.Sprintf("%q: entrada deve ser um arquivo regular legível", file))
	}
	scenario, parseErr := parser.Parse(file, input)
	closeErr := input.Close()
	if parseErr != nil {
		return c.fail(parseErr.Error())
	}
	if closeErr != nil {
		return c.fail(fmt.Sprintf("%q: não foi possível fechar o arquivo", file))
	}
	result := runner.NewWithRenderer(c.executor, renderer, c.renderEntry).Run(ctx, scenario)
	return c.renderSummary(scenario, result)
}

func (c *Command) renderEntry(entry runner.EntryResult) error {
	shownURL := displayURL(entry.DisplayURL)
	if entry.Outcome == runner.RenderFailed {
		shownURL = "[URL não renderizada]"
	}
	line := fmt.Sprintf("%s %s: %s", entry.Entry.Method, shownURL, entry.Outcome)
	switch entry.Outcome {
	case runner.Passed, runner.Mismatch:
		line += fmt.Sprintf(" (esperado %d, recebido %d)", *entry.Entry.ExpectedStatus, entry.Status)
	case runner.Unvalidated:
		line += fmt.Sprintf(" (recebido %d)", entry.Status)
	case runner.Failed, runner.RenderFailed:
		// Headers alone are not a completed response: never invent a status.
	default:
		return fmt.Errorf("resultado de execução desconhecido")
	}
	if entry.Outcome == runner.Mismatch || entry.Outcome == runner.Failed || entry.Outcome == runner.RenderFailed {
		pos := entry.Entry.Position
		line = fmt.Sprintf("%q:%d:%d: %s", pos.File, pos.Line, pos.Column, line)
	}
	if _, err := io.WriteString(c.out, line+"\n"); err != nil {
		return fmt.Errorf("escrever resultado da requisição: %w", err)
	}
	return nil
}

func (c *Command) renderSummary(scenario parser.Scenario, result runner.Result) int {
	if result.ReportErr != nil {
		pos := result.ErrorPosition
		return c.fail(fmt.Sprintf("%q:%d:%d: não foi possível escrever o resultado; execução interrompida", pos.File, pos.Line, pos.Column))
	}
	s := result.Summary
	if c.writeText(fmt.Sprintf("resumo %q: total=%d iniciadas=%d concluídas=%d validadas=%d sem_validação=%d não_iniciadas=%d\n", scenario.File, s.Total, s.Started, s.Completed, s.Validated, s.WithoutValidation, s.NotStarted)) != 0 {
		return 2
	}
	if result.Err != nil {
		pos := result.ErrorPosition
		diagnostic := transportDiagnostic(result.Err)
		var renderErr *runner.RenderError
		if errors.As(result.Err, &renderErr) {
			diagnostic = renderErr.Error()
		}
		return c.fail(fmt.Sprintf("%q:%d:%d: %s; execução interrompida", pos.File, pos.Line, pos.Column, diagnostic))
	}
	if result.Diverged {
		entry := result.Entries[len(result.Entries)-1]
		pos := entry.Entry.ExpectedPosition
		if _, err := fmt.Fprintf(c.err, "%q:%d:%d: status divergente: esperado %d, recebido %d; execução interrompida\n", pos.File, pos.Line, pos.Column, *entry.Entry.ExpectedStatus, entry.Status); err != nil {
			return 2
		}
		return 1
	}
	return 0
}

func displayURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[URL inválida]"
	}
	hasQuery := u.RawQuery != "" || u.ForceQuery
	u.User, u.RawQuery, u.Fragment, u.RawFragment = nil, "", "", ""
	u.ForceQuery = false
	text := u.String()
	if hasQuery {
		text += "?[oculta]"
	}
	return text
}

func transportDiagnostic(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "requisição cancelada"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout da requisição (limite total de 10 segundos)"
	default:
		var safe *transport.Error
		if errors.As(err, &safe) {
			return safe.Error()
		}
		return "falha de transporte; verifique conexão, TLS e redirecionamentos"
	}
}

func (c *Command) writeText(text string) int {
	if _, err := io.WriteString(c.out, text); err != nil {
		return c.fail("não foi possível escrever o resultado")
	}
	return 0
}

func (c *Command) fail(message string) int {
	_, _ = fmt.Fprintln(c.err, message)
	return 2
}

func help() string {
	return `httpgo — executa requisições HTTP e valida status

Uso:
  httpgo test <arquivo> [--var nome=valor] [--secret nome]
  httpgo --help
  httpgo test --help
  httpgo --version

Exemplo: salve health.hurl e execute httpgo test health.hurl
  GET http://localhost:8080/health
  HTTP 200

  GET http://localhost:8080/inexistente
  HTTP 404

Exemplo JSON: salve products.hurl e execute httpgo test products.hurl
  POST http://localhost:8080/products
  Content-Type: application/json
  {"name":"Pizza","price":4500}

  HTTP 201

Métodos: GET, POST, PUT, PATCH e DELETE. GET não aceita body.
Headers e um valor JSON inline são opcionais; bytes do JSON são preservados.
Exatamente um arquivo; parsing completo antes da rede. Sem HTTP, qualquer
status é aceito e informado como sem validação. A primeira falha interrompe.
Cada resultado é exibido ao terminar a requisição, antes da próxima; resumo ao final.
Timeout total: 10 segundos por requisição. Até 10 redirects na mesma origem.
301/302/303 convertem os novos métodos em GET sem body; 307/308 preservam ambos.
Códigos: 0 sucesso, 1 status divergente, 2 entrada/renderização/transporte/cancelamento/saída.
Saída sempre em texto sem cores/animações, inclusive em TTY e com NO_COLOR.
Credenciais, query inteira e fragmento da URL não são exibidos.
Valores de headers e bodies não são impressos, nem em erros de entrada.

Variáveis textuais (flags repetíveis):
  httpgo test products.hurl --var api.baseUrl=http://localhost:8080 --var token=ficticio --secret token
  GET {{ api.baseUrl }}/products
  Authorization: Bearer {{token}}
  HTTP 200
--var nome=valor: somente texto; vazio e sinais = adicionais são preservados.
Nomes ASCII com segmentos separados por ponto; duplicação/conflito é erro.
URL e valores de headers aceitam placeholders, sem recursão ou encoding automático.
--secret nome: protege uma variável, sem fornecer valor; pode ainda estar ausente.
Proibido fornecer credenciais reais em argumentos, inclusive --var: use apenas
valores não sigilosos ou fictícios. A proteção não cobre histórico/processos.
Ambiente do processo, stdin, capturas e templates no body não estão disponíveis.
Atenção: o path não protegido permanece visível; não coloque segredos arbitrários nele.
`
}
