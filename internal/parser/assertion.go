package parser

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// HeaderAssertion contains literal values, never template expressions.
type HeaderAssertion struct {
	Name     string
	Operator assertion.HeaderOperator
	Expected string
	Compiled *regexp.Regexp
	Position Position
}

// BodyAssertion contains a literal substring, never a template expression.
type BodyAssertion struct {
	Operator assertion.BodyOperator
	Expected string
	Position Position
}

// Assertion is one of the concrete header/JSON/body variants, in source order.
type Assertion struct {
	Header *HeaderAssertion
	JSON   *JSONAssertion
	Body   *BodyAssertion
}

// ExpectedTemplate is operational metadata and must never be delivered to reports.
type ExpectedTemplate struct {
	Source     string
	JSON       template.JSONPlan
	References []template.Reference
}

// JSONAssertion owns a validated literal or an opt-in expected template.
type JSONAssertion struct {
	ExpectedTemplate *ExpectedTemplate
	Query            string
	Path             query.Path
	Operator         assertion.JSONOperator
	Count            bool
	Expected         any
	Pattern          string
	Compiled         *regexp.Regexp
	Position         Position
	QueryPosition    Position
}

func parseAssertion(text string, pos Position, options ParseOptions) (Assertion, error) {
	fields := strings.Fields(text)
	if len(fields) > 0 && fields[0] == "body" {
		declaration, err := parseBodyAssertion(text, pos)
		return Assertion{Body: &declaration}, err
	}
	if len(fields) > 0 && fields[0] == "jsonpath" {
		declaration, err := parseJSONAssertion(text, pos, options)
		return Assertion{JSON: &declaration}, err
	}
	declaration, err := parseHeaderAssertion(text, pos)
	return Assertion{Header: &declaration}, err
}

func parseBodyAssertion(text string, pos Position) (BodyAssertion, error) {
	fail := func(category, detail string) (BodyAssertion, error) {
		return BodyAssertion{}, &Error{Position: pos, Category: category, Detail: detail}
	}
	rest := strings.TrimSpace(text[len("body"):])
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return fail("sintaxe inválida", "assertion de body exige operador")
	}
	operator := fields[0]
	rest = strings.TrimSpace(rest[len(operator):])
	if operator == "not" {
		fields = strings.Fields(rest)
		if len(fields) == 0 {
			return fail("sintaxe inválida", "not exige contains e string literal")
		}
		operator += " " + fields[0]
		rest = strings.TrimSpace(rest[len(fields[0]):])
	}
	if operator != string(assertion.BodyContains) && operator != string(assertion.BodyNotContains) {
		return fail("construção não suportada", "operador ou filtro de assertion de body não suportado")
	}
	expected, tail, ok := assertionString(rest)
	if !ok || strings.TrimSpace(tail) != "" {
		return fail("sintaxe inválida", "assertion de body exige somente uma string literal entre aspas duplas")
	}
	return BodyAssertion{Operator: assertion.BodyOperator(operator), Expected: expected, Position: pos}, nil
}

