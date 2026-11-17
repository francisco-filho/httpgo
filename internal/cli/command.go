// Package cli owns argument handling and safe, plain-text presentation.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"capimgrosso.com/httpgo/internal/config"
	"capimgrosso.com/httpgo/internal/report"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/suite"
	"capimgrosso.com/httpgo/internal/transport"
)

type BuildInfo struct{ Version, Commit, Date string }

type Command struct {
	executor   runner.HTTPExecutor
	openReport func(string) (io.WriteCloser, error)
	reportNow  func() time.Time
	build      BuildInfo
	args       []string
	in         io.Reader
	out, err   io.Writer
	protection *executionProtection
	timeout    time.Duration
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
	var outFailure, errFailure error
	local.out = protectedWriter{writer: c.out, protect: local.protection.sanitize, failure: &outFailure}
	local.err = protectedWriter{writer: c.err, protect: local.protection.sanitize, failure: &errFailure}
	return local.executeContext(ctx)
}

func (c *Command) executeContext(ctx context.Context) int {
	if len(c.args) == 1 && c.args[0] == "--version" {
		return c.writeText(fmt.Sprintf("httpgo %s commit=%s date=%s\n", c.build.Version, c.build.Commit, c.build.Date))
	}
	if (len(c.args) == 1 && c.args[0] == "--help") || (len(c.args) == 2 && c.args[0] == "test" && c.args[1] == "--help") {
		return c.writeText(help())
	}
	args, argumentErr := parseArguments(c.args)
	settings, loadErr := config.LoadEnvironment(ctx, args.config, args.environment)
	settings, resolveErr := settings.Resolve(ctx, args.variables)
	initial := c.protection.initial
	configured := settings.Protection(args.variables, args.secrets)
	c.protection.initial = func(text string) string { return initial(configured(text)) }
	if argumentErr != nil {
		return c.fail(argumentErr.Error())
	}
	if loadErr != nil {
		return c.fail(loadErr.Error())
	}
	if resolveErr != nil {
		return c.fail(resolveErr.Error())
	}
	renderer, err := settings.Compose(args.variables, args.secrets)
	if err != nil {
		return c.fail(err.Error())
	}
	if err := ctx.Err(); err != nil {
		return c.fail("execução cancelada antes da leitura do cenário")
	}
	c.timeout = settings.Timeout
	entryReporter := c.renderEntry
	fileReporter := c.renderFile
	var diagnostics []string
	var htmlDetails []report.HTMLFile
	if args.output == "junit" || args.output == "html" {
		entryReporter = nil
		fileReporter = func(_ suite.FileResult, run runner.Result, loadErr error) error {
			text := c.junitDiagnostic(run, loadErr)
			if args.output == "html" {
				htmlDetails = append(htmlDetails, report.HTMLFile{Diagnostic: text, Entries: c.htmlEntries(run)})
			} else {
				diagnostics = append(diagnostics, text)
			}
			if text == "" {
				return nil
			}
			_, err := io.WriteString(c.err, text)
			return err
		}
	}
	execution := suite.NewWithProtection(runner.NewWithRenderer(c.executor, sessionRenderer{renderer, c.protection}, entryReporter), c.protection.sanitize)
	files, err := execution.Discover(ctx, args.files)
	if err != nil {
		return c.fail(err.Error())
	}
	result := execution.Run(ctx, files, suite.Options{Request: runner.Options{RenderFiles: args.renderFiles, RenderAsserts: args.renderAsserts, Timeout: settings.Timeout}, FailFast: args.failFast}, fileReporter)
	if args.output == "junit" {
		if err := c.writeJUnit(args.outputFile, result, diagnostics); err != nil {
			return c.fail(fmt.Sprintf("não foi possível gravar relatório JUnit em %q; verifique o destino e as permissões de escrita", args.outputFile))
		}
	}
	var htmlPath string
	if args.output == "html" {
		var err error
		htmlPath, err = c.writeHTML(args.outputFile, result, htmlDetails)
		if err != nil {
			return c.fail(fmt.Sprintf("não foi possível gravar relatório HTML em %q; verifique o destino e as permissões de escrita", args.outputFile))
		}
	}
	if result.ReportErr != nil {
		pos := result.ReportPosition
		return c.fail(fmt.Sprintf("%q:%d:%d: não foi possível escrever o resultado; execução interrompida", pos.File, pos.Line, pos.Column))
	}
	if result.Err != nil {
		if _, err := io.WriteString(c.err, "execução da suíte cancelada; arquivos restantes não executados\n"); err != nil {
			return 2
		}
	}
	if args.output == "html" {
		if _, err := fmt.Fprintf(c.err, "relatório HTML gerado em %q\n", htmlPath); err != nil {
			return 2
		}
	}
	if args.output == "junit" || args.output == "html" {
		return result.Code
	}
	if err := c.renderSuite(result.Summary); err != nil {
		return c.fail("não foi possível escrever o resumo da suíte")
	}
	return result.Code
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
	case runner.HeaderFailed:
		line += ": " + entry.HeaderError.Error()
	case runner.HeaderMismatch:
		line += ": " + headerDiagnostic(entry.HeaderFailure)
	case runner.BodyMismatch:
		line += ": " + bodyDiagnostic(entry.BodyFailure)
	case runner.BodyFailed:
		line += fmt.Sprintf(": %s: %s", entry.BodyError.Operator, entry.BodyError.Detail)
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
	case runner.Failed, runner.RenderFailed, runner.FileFailed, runner.Cancelled:
		// Headers alone are not a completed response: never invent a status.
	default:
		return fmt.Errorf("resultado de execução desconhecido")
	}
	if entry.Outcome == runner.Cancelled || entry.Outcome == runner.FileFailed || entry.Outcome == runner.Mismatch || entry.Outcome == runner.Failed || entry.Outcome == runner.RenderFailed || entry.Outcome == runner.CaptureFailed || entry.Outcome == runner.HeaderFailed || entry.Outcome == runner.HeaderMismatch || entry.Outcome == runner.JSONMismatch || entry.Outcome == runner.JSONFailed || entry.Outcome == runner.BodyMismatch || entry.Outcome == runner.BodyFailed {
		pos := entry.Entry.Position
		if entry.HeaderError != nil {
			pos = entry.HeaderError.Position
		}
		if entry.HeaderFailure != nil {
			pos = entry.HeaderFailure.Position
		}
		if entry.JSONFailure != nil {
			pos = entry.JSONFailure.Position
		}
		if entry.JSONError != nil {
			pos = entry.JSONError.Position
		}
		if entry.BodyFailure != nil {
			pos = entry.BodyFailure.Position
		}
		if entry.BodyError != nil {
			pos = entry.BodyError.Position
		}
		line = fmt.Sprintf("%q:%d:%d: %s", pos.File, pos.Line, pos.Column, line)
	}
	line += fmt.Sprintf(" [tentativas=%d]", entry.Attempts)
	if _, err := io.WriteString(c.out, line+"\n"); err != nil {
		return fmt.Errorf("escrever resultado da requisição: %w", err)
	}
	return nil
}

