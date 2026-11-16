package parser

import (
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
		return fail("captura exige consulta jsonpath entre aspas")
	}
	if fields[0] != "jsonpath" {
		return Capture{}, &Error{Position: pos, Category: "construção não suportada", Detail: "consulta de captura não suportada; use jsonpath"}
	}
	quoted := strings.TrimSpace(rest[len("jsonpath"):])
	if len(quoted) < 2 || quoted[0] != '"' {
		return fail("captura exige consulta jsonpath entre aspas")
	}
	source, tail, valid := assertionString(quoted)
	if !valid {
		return fail("consulta de captura entre aspas inválida")
	}
	if suffix := strings.Fields(tail); len(suffix) > 0 {
		if (tail[0] == ' ' || tail[0] == '\t') && suffix[0] == "count" {
			return Capture{}, &Error{Position: pos, Category: "construção não suportada", Detail: "count em captura não suportado; use count em assertion"}
		}
		return fail("consulta de captura entre aspas inválida")
	}
	queryPos := pos
	queryPos.Column += strings.Index(text, quoted) + 1
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
