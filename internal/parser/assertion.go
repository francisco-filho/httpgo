package parser

import (
	"strconv"
	"strings"

	"capimgrosso.com/httpgo/internal/assertion"
)

// HeaderAssertion contains literal values, never template expressions.
type HeaderAssertion struct {
	Name     string
	Operator assertion.HeaderOperator
	Expected string
	Position Position
}

func parseAssertion(text string, pos Position) (HeaderAssertion, error) {
	fail := func(category, detail string) (HeaderAssertion, error) {
		return HeaderAssertion{}, &Error{Position: pos, Category: category, Detail: detail}
	}
	token, rest, _ := strings.Cut(text, " ")
	// Tabs are valid separators too.
	fields := strings.Fields(text)
	if len(fields) > 0 {
		token = fields[0]
		rest = strings.TrimLeft(text[len(token):], " \t")
	}
	if token != "header" {
		return fail("construção não suportada", "consulta de assertion não suportada; use header")
	}
	name, rest, ok := assertionString(rest)
	if !ok || !validHeaderName(name) {
		return fail("sintaxe inválida", "assertion exige nome de header válido entre aspas duplas")
	}
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return fail("sintaxe inválida", "assertion exige operador separado por espaço")
	}
	rest = strings.TrimLeft(rest, " \t")
	fields = strings.Fields(rest)
	if len(fields) == 0 {
		return fail("sintaxe inválida", "assertion exige operador")
	}
	operator := assertion.HeaderOperator(fields[0])
	rest = strings.TrimSpace(rest[len(fields[0]):])
	switch operator {
	case assertion.HeaderExists:
		if rest != "" {
			return fail("sintaxe inválida", "exists não aceita valor esperado")
		}
	case assertion.HeaderEqual, assertion.HeaderContains:
		expected, tail, valid := assertionString(rest)
		if !valid || strings.TrimSpace(tail) != "" {
			return fail("sintaxe inválida", "assertion exige somente uma string literal esperada entre aspas duplas")
		}
		return HeaderAssertion{Name: name, Operator: operator, Expected: expected, Position: pos}, nil
	default:
		return fail("construção não suportada", "operador ou filtro de assertion não suportado")
	}
	return HeaderAssertion{Name: name, Operator: operator, Position: pos}, nil
}

func assertionString(text string) (string, string, bool) {
	if len(text) < 2 || text[0] != '"' {
		return "", "", false
	}
	for i := 1; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if text[i] == '"' {
			value, err := strconv.Unquote(text[:i+1])
			return value, text[i+1:], err == nil
		}
	}
	return "", "", false
}