func (c *Command) renderFile(file suite.FileResult, result runner.Result, loadErr error) error {
	s := file.Requests
	total, notStarted := fmt.Sprint(s.Total), fmt.Sprint(s.NotStarted)
	if !file.Known {
		total, notStarted = "desconhecido", "desconhecido"
	}
	if _, err := fmt.Fprintf(c.out, "resumo %q: total=%s iniciadas=%d concluídas=%d validadas=%d sem_validação=%d não_iniciadas=%s resultado=%s duração=%s tentativas_iniciadas=%d respostas_completas=%d status_comparados=%d\n", file.Name, total, s.Started, s.Completed, s.Validated, s.WithoutValidation, notStarted, file.State, file.Duration, file.Attempts.Started, file.Attempts.Completed, file.Attempts.StatusChecks); err != nil {
		return err
	}
	if loadErr != nil {
		_, err := fmt.Fprintln(c.err, loadErr.Error())
		return err
	}
	return c.renderDiagnostic(result)
}

func (c *Command) renderSuite(s suite.Summary) error {
	_, err := fmt.Fprintf(c.out, "resumo suíte: arquivos=%d processados=%d executados=%d sucessos=%d falhas_assertion=%d erros=%d não_executados=%d requisições_conhecidas=%d arquivos_contagem_desconhecida=%d iniciadas=%d concluídas=%d validadas=%d sem_validação=%d não_iniciadas=%d requisições_sucesso=%d requisições_falha_assertion=%d requisições_erro=%d duração=%s tentativas_iniciadas=%d respostas_completas=%d status_comparados=%d\n", s.Files, s.Processed, s.Executed, s.Passed, s.Failed, s.Errors, s.Skipped, s.Requests.Total, s.UnknownFiles, s.Requests.Started, s.Requests.Completed, s.Requests.Validated, s.Requests.WithoutValidation, s.Requests.NotStarted, s.Successful, s.AssertionFailures, s.RequestErrors, s.Duration, s.Attempts.Started, s.Attempts.Completed, s.Attempts.StatusChecks)
	return err
}

func (c *Command) renderDiagnostic(result runner.Result) error {
	if result.Err != nil {
		pos := result.ErrorPosition
		diagnostic := transportDiagnostic(result.Err, c.timeout)
		var fileErr *runner.FileError
		if errors.As(result.Err, &fileErr) {
			diagnostic = fileErr.Error()
		}
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
		var headerErr *runner.HeaderError
		if errors.As(result.Err, &headerErr) {
			diagnostic = headerErr.Error()
		}
		var bodyErr *runner.BodyError
		if errors.As(result.Err, &bodyErr) {
			diagnostic = bodyErr.Error()
		}
		_, err := fmt.Fprintf(c.err, "%q:%d:%d: %s; execução interrompida\n", pos.File, pos.Line, pos.Column, diagnostic)
		return err
	}
	if result.Diverged {
		entry := result.Entries[len(result.Entries)-1]
		if entry.HeaderFailure != nil {
			failure := entry.HeaderFailure
			pos := failure.Position
			_, err := fmt.Fprintf(c.err, "%q:%d:%d: %s; execução interrompida\n", pos.File, pos.Line, pos.Column, headerDiagnostic(failure))
			return err
		}
		if entry.JSONFailure != nil {
			failure := entry.JSONFailure
			pos := failure.Position
			_, err := fmt.Fprintf(c.err, "%q:%d:%d: %s; execução interrompida\n", pos.File, pos.Line, pos.Column, jsonDiagnostic(failure))
			return err
		}
		if entry.BodyFailure != nil {
			failure := entry.BodyFailure
			pos := failure.Position
			_, err := fmt.Fprintf(c.err, "%q:%d:%d: %s; execução interrompida\n", pos.File, pos.Line, pos.Column, bodyDiagnostic(failure))
			return err
		}
		pos := entry.Entry.ExpectedPosition
		_, err := fmt.Fprintf(c.err, "%q:%d:%d: status divergente: esperado %d, recebido %d; execução interrompida\n", pos.File, pos.Line, pos.Column, *entry.Entry.ExpectedStatus, entry.Status)
		return err
	}
	return nil
}

func bodyDiagnostic(failure *runner.BodyFailure) string {
	return fmt.Sprintf("assertion body %s: esperado %q; condição falsa", failure.Operator, failure.Expected)
}

