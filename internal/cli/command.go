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
	executor   runner.HTTPExecutor
	build      BuildInfo
	args       []string
	in         io.Reader
	out, err   io.Writer
	protection *executionProtection
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
	local.protection = &executionProtection{initial: argumentProtection(c.args)}
	local.out = protectedWriter{c.out, local.protection.sanitize}
	local.err = protectedWriter{c.err, local.protection.sanitize}
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
	result := runner.NewWithRenderer(c.executor, sessionRenderer{renderer, c.protection}, c.renderEntry).Run(ctx, scenario)
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
	case runner.Unvalidated, runner.Wildcard:
		line += fmt.Sprintf(" (recebido %d)", entry.Status)
	case runner.HeaderMismatch:
		line += ": " + headerDiagnostic(entry.HeaderFailure)
	case runner.JSONMismatch:
		line += ": " + jsonDiagnostic(entry.JSONFailure)
	case runner.JSONFailed:
		line += fmt.Sprintf(": consulta %q: %s", entry.JSONError.Query, entry.JSONError.Detail)
	case runner.CaptureFailed:
		if entry.Entry.ExpectedStatus != nil {
			line += fmt.Sprintf(" (status validado: esperado %d, recebido %d)", *entry.Entry.ExpectedStatus, entry.Status)
		} else if entry.Entry.AnyStatus {
			line += fmt.Sprintf(" (HTTP *, recebido %d)", entry.Status)
		} else {
			line += fmt.Sprintf(" (sem validação de status, recebido %d)", entry.Status)
		}
	case runner.Failed, runner.RenderFailed:
		// Headers alone are not a completed response: never invent a status.
	default:
		return fmt.Errorf("resultado de execução desconhecido")
	}
	if entry.Outcome == runner.Mismatch || entry.Outcome == runner.Failed || entry.Outcome == runner.RenderFailed || entry.Outcome == runner.CaptureFailed || entry.Outcome == runner.HeaderMismatch || entry.Outcome == runner.JSONMismatch || entry.Outcome == runner.JSONFailed {
		pos := entry.Entry.Position
		if entry.HeaderFailure != nil {
			pos = entry.HeaderFailure.Position
		}
		if entry.JSONFailure != nil {
			pos = entry.JSONFailure.Position
		}
		if entry.JSONError != nil {
			pos = entry.JSONError.Position
		}
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
		var captureErr *runner.CaptureError
		if errors.As(result.Err, &captureErr) {
			diagnostic = captureErr.Error()
		}
		var jsonErr *runner.JSONError
		if errors.As(result.Err, &jsonErr) {
			diagnostic = jsonErr.Error()
		}
		return c.fail(fmt.Sprintf("%q:%d:%d: %s; execução interrompida", pos.File, pos.Line, pos.Column, diagnostic))
	}
	if result.Diverged {
		entry := result.Entries[len(result.Entries)-1]
		if entry.HeaderFailure != nil {
			failure := entry.HeaderFailure
			pos := failure.Position
			if _, err := fmt.Fprintf(c.err, "%q:%d:%d: %s; execução interrompida\n", pos.File, pos.Line, pos.Column, headerDiagnostic(failure)); err != nil {
				return 2
			}
			return 1
		}
		if entry.JSONFailure != nil {
			failure := entry.JSONFailure
			pos := failure.Position
			if _, err := fmt.Fprintf(c.err, "%q:%d:%d: %s; execução interrompida\n", pos.File, pos.Line, pos.Column, jsonDiagnostic(failure)); err != nil {
				return 2
			}
			return 1
		}
		pos := entry.Entry.ExpectedPosition
		if _, err := fmt.Fprintf(c.err, "%q:%d:%d: status divergente: esperado %d, recebido %d; execução interrompida\n", pos.File, pos.Line, pos.Column, *entry.Entry.ExpectedStatus, entry.Status); err != nil {
			return 2
		}
		return 1
	}
	return 0
}

func jsonDiagnostic(failure *runner.JSONFailure) string {
	actual := "ausente"
	if failure.Present {
		actual = failure.Actual
	} else if failure.Actual == "[oculto]" {
		actual = "[oculto] (ausente)"
	}
	return fmt.Sprintf("assertion jsonpath %q ==: esperado %s, recebido %s", failure.Query, failure.Expected, actual)
}

