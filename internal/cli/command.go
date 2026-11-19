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
	isTerminal func(io.Writer) bool
	compact    bool // Selected on original streams, before protection wrappers.
	color      bool
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
	detect := c.isTerminal
	if detect == nil {
		detect = terminalWriter
	}
	local.compact = detect(c.out) && detect(c.err)
	_, noColor := os.LookupEnv("NO_COLOR")
	local.color = !noColor
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
	options := suite.Options{Request: runner.Options{RenderFiles: args.renderFiles, RenderAsserts: args.renderAsserts, Timeout: settings.Timeout}, FailFast: args.failFast}
	if c.compact && args.output == "" {
		progress := &fileProgress{command: c}
		entryReporter, fileReporter = progress.entry, progress.finish
		options.Loading, options.Loaded = progress.loading, progress.loaded
	}
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
	result := execution.Run(ctx, files, options, fileReporter)
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
	renderSummary := c.renderSuite
	if c.compact {
		renderSummary = c.renderCompactSuite
	}
	if err := renderSummary(result.Summary); err != nil {
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
	case runner.RegexMismatch:
		line += ": " + regexDiagnostic(entry.RegexFailure)
	case runner.RegexFailed:
		line += ": " + entry.RegexError.Error()
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
	if entry.Outcome == runner.Cancelled || entry.Outcome == runner.FileFailed || entry.Outcome == runner.Mismatch || entry.Outcome == runner.Failed || entry.Outcome == runner.RenderFailed || entry.Outcome == runner.CaptureFailed || entry.Outcome == runner.HeaderFailed || entry.Outcome == runner.HeaderMismatch || entry.Outcome == runner.JSONMismatch || entry.Outcome == runner.JSONFailed || entry.Outcome == runner.BodyMismatch || entry.Outcome == runner.BodyFailed || entry.Outcome == runner.RegexMismatch || entry.Outcome == runner.RegexFailed {
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
		if entry.RegexFailure != nil {
			pos = entry.RegexFailure.Position
		}
		if entry.RegexError != nil {
			pos = entry.RegexError.Position
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
		var regexErr *runner.RegexError
		if errors.As(result.Err, &regexErr) {
			diagnostic = regexErr.Error()
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
		if entry.RegexFailure != nil {
			failure := entry.RegexFailure
			pos := failure.Position
			_, err := fmt.Fprintf(c.err, "%q:%d:%d: %s; execução interrompida\n", pos.File, pos.Line, pos.Column, regexDiagnostic(failure))
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

func regexDiagnostic(failure *runner.RegexFailure) string {
	actual := "grupo presente; condição falsa"
	if !failure.Present {
		actual = "grupo ausente"
	}
	return fmt.Sprintf("assertion regex %q %s: esperado %q; %s", failure.Pattern, failure.Operator, failure.Expected, actual)
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
	return `httpgo — executa cenários de teste HTTP e valida respostas

Uso:
  httpgo test <arquivo-ou-diretório>... [opções]

Opções:
  --help                   Exibe esta ajuda (também: httpgo test --help).
  --version                Exibe versão, commit e data de build.
  --config arquivo         Seleciona a configuração YAML do projeto.
  --env nome               Seleciona um ambiente YAML nomeado.
  --fail-fast              Interrompe a suíte após a primeira falha.
  --output junit|html       Gera relatório no formato indicado; exige --output-file.
  --output-file caminho    Define o destino do relatório (requer --output).
  --render-files           Interpola bodies de arquivos externos como templates JSON.
  --render-asserts         Interpola valores esperados e operandos de filtros em assertions/capturas.
  --var nome=valor         Define variável não sigilosa (opção repetível).
  --secret nome            Marca nome protegido, sem fornecer valor secreto (repetível).

Documentação: README.md e docs/subset.md em
  https://github.com/francisco-filho/httpgo

Exemplos:
  httpgo test health.httpgo
  httpgo test tests/ --env local
  httpgo test tests/ --output junit --output-file target/results.xml
`
}
