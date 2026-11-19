package report

import (
	"fmt"
	"html/template"
	"io"
	"time"

	"capimgrosso.com/httpgo/internal/suite"
)

// HTMLEntry contains presentation text only, never an operational entry/response.
// The caller protects all text before serialization; template escaping follows.
type HTMLEntry struct {
	Number                           int
	Method, URL, Outcome, Diagnostic string
	Duration                         time.Duration
}

type HTMLFile struct {
	Result     suite.FileResult
	Diagnostic string
	Entries    []HTMLEntry
}

type HTMLDocument struct {
	Summary suite.Summary
	Files   []HTMLFile
}

// WriteHTML renders already classified, protected results, without I/O other
// than writer. The caller owns the destination and must close it even on error.
func WriteHTML(writer io.Writer, document HTMLDocument) error {
	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("preparar relatório HTML: %w", err)
	}
	if err := tmpl.Execute(completeWriter{writer}, document); err != nil {
		return fmt.Errorf("serializar relatório HTML: %w", err)
	}
	return nil
}

const htmlTemplate = `<!doctype html>
<html lang="pt-BR">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>HTTPGo — resultados dos testes</title>
<style>
:root { color-scheme: light; }
body { background: #fff; color: #1f2328; font: 16px/1.6 system-ui, sans-serif; margin: 0 auto; max-width: 1100px; padding: 32px; overflow-wrap: anywhere; }
h1, h2, h3 { line-height: 1.25; margin-top: 24px; }
h1, h2 { border-bottom: 1px solid #d1d9e0; padding-bottom: 8px; }
table { border-collapse: collapse; display: block; overflow-x: auto; margin: 16px 0; }
th, td { border: 1px solid #d1d9e0; padding: 6px 13px; text-align: left; vertical-align: top; }
tr:nth-child(even) { background: #f6f8fa; }
pre, code { font-family: ui-monospace, monospace; font-size: 0.9em; }
pre { background: #f6f8fa; border-radius: 6px; padding: 16px; white-space: pre-wrap; }
</style>
</head>
<body>
<h1>HTTPGo — resultados dos testes</h1>
<h2>Resumo agregado</h2>
<table>
<thead><tr><th>Arquivos</th><th>Sucesso</th><th>Falha de assertion</th><th>Erro</th><th>Não executado</th><th>Duração</th></tr></thead>
<tbody><tr><td>{{.Summary.Files}}</td><td>{{.Summary.Passed}}</td><td>{{.Summary.Failed}}</td><td>{{.Summary.Errors}}</td><td>{{.Summary.Skipped}}</td><td>{{.Summary.Duration}}</td></tr></tbody>
</table>
<p>Requisições conhecidas: {{.Summary.Requests.Total}}; arquivos com contagem desconhecida: {{.Summary.UnknownFiles}}.</p>
<p>Iniciadas: {{.Summary.Requests.Started}}; concluídas: {{.Summary.Requests.Completed}}; não iniciadas conhecidas: {{.Summary.Requests.NotStarted}}; sucesso: {{.Summary.Successful}}; falhas de assertion: {{.Summary.AssertionFailures}}; erros: {{.Summary.RequestErrors}}.</p>
<p>Status validados: {{.Summary.Requests.Validated}}; sem validação de status: {{.Summary.Requests.WithoutValidation}}. Status validado não significa sucesso de todas as assertions; ausência de assertion não é validação.</p>
<p>Tentativas iniciadas: {{.Summary.Attempts.Started}}; respostas completas: {{.Summary.Attempts.Completed}}; status comparados: {{.Summary.Attempts.StatusChecks}}.</p>
<h2>Resultados por arquivo</h2>
{{range .Files}}
<section>
<h3>{{.Result.Name}}</h3>
<p>Resultado: <strong>{{.Result.State}}</strong>; duração: {{.Result.Duration}}.</p>
<p>Requisições: total={{if .Result.Known}}{{.Result.Requests.Total}}{{else}}desconhecido{{end}}; iniciadas={{.Result.Requests.Started}}; concluídas={{.Result.Requests.Completed}}; não iniciadas={{if .Result.Known}}{{.Result.Requests.NotStarted}}{{else}}desconhecido{{end}}; status validados={{.Result.Requests.Validated}}; sem validação de status={{.Result.Requests.WithoutValidation}}.</p>
{{if .Diagnostic}}<pre>{{.Diagnostic}}</pre>{{end}}
{{if .Entries}}
<table>
<thead><tr><th>Requisição</th><th>Método</th><th>URL protegida</th><th>Resultado</th><th>Duração lógica</th><th>Detalhes</th></tr></thead>
<tbody>{{range .Entries}}
<tr><td>{{.Number}}</td><td>{{.Method}}</td><td><code>{{.URL}}</code></td><td>{{.Outcome}}</td><td>{{.Duration}}</td><td><pre>{{.Diagnostic}}</pre></td></tr>{{end}}
</tbody>
</table>
{{else}}<p>Sem detalhes de requisições alcançadas.</p>{{end}}
</section>
{{end}}
</body>
</html>
`