func headerDiagnostic(failure *runner.HeaderFailure) string {
	actual := "ausente"
	if failure.Present {
		actual = fmt.Sprintf("%q", failure.Actual)
	} else if failure.Actual == "[oculto]" {
		actual = "[oculto] (ausente)"
	}
	expected := fmt.Sprintf("%q", failure.Expected)
	if failure.Operator == "exists" && failure.Expected != "[oculto]" {
		expected = "presente"
	}
	return fmt.Sprintf("assertion header %q %s: esperado %s, recebido %s", failure.Name, failure.Operator, expected, actual)
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
	return `httpgo — executa requisições HTTP e valida status e headers

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
Headers e um valor JSON inline são opcionais; JSON estático preserva seus bytes.
Exatamente um arquivo; parsing completo antes da rede. Sem HTTP, qualquer
status é aceito e informado como sem validação. HTTP * aceita qualquer status,
inclusive 404/503, sem comparação numérica; aparece como status aceito por HTTP *
e conta em sem_validação, não em validadas. A primeira falha interrompe.
Cada resultado é exibido ao terminar a requisição, antes da próxima; resumo ao final.
Timeout total: 10 segundos por requisição. Até 10 redirects na mesma origem.
301/302/303 convertem os novos métodos em GET sem body; 307/308 preservam ambos.
Códigos: 0 sucesso, 1 status ou assertion de header divergente,
2 entrada/captura/renderização/transporte/cancelamento/saída.
Saída sempre em texto sem cores/animações, inclusive em TTY e com NO_COLOR.
Credenciais, query inteira e fragmento da URL não são exibidos.
Bodies e coleções de headers não são impressos, nem em erros de entrada.
Valores comuns sanitizados podem aparecer no diagnóstico de assertion.

Assertions de headers: salve health.hurl e execute httpgo test health.hurl
(requer resposta com Content-Type application/json e X-Request-Id req-123):
  GET http://localhost:8080/health
  HTTP 200
  [Asserts]
  header "Content-Type" contains "application/json"
  header "X-Request-Id" exists
  header "X-Request-Id" == "req-123"
[Asserts] exige HTTP numérico ou HTTP *. Nomes ignoram caixa; valores não.
Somente primeira ocorrência; não concatena nem separa vírgulas.
Ausente falha nos três operadores; vazio existe e satisfaz == "".
contains "" passa para qualquer valor presente, inclusive vazio.
Esperados são strings literais: {{nome}} não interpola, mesmo se definido.
Sem regex, filtros, captura de headers, assertions JSON ou outros operadores.
Ordem: resposta consumida/fechada, status, headers, capturas, resultado.
Falha de header retorna 1, não publica capturas e impede próximo envio.
Diagnóstico localizado distingue ausente de ""; valores de Authorization,
Proxy-Authorization, Cookie e Set-Cookie são ocultos nos dois lados.
Segredos conhecidos também são removidos de headers comuns.
Diferenças Hurl 8.0.0: repetidos usam primeira ocorrência, não lista;
esperados são literais e assertions antecedem capturas. Veja docs/compatibility.md.

Variáveis tipadas (flags repetíveis; extensão de tipagem HTTPGo):
  httpgo test products.hurl --var api.baseUrl=http://localhost:8080 --var token=ficticio --secret token
  GET {{ api.baseUrl }}/products
  Authorization: Bearer {{token}}
  HTTP 200
--var nome=valor: literal JSON válido preserva tipo; caso contrário é texto,
inclusive vazio. Sinais = após o primeiro são preservados. Para forçar string:
  --var 'product.code="001"'
  httpgo test products.hurl --var api.baseUrl=http://localhost:8080 --var product.name=Pizza --var product.price=4500 --var product.active=true --var 'product.code="001"'
  POST {{api.baseUrl}}/products
  Content-Type: application/json
  {"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}},"code":{{product.code}}}
  HTTP 201
Nomes ASCII pontuados são exatos: objetos são opacos, sem acesso a propriedades.
Duplicação e ancestral/descendente são erros, sem merge nem sobrescrita.
Valores JSON completos (inclusive raiz) aceitam qualquer tipo. Dentro de strings
JSON, URL e headers, somente escalares; strings JSON são escapadas corretamente.
Chaves JSON e nomes de headers são estáticos. Sem recursão ou encoding de URL automático.
Capturas tipadas da resposta para entradas seguintes (URL, headers e JSON inline):
  POST {{base_url}}/auth/login
  {"username":"{{username}}","password":"{{password}}"}
  HTTP 200
  [Captures]
  accessToken: jsonpath "$.accessToken"
  GET {{base_url}}/products
  Authorization: Bearer {{accessToken}}
  HTTP 200
  httpgo test products.hurl --var base_url=http://localhost:8080 --var username=demo --var password=senha-ficticia --secret password --secret accessToken
JSONPath limitado: raiz $, propriedades por ponto e índices não negativos,
como $.product.id e $.items[0]. Aceita string, número, booleano, null, array ou objeto,
inclusive raiz e vazios. Números mantêm precisão, também nas estruturas.
  POST {{base_url}}/products
  Content-Type: application/json
  {"name":"Pizza"}
  HTTP 201
  [Captures]
  productId: jsonpath "$.id"
  GET {{base_url}}/products/{{productId}}
  HTTP 200
  POST {{base_url}}/echo
  Content-Type: application/json
  {"productId":{{productId}}}
  HTTP 200
  httpgo test products.hurl --var base_url=http://localhost:8080
Um id recebido como 9007199254740993 chega exato ao path e como número no JSON.
Strings como "null" ou "{{literal}}" permanecem strings, sem reinterpretação.
Array/objeto exige valor JSON completo: URL, header ou string JSON gera erro antes do envio.
JSON inválido, campo ausente ou índice fora do array geram erro de captura;
null é valor existente e válido. Objetos não criam variáveis para suas propriedades.
Status ou header divergente impede capturas e próximos envios. Capturas não sobrescrevem
variáveis iniciais nem capturas anteriores; cada execução começa sem capturas.
--secret nome: protege variável ou captura futura, sem fornecer valor nem resolver ausência.
Proibido fornecer credenciais reais em argumentos, inclusive --var: use apenas
valores não sigilosos ou fictícios. A proteção não cobre histórico/processos.
Ambiente do processo, stdin e templates de arquivos externos indisponíveis.
Proteção cobre valores brutos/decodificados e escapes conhecidos, não codificações arbitrárias.
Atenção: o path não protegido permanece visível; não coloque segredos arbitrários nele.
`
}