func parseJSONAssertion(text string, pos Position, options ParseOptions) (JSONAssertion, error) {
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
	operator := fields[0]
	rest = strings.TrimSpace(rest[len(operator):])
	if operator == "not" {
		fields = strings.Fields(rest)
		if len(fields) == 0 {
			return fail(pos, "sintaxe inválida", "not exige contains ou exists")
		}
		operator += " " + fields[0]
		rest = strings.TrimSpace(rest[len(fields[0]):])
	}
	declaration.Operator = assertion.JSONOperator(operator)
	if declaration.Count && (declaration.Operator == assertion.JSONContains || declaration.Operator == assertion.JSONNotContains || declaration.Operator == assertion.JSONNotExists) {
		return fail(pos, "construção não suportada", "count aceita somente ==, >, >=, < ou <=")
	}
	switch declaration.Operator {
	case assertion.JSONIsString, assertion.JSONIsInteger, assertion.JSONIsBoolean, assertion.JSONIsEmpty:
		if declaration.Count {
			return fail(pos, "construção não suportada", "count aceita somente ==, >, >=, < ou <=")
		}
		if rest != "" {
			return fail(pos, "sintaxe inválida", "predicado de tipo ou vazio não aceita valor esperado")
		}
		return declaration, nil
	case assertion.JSONExists, assertion.JSONNotExists:
		if declaration.Count {
			return fail(pos, "construção não suportada", "count aceita somente ==, >, >=, < ou <=")
		}
		if rest != "" {
			return fail(pos, "sintaxe inválida", "exists e not exists não aceitam valor esperado")
		}
		return declaration, nil
	case assertion.JSONEqualOperator, assertion.JSONContains, assertion.JSONNotContains:
	case assertion.JSONMatches:
		pattern, compiled, err := parseMatches(rest, pos)
		if err != nil {
			return JSONAssertion{}, err
		}
		declaration.Expected = pattern
		declaration.Pattern = pattern
		declaration.Compiled = compiled
		return declaration, nil
	case assertion.JSONGreater, assertion.JSONGreaterEqual, assertion.JSONLess, assertion.JSONLessEqual:
	default:
		return fail(pos, "construção não suportada", "operador ou filtro de assertion JSON não suportado")
	}
	if options.RenderAsserts {
		plan, err := template.ParseJSON([]byte(rest))
		if err != nil || strings.TrimSpace(rest[plan.End:]) != "" {
			return fail(pos, "sintaxe inválida", "assertion exige um único template JSON esperado válido")
		}
		if len(plan.References) > 0 {
			// Only a complete value reference may change the operand's outer type.
			complete := len(plan.References) == 1 && !plan.References[0].InString && plan.References[0].Start == 0 && plan.References[0].End == len(rest)
			if !complete {
				masked := rest
				for i := len(plan.References) - 1; i >= 0; i-- {
					ref := plan.References[i]
					replacement := "null"
					if ref.InString {
						replacement = "x"
					}
					masked = masked[:ref.Start] + replacement + masked[ref.End:]
				}
				value, _ := query.Decode([]byte(masked))
				if err := assertion.ValidateJSONExpected(declaration.Operator, declaration.Count, value); err != nil {
					return fail(pos, "sintaxe inválida", err.Error())
				}
			}
			declaration.ExpectedTemplate = &ExpectedTemplate{Source: rest, JSON: plan}
			return declaration, nil
		}
	}
	expected, err := query.Decode([]byte(rest))
	if err != nil {
		return fail(pos, "sintaxe inválida", "assertion exige um único literal JSON esperado válido")
	}
	if err := assertion.ValidateJSONExpected(declaration.Operator, declaration.Count, expected); err != nil {
		return fail(pos, "sintaxe inválida", err.Error())
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
		return fail("construção não suportada", "consulta de assertion não suportada; use header, jsonpath ou body")
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
	token = fields[0]
	rest = strings.TrimSpace(rest[len(token):])
	if token == "not" {
		fields = strings.Fields(rest)
		if len(fields) == 0 {
			return fail("sintaxe inválida", "not exige contains e string literal")
		}
		token += " " + fields[0]
		rest = strings.TrimSpace(rest[len(fields[0]):])
	}
	operator := assertion.HeaderOperator(token)
	switch operator {
	case assertion.HeaderExists:
		if rest != "" {
			return fail("sintaxe inválida", "exists não aceita valor esperado")
		}
	case assertion.HeaderMatches:
		pattern, compiled, err := parseMatches(rest, pos)
		if err != nil {
			return HeaderAssertion{}, err
		}
		return HeaderAssertion{Name: name, Operator: operator, Expected: pattern, Compiled: compiled, Position: pos}, nil
	case assertion.HeaderEqual, assertion.HeaderContains, assertion.HeaderNotContains:
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

// parseMatches validates the static JSON operand without exposing its contents or
// the regexp library's error, which can contain protected pattern fragments.
func parseMatches(text string, pos Position) (string, *regexp.Regexp, error) {
	pattern, trailing, valid := jsonString(text)
	if !valid || strings.TrimSpace(trailing) != "" {
		return "", nil, &Error{Position: pos, Category: "sintaxe inválida", Detail: "matches exige somente uma string JSON entre aspas duplas"}
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		category := "sintaxe inválida"
		if unsupportedRegexp(pattern) {
			category = "construção não suportada"
		}
		return "", nil, &Error{Position: pos, Category: category, Detail: "padrão matches inválido ou não suportado"}
	}
	return pattern, compiled, nil
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
