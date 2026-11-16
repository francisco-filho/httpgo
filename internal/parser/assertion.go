package parser

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/query"
)

// HeaderAssertion contains literal values, never template expressions.
type HeaderAssertion struct {
	Name     string
	Operator assertion.HeaderOperator
	Expected string
	Position Position
}

// Assertion is one of the concrete header/JSON variants, in source order.
type Assertion struct {
	Header *HeaderAssertion
	JSON   *JSONAssertion
}

// JSONAssertion owns an already validated literal, never a template plan.
type JSONAssertion struct {
	Query         string
	Path          query.Path
	Operator      assertion.JSONOperator
	Count         bool
	Expected      any
	Pattern       string
	Compiled      *regexp.Regexp
	Position      Position
	QueryPosition Position
}

func parseAssertion(text string, pos Position) (Assertion, error) {
	fields := strings.Fields(text)
	if len(fields) > 0 && fields[0] == "jsonpath" {
		declaration, err := parseJSONAssertion(text, pos)
		return Assertion{JSON: &declaration}, err
	}
	declaration, err := parseHeaderAssertion(text, pos)
	return Assertion{Header: &declaration}, err
}

func parseJSONAssertion(text string, pos Position) (JSONAssertion, error) {
	fail := func(position Position, category, detail string) (JSONAssertion, error) {
		return JSONAssertion{}, &Error{Position: position, Category: category, Detail: detail}
	}
	rest := strings.TrimLeft(text[len("jsonpath"):], " \t")
	source, tail, ok := assertionString(rest)
	if !ok {
		return fail(pos, "sintaxe inválida", "assertion exige consulta JSONPath entre aspas duplas")
	}
	queryPos := pos
	queryPos.Column += len(text) - len(rest) + 1
	path, err := query.Parse(source)
	if err != nil {
		category := "sintaxe inválida"
		if located, ok := err.(*query.Error); ok && located.Unsupported {
			category = "construção não suportada"
		}
		return fail(queryPos, category, err.Error())
	}
	if tail == "" || (tail[0] != ' ' && tail[0] != '\t') {
		return fail(pos, "sintaxe inválida", "assertion exige operador separado por espaço")
	}
	rest = strings.TrimSpace(tail)
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return fail(pos, "sintaxe inválida", "assertion exige operador")
	}
	declaration := JSONAssertion{Query: source, Path: path, Position: pos, QueryPosition: queryPos}
	if fields[0] == "count" {
		declaration.Count = true
		rest = strings.TrimSpace(rest[len("count"):])
		fields = strings.Fields(rest)
		if len(fields) == 0 {
			return fail(pos, "sintaxe inválida", "count exige comparador e quantidade")
		}
	}
	declaration.Operator = assertion.JSONOperator(fields[0])
	rest = strings.TrimSpace(rest[len(fields[0]):])
	switch declaration.Operator {
	case assertion.JSONExists:
		if declaration.Count {
			return fail(pos, "construção não suportada", "count aceita somente ==, >, >=, < ou <=")
		}
		if rest != "" {
			return fail(pos, "sintaxe inválida", "exists não aceita valor esperado")
		}
		return declaration, nil
	case assertion.JSONEqualOperator:
	case assertion.JSONMatches:
		pattern, trailing, valid := jsonString(rest)
		if !valid || strings.TrimSpace(trailing) != "" {
			return fail(pos, "sintaxe inválida", "matches exige somente uma string JSON entre aspas duplas")
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			category := "sintaxe inválida"
			if unsupportedRegexp(pattern) {
				category = "construção não suportada"
			}
			return fail(pos, category, "padrão matches inválido ou não suportado")
		}
		declaration.Expected = pattern
		declaration.Pattern = pattern
		declaration.Compiled = compiled
		return declaration, nil
	case assertion.JSONGreater, assertion.JSONGreaterEqual, assertion.JSONLess, assertion.JSONLessEqual:
	default:
		return fail(pos, "construção não suportada", "operador ou filtro de assertion JSON não suportado")
	}
	expected, err := query.Decode([]byte(rest))
	if err != nil {
		return fail(pos, "sintaxe inválida", "assertion exige um único literal JSON esperado válido")
	}
	if declaration.Count && !assertion.ValidCount(expected) {
		return fail(pos, "sintaxe inválida", "count exige quantidade inteira não negativa")
	}
	if declaration.Operator != assertion.JSONEqualOperator {
		if _, ok := expected.(json.Number); !ok {
			return fail(pos, "sintaxe inválida", "comparação ordenada exige número JSON esperado")
		}
	}
	declaration.Expected = expected
	return declaration, nil
}

func parseHeaderAssertion(text string, pos Position) (HeaderAssertion, error) {
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
		return fail("construção não suportada", "consulta de assertion não suportada; use header ou jsonpath")
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

func jsonString(text string) (string, string, bool) {
	if len(text) < 2 || text[0] != '"' {
		return "", "", false
	}
	for i := 1; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if text[i] == '"' {
			var value string
			if err := json.Unmarshal([]byte(text[:i+1]), &value); err != nil {
				return "", "", false
			}
			return value, text[i+1:], true
		}
	}
	return "", "", false
}

func unsupportedRegexp(pattern string) bool {
	if strings.Contains(pattern, "(?=") || strings.Contains(pattern, "(?!") || strings.Contains(pattern, "(?<=") || strings.Contains(pattern, "(?<!") {
		return true
	}
	for i := 0; i+1 < len(pattern); i++ {
		if pattern[i] == '\\' && pattern[i+1] >= '1' && pattern[i+1] <= '9' {
			return true
		}
		if pattern[i] == '\\' {
			i++
		}
	}
	return false
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
