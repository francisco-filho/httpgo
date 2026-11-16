package parser

import (
	"regexp"
	"strings"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func parseCapture(text string, pos Position) (Capture, error) {
	fail := func(detail string) (Capture, error) {
		return Capture{}, &Error{Position: pos, Category: "sintaxe inválida", Detail: detail}
	}
	name, rest, ok := strings.Cut(text, ":")
	name = strings.TrimRight(name, " \t")
	if !ok || !template.ValidName(name) {
		return fail("nome ou declaração de captura inválido")
	}
	rest = strings.TrimLeft(rest, " \t")
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return fail("captura exige consulta jsonpath ou header entre aspas")
	}
	if fields[0] == "regex" {
		queryPos := pos
		queryPos.Column += len(text) - len(rest)
		return parseCaptureRegex(text, rest[len("regex"):], Capture{Name: name, Kind: RegexCapture, Position: pos, QueryPosition: queryPos})
	}
	if fields[0] != "jsonpath" && fields[0] != "header" {
		return Capture{}, &Error{Position: pos, Category: "construção não suportada", Detail: "consulta de captura não suportada; use jsonpath, header ou regex"}
	}
	quoted := strings.TrimSpace(rest[len(fields[0]):])
	if len(quoted) < 2 || quoted[0] != '"' {
		return fail("captura exige consulta jsonpath ou header entre aspas")
	}
	source, tail, valid := assertionString(quoted)
	if !valid {
		return fail("consulta de captura entre aspas inválida")
	}
	queryPos := pos
	queryPos.Column += strings.Index(text, quoted) + 1
	if fields[0] == "header" && validHeaderName(source) && len(tail) > 0 && (tail[0] == ' ' || tail[0] == '\t') {
		suffix := strings.Fields(tail)
		if len(suffix) > 0 && suffix[0] == "regex" {
			rest := strings.TrimLeft(tail, " \t")
			return parseCaptureRegex(text, rest[len("regex"):], Capture{Kind: HeaderCapture, Header: source, Name: name, Position: pos, QueryPosition: queryPos})
		}
	}
	if suffix := strings.Fields(tail); len(suffix) > 0 {
		if (tail[0] == ' ' || tail[0] == '\t') && suffix[0] == "count" {
			return Capture{}, &Error{Position: pos, Category: "construção não suportada", Detail: "count em captura não suportado; use count em assertion"}
		}
		if fields[0] == "header" && (tail[0] == ' ' || tail[0] == '\t') {
			return Capture{}, &Error{Position: pos, Category: "construção não suportada", Detail: "filtro ou seletor em captura de header não suportado"}
		}
		return fail("consulta de captura entre aspas inválida")
	}
	if fields[0] == "header" {
		if !validHeaderName(source) {
			return fail("captura exige nome de header literal válido entre aspas")
		}
		return Capture{Kind: HeaderCapture, Header: source, Name: name, Position: pos, QueryPosition: queryPos}, nil
	}
	path, err := query.Parse(source)
	if err != nil {
		category := "sintaxe inválida"
		if located, ok := err.(*query.Error); ok {
			if located.Unsupported {
				category = "construção não suportada"
			}
			// Point to the original quoted query, never an offset in decoded escapes.
		}
		return Capture{}, &Error{Position: queryPos, Category: category, Detail: err.Error()}
	}
	return Capture{Name: name, Path: path, Position: pos, QueryPosition: queryPos}, nil
}

// parseCaptureRegex accepts exactly one static JSON string, not a pipeline or template.
func parseCaptureRegex(text, rest string, declaration Capture) (Capture, error) {
	pos := declaration.Position
	pos.Column += len(text) - len(rest)
	fail := func(category, detail string) (Capture, error) {
		return Capture{}, &Error{Position: pos, Category: category, Detail: detail}
	}
	if len(rest) == 0 || (rest[0] != ' ' && rest[0] != '\t') {
		return fail("sintaxe inválida", "regex exige padrão JSON entre aspas separado por espaço")
	}
	quoted := strings.TrimLeft(rest, " \t")
	pos.Column += len(rest) - len(quoted)
	pattern, tail, valid := jsonString(quoted)
	if !valid {
		return fail("sintaxe inválida", "regex exige padrão em string JSON válida")
	}
	if strings.TrimSpace(tail) != "" {
		return fail("construção não suportada", "operando extra ou encadeamento em captura regex não suportado")
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		category := "sintaxe inválida"
		if unsupportedRegexp(pattern) {
			category = "construção não suportada"
		}
		return fail(category, "padrão regex inválido ou não suportado")
	}
	declaration.Pattern, declaration.Compiled, declaration.PatternPosition = pattern, compiled, pos
	return declaration, nil
}