func jsonDiagnostic(failure *runner.JSONFailure) string {
	actual := "ausente"
	if failure.Present {
		actual = failure.Actual
	} else if failure.Actual == "[oculto]" {
		actual = "[oculto] (ausente)"
	}
	operator := string(failure.Operator)
	if failure.Count {
		operator = "count " + operator
	}
	diagnostic := fmt.Sprintf("assertion jsonpath %q %s: esperado %s, recebido %s", failure.Query, operator, failure.Expected, actual)
	if failure.Operator.TypePredicate() {
		diagnostic = fmt.Sprintf("assertion jsonpath %q %s: recebido %s; condição falsa", failure.Query, operator, actual)
	}
	if failure.Detail != "" {
		diagnostic += "; " + failure.Detail
	}
	return diagnostic
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

func transportDiagnostic(err error, timeout time.Duration) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "requisição cancelada"
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Sprintf("timeout da requisição (limite total de %g segundos)", timeout.Seconds())
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
	return `httpgo — executa requisições HTTP e valida status, headers, JSON e body

Uso:
  httpgo test <caminho>... [--config arquivo] [--env nome] [--fail-fast] [--output junit|html --output-file caminho] [--render-files] [--render-asserts] [--var nome=valor] [--secret nome]
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

Polling por entrada: salve processing.hurl e execute httpgo test processing.hurl
  GET http://localhost:8080/jobs/123
  [Options]
  retry: 3
  retry-interval: 500
  HTTP 200
  [Asserts]
  jsonpath "$.status" == "READY"
retry: novas tentativas após a inicial (default 0); retry-interval: milissegundos
inteiros não negativos (default 1000; zero sem espera deliberada). Sem ilimitado,
opções duplicadas ou outras opções: erro estático 2 antes de todo HTTP do arquivo.
Somente divergência de status/assertion (1) permite repetir; erros de avaliação,
transporte, captura, renderização, cancelamento ou escrita (2) não são retentados.
Repete a requisição completa, inclusive POST: pode repetir efeitos, sem idempotência.
Variáveis/caminhos iguais; arquivos externos/multipart relidos/reabertos a cada
tentativa, sem snapshot se o arquivo mudar. Capturas publicadas só após aprovação;
segredos descobertos continuam protegidos. Intervalo cancelável entre tentativas;
timeout HTTP por tentativa, sem timeout global, backoff ou retry global.
Um resultado terminal por entrada mostra tentativas=N; contadores legados são
lógicos, métricas tentativas_iniciadas/respostas_completas/status_comparados são reais.
Esgotamento retorna 1 e impede a próxima entrada; cancelamento retorna 2.

Headers e um body são opcionais: JSON inline, file,<caminho>; ou multipart (exclusivos).
JSON inline estático preserva seus bytes. Sem --render-files, arquivo externo
preserva todos os bytes, inclusive vazio, binário e {{placeholder}} literal,
sem validar JSON nem renderizar.
Exemplo: salve tests/products.hurl e execute httpgo test tests/products.hurl
  POST http://localhost:8080/products
  Content-Type: application/json
  file,../payloads/product.json;
  HTTP 201
Caminho literal entre file, e ; final, incluindo espaços, sem aspas/escapes especiais
nem interpolação. Vírgula/ponto e vírgula no nome não são aceitos.
Relativos partem do diretório do .hurl, não do diretório corrente; absolutos aceitos.
Sem sandbox: .. e absolutos podem ler fora do projeto, com permissões do usuário.
Leitura completa e fechamento só na vez da entrada, antes do envio; falha localizada
retorna 2 sem enviar a entrada nem as seguintes. As anteriores não são desfeitas.
No body externo completo, Content-Type não é inferido. Sem --render-files, bytes literais.
--render-files: extensão HTTPGo; trata bodies externos completos como templates JSON,
com as mesmas variáveis/tipos do JSON inline. Não renderiza arquivos multipart.
Independe de extensão/Content-Type; JSON estático preserva inclusive whitespace externo.
No payload: {"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}}}
Exemplo: httpgo test tests/products.hurl --render-files --var product.name=Pizza --var product.price=45 --var product.active=true
Flag sem valor, antes/depois do arquivo após test, somente na execução atual.
Conteúdo validado somente na vez da requisição: vazio/não JSON, template malformado,
variável ausente, tipo incompatível ou JSON final inválido retornam 2 antes do envio.
Diagnóstico aponta payload/linha/coluna originais e referência no cenário,
sem conteúdo bruto. Anteriores permanecem; afetada e seguintes não são enviadas.
Não promete renderização equivalente pelo Hurl.
O prazo do transporte não interrompe uma leitura local bloqueada pelo sistema.

Upload multipart: salve tests/documents.hurl, crie fixtures/document.pdf e execute
  httpgo test tests/documents.hurl --var base_url=http://localhost:8080 --var category=reports --var token=ficticio --secret token
  POST {{base_url}}/documents
  Authorization: Bearer {{token}}
  [MultipartFormData]
  description: Documento de teste
  category: {{category}}
  document: file,../fixtures/document.pdf; application/pdf
  HTTP 201
[MultipartFormData]: POST, PUT ou PATCH, exatamente um arquivo e zero ou mais textos.
Nomes estáticos únicos; não combina com JSON ou outro body. Só valores textuais
interpolam, sem converter arrays/objetos ou interpretar JSON; segredos protegidos.
Arquivo literal nunca é renderizado; caminhos seguem as regras acima, sem sandbox.
Filename é somente basename, preservando espaços/Unicode. MIME explícito prevalece;
sem MIME: tabela determinística por extensão (docs/subset.md), fallback application/octet-stream.
Boundary e Content-Type são automáticos; header opcional multipart/form-data sem boundary manual.
Boundary manual, tipo incompatível, metadados com controles e estrutura inválida: erro 2.
Estrutura inválida impede todos os envios; falha de variável/arquivo na entrada
impede a afetada e as seguintes, sem desfazer anteriores nem contar envio inexistente.
Upload incremental, sem buffer integral nem goroutine produtora; fonte fechada antes
do resultado, inclusive em falha/cancelamento. Não imprime arquivo ou campos brutos.
413 esperado passa; divergência de status retorna 1. Sem múltiplos arquivos,
campos repetidos, filename customizado, headers de parte ou multipart aninhado.
Diferenças Hurl 8.0.0 (caminhos/filename/restrições): docs/compatibility.md.

Suítes: um ou mais arquivos/diretórios, inclusive misturados e sobrepostos.
  httpgo test tests/ --env local
  httpgo test tests/ --env local --fail-fast
Diretórios descobrem .hurl recursivamente; arquivo explícito independe da extensão.
Normaliza absolutos/relativos, . e ..; elimina duplicações pela identidade do filesystem,
sem forçar minúsculas. Ordem lexicográfica pelo caminho relativo ao cwd, com /;
externos podem ter ../. Entre unidades sem relativo, usa absoluto normalizado com /.
Não segue links simbólicos: ignora descobertos e rejeita explícitos, inclusive ancestrais.
Descoberta completa antes de HTTP; entrada inválida/inacessível, falha de acesso ou
cada diretório explícito sem .hurl elegível retorna 2 e impede todos os envios.
Configuração global única; parsing integral de cada arquivo antes de iniciá-lo.
Arquivos e requisições sequenciais; primeira falha definitiva para as entradas do arquivo.
Sem --fail-fast continua os outros, inclusive após erro de parsing/transporte;
com --fail-fast marca os restantes não executados, sem analisá-los para contagem.
Capturas e cookies não passam entre arquivos; variáveis/opções iniciais permanecem.
Não há cookie jar, paralelismo ou dependências entre arquivos.
JUnit: httpgo test tests/ --output junit --output-file target/test-results.xml
--output junit exige --output-file; cria diretório pai quando possível.
Um testcase por arquivo; assertion divergente=failure, demais erros=error,
não executados=skipped. Nomes seguem os caminhos normalizados da descoberta.
Tempos em segundos por arquivo; nenhum reenvio para gerar o XML.
Resultado vai somente ao arquivo, stdout vazio; diagnósticos seguros em stderr.
Artefato também em falha de assertion (1) ou execução (2); erro de escrita/fechamento
retorna 2, sem anunciar publicação. Configuração/descoberta inválida não cria relatório.
Sem bodies/headers integrais ou envio para serviços externos.
HTML: httpgo test tests/ --fail-fast --output html --output-file target/report.html
--output html exige --output-file antes de qualquer HTTP; também aceita arquivo único.
O destino é um nome-base: gera report-YYYYMMDDTHHMMSSmmmZ.html em UTC,
com -1, -2 etc. na primeira colisão livre, sem substituir relatórios anteriores.
Cria o pai quando possível; informa o caminho final em stderr só após gravar e fechar.
HTML único offline, somente tema claro, tabelas e detalhes por requisição,
com duração lógica (preparação, tentativas e esperas, sem apresentação).
Estados e contagens desconhecidas explícitos; ausência de assertion não é validação.
Nomes/URLs/diagnósticos escapados como texto, sem bodies/headers integrais.
Proteção final cobre também segredos conhecidos em arquivo posterior; stderr já emitido
não pode ser redigido retroativamente. Segredos arbitrários não marcados não são detectados.
Sem reenvios para gerar HTML; códigos 0/1/2 preservados, geração/confirmação falha retorna 2.
Erros globais anteriores à execução não geram relatório; falha de gravação pode deixar
arquivo parcial. Sem HTML e JUnit simultâneos, tema escuro ou flags HTML do Hurl.
Resumo por arquivo preserva contadores e inclui resultado/duração; resumo suíte
agrega arquivos/requisições. total e não_iniciadas desconhecidos sem parsing completo;
zero envios não significa zero requisições planejadas. validadas conta só status,
não sucesso completo; ausência de assertion não é validação. Código 2 prevalece sobre 1.
Cancelamento ou falha de escrita encerra toda a suíte, sem próximos envios ou retry de saída.
Proteção de segredos conhecidos/derivados dura toda a invocação, não só o arquivo.
Sem HTTP, qualquer status é aceito e informado como sem validação. HTTP * aceita qualquer status,
inclusive 404/503, sem comparação numérica; aparece como status aceito por HTTP *
e conta em sem_validação, não em validadas. A primeira falha definitiva interrompe.
Cada resultado é exibido ao terminar a requisição, antes da próxima; resumo ao final.
Timeout total: default de 10 segundos por requisição, configurável em YAML.
Até 10 redirects na mesma origem.
301/302/303 convertem os novos métodos em GET sem body; 307/308 preservam ambos.
Códigos: 0 sucesso, 1 status ou assertion de header/JSON/body divergente,
2 entrada/arquivo externo/avaliação de esperado, JSON, header ou body/captura/renderização/transporte/cancelamento/saída.
Saída sempre em texto sem cores/animações, inclusive em TTY e com NO_COLOR.
Credenciais, query inteira e fragmento da URL não são exibidos.
Bodies e coleções de headers não são impressos, nem em erros de entrada.
Valores comuns sanitizados podem aparecer no diagnóstico de assertion.

Esperados dinâmicos (evolução explícita HTTPGo, opt-in por invocação):
  httpgo test order.hurl --render-asserts --var base_url=http://localhost:8080
  POST {{base_url}}/orders
  HTTP 200
  [Captures]
  order_number: jsonpath "$.number"
  order_name: jsonpath "$.name"
  GET {{base_url}}/orders/{{order_number}}
  HTTP 200
  [Asserts]
  jsonpath "$.number" == {{order_number}}
  jsonpath "$.name" == "{{order_name}}"
Requer POST e GET retornando {"number":9007199254740993,"name":"Pizza"}.
--render-asserts é flag sem valor, válida para todos os arquivos desta invocação.
Sem ela, o perfil literal abaixo não interpola esperados, mesmo definidos/ausentes;
placeholder fora de literal válido continua rejeitado antes da rede.
Com ela, resolve o esperado da assertion alcançada antes dos operandos da consulta.
JSON completo preserva tipo/precisão; dentro de string usa texto escalar escapado.
Assim, == {{number}} difere de == "{{number}}". Count exige inteiro não negativo,
comparação ordenada exige número, pertencimento JSON exige escalar.
Headers/body e padrões matches JSON/header usam texto, sem converter array/objeto.
Exemplos: body contains "{{substring}}"; header "X" matches "^{{pattern}}$".
Regex final usa metacaracteres inseridos sem escape automático, compilada na avaliação;
sem referências continua compilada antes da rede. Substituição nunca é recursiva.
Variável ausente, captura futura, tipo ou regex final inválidos: avaliação 2,
sem confirmação nem próximo envio. Captura própria válida fica disponível (US-34).
Status/assertion anterior divergente continua 1.
Segredos e derivados são protegidos antes de resultados/callbacks.
Não habilita novos operadores, consultas inteiras, nomes de header ou regex de captura
dinâmicos, nem assertion regex de body. Filtros aceitam operandos conforme US-31 abaixo.
Ordem: status → capturas provisórias → assertions na ordem escrita → confirmação.
Perfis e diferenças Hurl 8.0.0: docs/subset.md e docs/compatibility.md.

Desigualdade JSON — US-32 (somente valor presente):
  httpgo test rotation.hurl --render-asserts --secret old_token --secret new_token --var base_url=http://localhost:8080
  POST {{base_url}}/session/login
  HTTP 200
  [Captures]
  old_token: jsonpath "$.accessToken"
  POST {{base_url}}/session/renew
  HTTP 200
  [Asserts]
  jsonpath "$.accessToken" != "{{old_token}}"
Novo token passa com 0; repetição ou ausência falha com 1. null presente é valor.
Mesmo contrato tipado/exato de ==: 1, 1.0 e 1e0 não diferem; tipos diferentes diferem.
Objetos ignoram ordem de chaves; ordem de arrays participa. Coleções não são despejadas.
JSON inválido/percurso incompatível/variável dinâmica ausente retornam 2, nunca sucesso.
Sem --render-asserts, placeholders em strings permanecem literais. HTTP é obrigatório.
--secret marca nomes de capturas sem fornecer valores; marque os tokens e derivados.
Segredo provisório oculta ambos os operandos antes da confirmação do lote.
count !=, header/body != e negação genérica continuam não suportados.
Diferença não garante revogação do token antigo, validade do novo ou outros efeitos;
verifique essas propriedades com requisições independentes. Exemplo completo:
testdata/scenarios/scenario_json_inequality_rotation.hurl (inclui uso de new_token).

Renovar capturas — US-34 (mudança explícita de contrato, sem nova flag):
  httpgo test jobs.hurl --render-asserts --var base_url=http://localhost:8080 --var job_id=inicial
  GET {{base_url}}/jobs/current
  HTTP 200
  [Captures]
  job_id: jsonpath "$.selectedId"
  [Asserts]
  jsonpath "$.jobs[?(@.id=='{{job_id}}')].status" == "READY"
Resposta completa → status → todas as capturas → assertions na ordem escrita → confirmação.
Assertions enxergam capturas da própria resposta, mesmo invertendo as seções.
Renovação de nome exato preserva tipo e precisão; vale só no arquivo, sem alterar configuração.
Consultas de captura usam somente iniciais/confirmados anteriores, nunca outras capturas do lote.
Duplicação no lote e conflito ancestral/descendente: erro 2, sem atualização parcial.
Falha de captura/avaliação, divergência ou cancelamento antes de confirmar descarta o lote.
Polling reenvia com valores confirmados antes da entrada; somente sucesso confirma.
Marcas e valores secretos antigos, novos e derivados continuam protegidos após descarte.
Sem --render-asserts, esperados continuam literais e consultas dinâmicas são rejeitadas.
Não habilita query variable, encadeamento de capturas nem nova política de retry.
Exemplo completo: testdata/scenarios/scenario_capture_renewal.hurl e docs/subset.md.

Captura de contagem — US-33:
  httpgo test unchanged.hurl --render-asserts --var base_url=http://localhost:8080
  GET {{base_url}}/orders
  HTTP 200
  [Captures]
  orders_before: jsonpath "$.items" count
  POST {{base_url}}/orders/preview
  Content-Type: application/json
  {"productId":"p1"}
  HTTP 200
  GET {{base_url}}/orders
  HTTP 200
  [Asserts]
  jsonpath "$.items" count == {{orders_before}}
Quantidade preservada passa com 0; alterada falha com 1, sem imprimir pedidos.
count em captura JSONPath publica número JSON inteiro não negativo, nunca string.
Conta só array final, incluindo null; vazio publica zero. Não conta strings/objetos.
Ausência, null ou tipo não array em captura: erro 2, sem baseline zero fictício.
Wildcard/filtro: zero ausente; um conserva tipo; vários formam array, sem achatamento.
Captura e reutilização em requisição não exigem --render-asserts; esperado dinâmico exige.
Comparadores: ==, >, >=, <, <=. Sem parâmetros, pipelines, aritmética ou count em header.
Ordem status → capturas provisórias → assertions → confirmação atômica (US-34).
Sobrescrita exata e captura própria são permitidas; duplicações no lote não.
--secret orders_before protege quantidade e ambos os operandos na comparação posterior.
Diagnóstico mostra consulta/count/causa/localização, não coleção ou quantidade protegida.
Exemplos: testdata/scenarios/scenario_count_unchanged.hurl e capture_count.hurl.

Assertions de body: salve privacy.hurl e execute
  httpgo test privacy.hurl --var base_url=http://localhost:8080
  GET {{base_url}}/export
  HTTP 200
  [Asserts]
  body contains "customer"
  body not contains "passwordHash"
  body not contains "trackingToken"
Busca substring no texto completo da resposta final consumida/fechada, sem reserializar JSON.
Distingue caixa; não normaliza espaços, quebras de linha ou Unicode.
Todo texto contém "", inclusive body vazio; not contains "" sempre falha.
Charset do primeiro Content-Type; omitido usa UTF-8 estrito. ISO-8859-1 usa índice web.
Charset desconhecido ou sequência inválida: erro de avaliação de body (2), inclusive na negação.
Condição falsa retorna 1. Exige HTTP numérico ou HTTP * e uma única string literal.
Perfil literal (sem --render-asserts): Esperado nunca interpola; sem inspeção de binários.
Regex de body é suportada somente em capturas (US-25), não como assertion.
Status → capturas provisórias → assertions de header/JSON/body na ordem escrita → confirmação.
Nas assertions, texto decodifica na primeira de body, independente do JSON.
Capturas anteriores podem exigir decodificação e falhar antes das assertions.
Falha impede confirmação e próxima requisição; sintaxe inválida tardia impede todo HTTP do arquivo.
Diagnóstico identifica operador/arquivo/linha/coluna, nunca body ou fragmentos recebidos.
Esperado secreto é protegido; captura secreta provisória oculta esperado sem confirmar o lote.
Busca literal não garante detectar codificações alternativas de dados sensíveis.

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
Ausente falha em ==, contains e exists; vazio existe e satisfaz == "".
header "Set-Cookie" not contains "Domain" aprova ausência ou primeira ocorrência sem trecho.
Header presente com not contains "" falha, inclusive vazio.
contains "" passa para qualquer valor presente, inclusive vazio.
Sem --render-asserts, esperados são strings literais: {{nome}} não interpola, mesmo se definido.
Regex de headers: salve session.hurl e execute
  httpgo test session.hurl --var base_url=http://localhost:8080
(requer um cookie no formato de sessão abaixo):
  GET {{base_url}}/session
  HTTP 200
  [Asserts]
  header "Set-Cookie" matches "^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"
Padrão string JSON literal, sem interpolação no perfil padrão; busca parcial, ^/$ ancoram.
Opções Go (?i), (?m), (?s) e Unicode seguem a US-15, sem normalização.
Ausente falha com 1; matches vazio passa em todo header presente, inclusive vazio.
^$ passa somente no vazio presente. Somente primeira ocorrência, sem juntar valores.
Regex/escape inválido, lookaround e backreferences: 2 antes de qualquer HTTP do arquivo.
Padrão e real de Set-Cookie e demais nomes sensíveis ficam integralmente ocultos.
Sem regex de body/variável em assertions, filtros ou seleção de todas as ocorrências.
Extração regex de body/header é suportada em capturas (US-25, exemplo abaixo).
Capturas literais de headers são suportadas em [Captures] (exemplo abaixo).
Ordem: resposta consumida/fechada, status, capturas provisórias, assertions na ordem escrita, confirmação, resultado.
Falha de header retorna 1, não publica capturas e impede próximo envio.
Diagnóstico localizado distingue ausente de ""; valores de Authorization,
Proxy-Authorization, Cookie e Set-Cookie são ocultos nos dois lados.
Segredos conhecidos também são removidos de headers comuns.
Diferenças Hurl 8.0.0: repetidos usam primeira ocorrência, não lista;
esperados são literais sem --render-asserts. Capturas antecedem assertions desde a US-34. Veja docs/compatibility.md.

Assertions JSON: salve product.hurl e execute
  httpgo test product.hurl --var base_url=http://localhost:8080
(requer resposta {"name":"Pizza","price":4500,"active":true}):
  GET {{base_url}}/products/1
  HTTP 200
  [Asserts]
  jsonpath "$.name" == "Pizza"
  jsonpath "$.price" == 4500
  jsonpath "$.active" == true
jsonpath "consulta" == <literal JSON>: string, número, booleano, null, array ou objeto,
inclusive vazios, na mesma linha. Exige HTTP numérico ou HTTP *.
Consultas estáticas: $, propriedades por ponto, índices não negativos e [*] de arrays.
Sem --render-asserts: Esperados nunca interpolam; "{{nome}}" é texto literal, inclusive aninhado.
Igualdade preserva tipos: "4500" difere de 4500. 1, 1.0 e 1e0 são iguais,
com precisão exata, também aninhada, sem float64 nem expansão de potências.
Arrays exigem a mesma ordem; objetos ignoram a ordem das chaves.
Ausência e índice fora do array falham com 1, distinguíveis de null presente.
JSON inválido/vazio/adicional ou percurso incompatível gera erro de avaliação JSON (2).
Status vem primeiro; todo o lote de capturas deve passar antes de headers, JSON e body na ordem escrita.
Nas assertions, JSON decodifica sob demanda: header anterior pode falhar antes do JSON inválido,
desde que todo o lote de capturas tenha passado.
Primeira falha impede confirmação e próximos envios. Sintaxe inválida rejeita todo arquivo.
Diagnóstico: consulta, arquivo/linha/coluna, esperado/real tipados e sanitizados.
Captura secreta pendente na mesma resposta oculta integralmente os dois valores
em falhas de headers/JSON/body, sem confirmar a captura antecipadamente.
Proteção não detecta segredos arbitrários não marcados. Sem dumps de body/documento.
Inclusão e ausência JSON/header: salve validation.hurl e execute
  httpgo test validation.hurl --var base_url=http://localhost:8080
  GET {{base_url}}/validation
  HTTP 200
  [Asserts]
  jsonpath "$.codes" contains "REQUIRED"
  jsonpath "$.codes" not contains "OTHER_TENANT"
  jsonpath "$.passwordHash" not exists
  header "Set-Cookie" not contains "Domain"
Resposta exemplo: {"codes":["REQUIRED","INVALID"],"name":"Pizza"}; cookie sem Domain.
contains/not contains exigem literal escalar JSON: string, número, booleano ou null.
String busca substring literal, distinguindo caixa, sem normalizar espaços/Unicode;
nessa busca somente esperado string é aplicável. Array busca elemento direto tipado,
com igualdade numérica exata, sem JSON textual, coerção ou busca recursiva.
Array vazio não contém escalar; toda string contém "", inclusive vazia.
Ausência aprova not exists e not contains. null, vazios, false e zero presentes existem.
Tipo presente incompatível falha com 1 em ambos os sentidos de pertencimento.
JSON inválido/percurso incompatível/transporte nunca vira sucesso por negação: 2.
not exists não aceita operando. Consultas estáticas; esperados literais sem --render-asserts.
Sem negação genérica, header not exists, count contains ou wildcard de objeto.
Filtros JSONPath estáticos por igualdade são o recorte US-28 abaixo.
Falhas mostram presença/tipo e esperado protegido, nunca arrays/objetos selecionados.
Primeira falha impede confirmação/envios; sintaxe inválida tardia impede todo HTTP.
Tipos e valores vazios — US-29:
  GET {{base_url}}/session-info
  HTTP 200
  [Asserts]
  jsonpath "$.token" isString
  jsonpath "$.attempts" isInteger
  jsonpath "$.active" isBoolean
  jsonpath "$.issues" isEmpty
Resposta: {"token":"abc","attempts":3,"active":false,"issues":[]}; saída 0.
Exemplo: httpgo test testdata/scenarios/json_types.hurl --var base_url=http://localhost:8080
Unários sem esperado: isString aceita string inclusive vazia; isBoolean aceita true/false.
isInteger exige notação inteira JSON, inclusive negativo e -0, sem limite de máquina;
3.0, 3e0 e string "3" falham. Igualdade numérica continua aceitando 3 == 3.0 == 3e0;
count aceita quantidades integrais como 3.0/3e0, sem mudar o contrato anterior.
isEmpty aprova string/array/objeto vazios; whitespace, [null], {"x":null}, null,
número e booleano não passam. Ausência/tipo incompatível/condição falsa: 1.
JSON inválido/percurso incompatível: 2; operando extra ou count isInteger (e outros
unários) impede todo HTTP do arquivo. Sem schema, outros tipos ou negação genérica.
Predicado aplicado uma vez ao valor reduzido, nunca implicitamente por item:
zero resultados é ausência; um conserva tipo; vários formam array não vazio.
Diagnóstico categórico de consulta/predicado/presença/tipo, sem valores ou estruturas;
metadados protegidos e captura secreta provisória são ocultos, sem confirmar o lote.
Atalhos: matches "" verifica tipo string; == ""/count == 0 verificam vazios específicos.
Estrutura, quantidade e limites:
  GET {{base_url}}/products
  HTTP 200
  [Asserts]
  jsonpath "$.items" count == 3
  jsonpath "$.total" >= 1
  jsonpath "$.items[0].id" exists
jsonpath "$.items[0].id" exists: null, vazios, false e zero presentes existem.
jsonpath "$.items" count == 3: conta somente arrays, inclusive vazio (zero).
count aceita ==, >, >=, < e <=; esperado inteiro não negativo, sem limite de máquina.
Ausência ou tipo não array falha com 1; diagnóstico de count não despeja o array.
jsonpath "$.total" >= 1: >, >=, < e <= exigem números, sem coerção de strings.
Negativos, decimais e expoentes com precisão exata, sem arredondar.
jsonpath "$.name" matches "^Pizza": encontra trecho em string JSON, sem coerção;
^/$ ancoram, opções Go como (?i) são aceitas. Padrão JSON literal, sem interpolação no perfil padrão.
Ausência/tipo incompatível/condição falsa: 1; JSON inválido: 2.
Regex Go inválida/não suportada e matches em body/variável: erro 2 antes da rede.
Headers também aceitam matches, com primeira ocorrência e padrão JSON literal (acima).
Sem count em strings/objetos ou JSONPath completo. Captura JSONPath count: US-33 acima.
Diferenças Hurl 8.0.0 observadas: literais compostos e esperado 125e-2 rejeitados
pelo Hurl; esperado literal aqui no perfil padrão, sem interpolação. Veja docs/compatibility.md.

Projeção de coleções — US-27 (arrays, não objetos):
  GET {{base_url}}/validation
  HTTP 200
  [Asserts]
  jsonpath "$.issues[*].code" contains "REQUIRED"
  jsonpath "$.issues[*].code" not contains "OTHER_TENANT"
Resposta: {"issues":[{"code":"REQUIRED"},{"code":"INVALID"}]}; independe da ordem.
Combina [*], propriedades e índices: $.categories[*].products[*].id.
Redução somente ao final: zero resultados = ausência; um = próprio valor;
vários = array na ordem de percurso, sem ordenar/deduplicar nem achatar arrays.
Ramo ausente/índice fora do array não acrescenta null; null selecionado existe.
Array vazio selecionado existe; wildcard em array vazio não encontra resultados.
count conta o array reduzido: dois números contam 2; número único/ausência falha com 1.
Capturas usam a mesma seleção tipada e publicação atômica; zero resultados falha com 2.
Arrays/objetos capturados exigem valor JSON completo, sem escolher o primeiro elemento.
Precisão preservada além de int64, decimais próximos e expoentes extremos, sem coerção.
US-27b: Sem nova sintaxe ou opção de avaliador na adoção do adapter;
exemplo: testdata/scenarios/scenario_projection_precision.hurl. Filtros são a US-28 abaixo.
Ramo incompatível invalida toda consulta expansiva com 2, mesmo com ramos válidos.
Diagnóstico resume presença/tipo/contagem sem coleção ou esperado composto;
resumos protegidos ficam ocultos, inclusive com captura secreta pendente.
Sem .*, slices, unions, descida recursiva ou JSONPath completo.
Exemplo executável com captura/reuso: testdata/scenarios/scenario_projection.hurl.

Selecionar entidades por igualdade — US-28:
  GET {{base_url}}/products
  HTTP 200
  [Captures]
  pizza_id: jsonpath "$[?(@.slug=='pizza')].id"
  [Asserts]
  jsonpath "$[?(@.slug=='pizza')].active" == true
  GET {{base_url}}/products/{{pizza_id}}
  HTTP 200
Filtro [?(@.caminho == literal escalar)] estático, em assertions e capturas.
Caminho do atributo: propriedades por ponto e índices não negativos, no próprio item.
Literal: string com aspas simples/duplas e escapes, número JSON exato, booleano ou null.
Ausente não corresponde nem a null; sem coerção ou mistura entre entidades.
Composição com [*] e filtros aninhados; redução somente ao final, sem achatamento.
count conta o array reduzido, não correspondências; null único é captura válida.
Captura plural é JSON tipado, nunca texto em URL/header ou escolha do primeiro ID.
Percurso incompatível no atributo ou filtro em não array: 2, sem resultado parcial.
Status → capturas provisórias → assertions → confirmação; falha impede publicação e próxima requisição.
Sem outros operadores, existência, lógica, regex ou funções no filtro.
Operandos dinâmicos exigem --render-asserts (US-31); sem a flag, placeholders na
consulta são rejeitados, inclusive em literal. Consulta inválida tardia impede todo HTTP.
Consulta apresentada oculta sempre os literais do filtro; planos não entram no resultado.
Exemplo executável: testdata/scenarios/scenario_filter.hurl; diferenças: docs/compatibility.md.

Selecionar com valores anteriores — US-31 (evolução explícita HTTPGo):
  httpgo test testdata/scenarios/dynamic_filter.hurl --render-asserts --var base_url=http://localhost:8080
  GET {{base_url}}/first
  HTTP 200
  [Captures]
  order_number: jsonpath "$.number"
  GET {{base_url}}/orders
  HTTP 200
  [Asserts]
  jsonpath "$.items[?(@.number=={{order_number}})].status" == "PLACED"
  [Captures]
  selected_id: jsonpath "$.items[?(@.number=={{order_number}})].id"
Requer /first com {"number":42} e /orders com
{"items":[{"number":42,"status":"PLACED","id":"selected"}]}.
O exemplo versionado também consulta /selected após publicar a captura.
Placeholder completo conserva string, número exato, booleano ou null; composição
como @.name=='Order {{suffix}}' sempre produz string. Array/objeto é avaliação 2,
inclusive vazio. Aspas, barras, colchetes e texto de filtro inseridos são dados:
operandos vinculados ao plano validado, sem renderizar/reanalisar a expressão.
Sem recursão, coerção de string numérica ou placeholders em propriedades, índices,
caminhos relativos, operadores ou consulta inteira. Não amplia a linguagem US-28.
Valida estrutura de todo arquivo antes da rede, sem resolver variável. Ao avaliar,
prepara esperado primeiro, depois todos os operandos antes de decode/seleção,
mesmo com seleção vazia.
Ausência/tipo inválido: 2 localizado, sem publicar lote ou enviar próxima entrada.
Consultas de captura usam só iniciais/capturas anteriores; assertions veem o lote atual.
Status → capturas provisórias → assertions → confirmação atômica.
Hurl interpola por padrão e admite formas estruturais excluídas aqui; tipos,
escapes e publicação podem divergir. Veja docs/compatibility.md e docs/subset.md.

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
Duplicação na mesma fonte e ancestral/descendente são erros.
CLI substitui o mesmo nome YAML, preservando irmãos; objetos CLI continuam opacos.
Valores JSON completos (inclusive raiz) aceitam qualquer tipo. Dentro de strings
JSON, URL e headers, somente escalares; strings JSON são escapadas corretamente.
Chaves JSON e nomes de headers são estáticos. Sem recursão ou encoding de URL automático.
Capturas literais de headers — salve cache.hurl:
  GET {{base_url}}/catalog
  HTTP 200
  [Captures]
  catalog_etag: header "ETag"
  GET {{base_url}}/catalog
  If-None-Match: {{catalog_etag}}
  HTTP 304
  httpgo test cache.hurl --var base_url=http://localhost:8080
Nome literal sem distinguir caixa; primeira ocorrência, sem juntar ou dividir vírgulas.
Preserva string e aspas, mesmo com números/JSON/placeholders; vazio é string vazia.
Ausente: erro de captura 2. Somente resposta final completa, após status aceito.
Lotes header/JSONPath são atômicos e permitem sobrescrita exata (US-34). Header isolado não retém body.
Authorization, Proxy-Authorization, Cookie e Set-Cookie marcam a captura como secreta
na sessão, inclusive em composição posterior; --secret continua acumulando proteção.
Somente sufixo regex estático (US-25), sem outros filtros/seletores.
Não há cookie jar: capturar Set-Cookie não cria Cookie
nem extrai atributos; essa extração pertence à US-25. Diferenças Hurl em docs/compatibility.md.
Extração regex — salve otp.hurl e marque --secret otp_code:
GET {{mailbox_url}}/message/123
HTTP 200
[Captures]
otp_code: regex "verificação é: ([0-9]{6})"

POST {{base_url}}/verify
Content-Type: application/json
{"code":"{{otp_code}}"}
HTTP 200

Header com regex: refresh_token: header "Set-Cookie" regex "refresh=([^;]+)"
Na próxima requisição: Cookie: refresh={{refresh_token}} (sem atributos de resposta).
Extrai string do primeiro grupo da primeira correspondência. Grupo participante vazio
é válido; sem grupo, sem correspondência ou grupo não participante: erro de captura 2.
Padrão string JSON estático, regex Go/US-15, sem interpolação; inválido rejeita todo
arquivo antes da rede. Body usa charset/UTF-8 estrito; header não decodifica body.
Headers sensíveis protegem também o grupo; body exige --secret ou secrets YAML.
Sem escolha de grupo, substituição ou pipelines. Assertions podem usar captura própria desde a US-34.

Capturas tipadas para entradas seguintes (URL, headers, JSON inline e externo com --render-files):
  POST {{base_url}}/auth/login
  {"username":"{{username}}","password":"{{password}}"}
  HTTP 200
  [Captures]
  accessToken: jsonpath "$.accessToken"
  GET {{base_url}}/products
  Authorization: Bearer {{accessToken}}
  HTTP 200
  httpgo test products.hurl --var base_url=http://localhost:8080 --var username=demo --var password=senha-ficticia --secret password --secret accessToken
JSONPath limitado: raiz $, propriedades por ponto, índices não negativos, [*]
de arrays e filtros estáticos por igualdade (US-28), como $[?(@.slug=='pizza')].id. Aceita string, número, booleano, null, array ou objeto,
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
Status divergente impede capturas; captura inválida precede assertions. Divergência/erro
impede confirmação e próximos envios. Capturas substituem nome exato inicial/anterior;
duplicações no lote e conflitos hierárquicos são erro 2. Cada arquivo começa com o contexto inicial.
--secret nome: protege variável ou captura futura, sem fornecer valor nem resolver ausência.
Proibido fornecer credenciais reais em argumentos, inclusive --var: use apenas
valores não sigilosos ou fictícios. A proteção não cobre histórico/processos.
Configuração do projeto — extensão HTTPGo (US-11):
  httpgo test products.hurl --config testdata/config/project.yaml
  httpgo test products.hurl --config testdata/config/project.yaml --var api.baseUrl=http://localhost:9090
Sem --config: somente .httpgo.yaml do diretório corrente, sem buscar ancestrais.
--config seleciona somente o arquivo indicado; inexistente/ilegível é erro.
Ausência do opcional e arquivo vazio/só comentários conservam os defaults internos.
Exemplo de .httpgo.yaml (YAML, não sintaxe Hurl):
  version: 1
  defaults:
    timeout: 2.5
  variables:
    api:
      baseUrl: http://localhost:8080
    product:
      price: 4500
  secrets: [auth.token, password]
Precedência: defaults internos < YAML base < ambiente selecionado < CLI.
--var substitui por nome exato, preservando folhas irmãs.
Mapas variables definem caminhos, não objetos interpoláveis. Tipos: string,
número finito exato, booleano, null e arrays JSON-compatíveis; mapas dentro de
arrays são conteúdo. Strings numéricas entre aspas conservam o tipo string.
Timeout: número de segundos entre 0.001 e 86400, precisão de milissegundos inteiros,
sem texto com unidade ou arredondamento. Um orçamento cobre conexão/TLS,
redirects e consumo da resposta; cada nova requisição recebe seu próprio limite.
YAML: máximo 1 MiB, documento único, raiz mapa, version: 1 obrigatória.
Rejeita campos desconhecidos, chaves/caminhos repetidos, conflitos, âncoras,
aliases e tags explícitas, sem mostrar entrada bruta.
Referências ${NOME} seguem o contrato da extensão US-13 abaixo.
secrets acumula marcações da base, ambiente e --secret, inclusive para capturas
futuras e overrides; lista vazia nunca remove proteção de valores anteriores.
Payloads permanecem relativos ao .hurl, nunca à configuração.
Erro de configuração: código 2, antes de qualquer HTTP.
Ambientes nomeados — extensão HTTPGo (US-12):
  httpgo test tests/products.hurl --env local
  httpgo test tests/products.hurl --config projeto/base.yaml --env staging
--env <nome> (ou --env=nome): seleção única; repetir, mesmo nome, é erro 2.
Nome não vazio: somente letras ASCII, números, _ e -; sem pontos ou separadores.
Carrega somente environments/<nome>.yaml, relativo ao diretório da base selecionada,
inclusive vazia. Sem base, usa o diretório corrente, nunca cenário ou ancestrais.
Sem --env, não abre ambiente algum. Arquivo solicitado ausente/ilegível é erro 2,
antes de qualquer HTTP e sem fallback. Versão 1 opcional no ambiente; mesmos campos,
tipos, timeout, limite de 1 MiB e proteções YAML da base. Vazio/comentários não alteram nada.
Mapas combinam profundamente por caminho; escalares, listas e null substituem folhas,
sem concatenar listas ou remover null. Trocar mapa de caminhos por valor ou vice-versa
é erro, inclusive com mapa vazio. Cada fonte é validada antes de aplicar overrides.
Contexto, timeout e capturas são novos por execução; payloads seguem relativos ao .hurl.
Ambiente nomeado NÃO é ambiente do processo. Referências explícitas: US-13 abaixo.
Sem overrides automáticos HTTPGO_* nem entrada de segredos por stdin.

Credenciais pelo ambiente do processo — extensão HTTPGo (US-13):
Em environments/local.yaml (ou em variables da base com version: 1):
  variables:
    auth:
      token: ${OWNER_TOKEN}
Em tests/profile.hurl:
  GET http://localhost:8080/profile
  Authorization: Bearer {{auth.token}}
  HTTP 200
Com OWNER_TOKEN fornecido externamente: httpgo test tests/profile.hurl --env local
${NOME} ocupa o valor string completo; nome segue [A-Za-z_][A-Za-z0-9_]*.
Somente folhas de variables: mapas são caminhos; listas/objetos dentro delas
não aceitam referências. Composição parcial (Bearer ${OWNER_TOKEN}), nomes
inválidos e delimitadores malformados são erros, mesmo com override posterior.
Valida cada fonte; aplica base < ambiente < CLI e consulta só referências efetivas.
Referência substituída não exige a variável do processo. Todas as efetivas são
obrigatórias, mesmo não usadas: ausência retorna 2 antes de HTTP, com nome/chave/origem.
Presente mas vazia é string vazia, distinta de ausência; 123, true e null são strings.
Conteúdo recebido é literal: não expande outra referência ou {{placeholder}}.
Destino ganha proteção automática antes dos overrides e acumula secrets/--secret.
--var pode substituir por valor fictício, conservando precedência e proteção;
credenciais reais em argumentos continuam proibidas. Valores conhecidos, derivados
e esperado/recebido ecoados são sanitizados antes de resultados/callbacks/streams.
Não importa variáveis sem referência nem oferece dotenv, stdin ou listas.
Não detecta segredos desconhecidos ou transformações arbitrárias.
Templates externos exigem --render-files.
Proteção cobre valores brutos/decodificados e escapes conhecidos, não codificações arbitrárias.
Atenção: o path não protegido permanece visível; não coloque segredos arbitrários nele.
`
}
