package query

import (
	"encoding/json"
	"strings"

	"capimgrosso.com/httpgo/internal/template"
)

// equalityFilter is operational state, never presentation metadata. Its operand
// is decoded independently of jp so all JSON numbers retain their exact spelling.
type equalityFilter struct {
	attribute                Path
	literal                  any
	operand                  *filterOperand
	literalStart, literalEnd int
}

func filterError(offset int, unsupported bool) error {
	detail := "filtro JSONPath inválido; use [?(@.caminho == literal escalar)]"
	if unsupported {
		detail = "construção de filtro JSONPath não suportada; use somente caminho por propriedades/índices e igualdade com literal escalar"
	}
	return &Error{Offset: offset, Unsupported: unsupported, Detail: detail}
}

func skipFilterSpace(source string, i int) int {
	for i < len(source) && (source[i] == ' ' || source[i] == '\t') {
		i++
	}
	return i
}

func parseEqualityFilter(source string, start int, renderAsserts bool) (*equalityFilter, int, error) {
	i := start + 2 // [?
	if i == len(source) || source[i] != '(' {
		return nil, i, filterError(i, false)
	}
	i = skipFilterSpace(source, i+1)
	if i == len(source) || source[i] != '@' {
		return nil, i, filterError(i, true)
	}
	i++
	attrStart := i
	// The relative path has no quotes, wildcards, filters or dynamic fragments.
	for i < len(source) && (source[i] == '.' || source[i] == '_' || source[i] == '[' || source[i] == ']' || source[i] >= 'a' && source[i] <= 'z' || source[i] >= 'A' && source[i] <= 'Z' || source[i] >= '0' && source[i] <= '9') {
		i++
	}
	if attrStart == i || source[attrStart] != '.' {
		return nil, i, filterError(i, true)
	}
	if i < len(source) && strings.ContainsRune("*-'\"{:,", rune(source[i])) {
		return nil, i, filterError(i, true)
	}
	attribute, err := Parse("$" + source[attrStart:i])
	if err != nil {
		located, ok := err.(*Error)
		return nil, i, filterError(i, ok && located.Unsupported)
	}
	i = skipFilterSpace(source, i)
	if i == len(source) {
		return nil, i, filterError(i, false)
	}
	if !strings.HasPrefix(source[i:], "==") {
		return nil, i, filterError(i, true)
	}
	i = skipFilterSpace(source, i+2)
	if i == len(source) {
		return nil, i, filterError(i, false)
	}
	if source[i] == '@' {
		return nil, i, filterError(i, true)
	}
	literalStart := i
	var literal any
	var operand *filterOperand
	if strings.HasPrefix(source[i:], "{{") {
		if !renderAsserts {
			return nil, i, filterError(i, true)
		}
		end := strings.Index(source[i+2:], "}}")
		if end < 0 {
			return nil, i, filterError(i, false)
		}
		end += i + 4
		refs, parseErr := template.Parse(source[i:end])
		if parseErr != nil || len(refs) != 1 {
			return nil, i, filterError(i, false)
		}
		operand = &filterOperand{source: source[i:end], complete: true}
		i = end
	} else if source[i] == '\'' || source[i] == '"' {
		literal, i, err = filterString(source, i)
		if err != nil {
			return nil, i, err
		}
		if renderAsserts {
			text := literal.(string)
			refs, parseErr := template.Parse(text)
			if parseErr != nil {
				return nil, literalStart, filterError(literalStart, false)
			}
			if len(refs) > 0 {
				operand = &filterOperand{source: text}
				literal = nil
			}
		}
	} else {
		begin := i
		for i < len(source) && source[i] != ')' && source[i] != ' ' && source[i] != '\t' {
			i++
		}
		literal, err = Decode([]byte(source[begin:i]))
		if err != nil {
			return nil, begin, filterError(begin, false)
		}
		switch literal.(type) {
		case nil, bool, json.Number:
		default:
			return nil, begin, filterError(begin, true)
		}
	}
	literalEnd := i
	i = skipFilterSpace(source, i)
	if i == len(source) {
		return nil, i, filterError(i, false)
	}
	if source[i] != ')' {
		return nil, i, filterError(i, true)
	}
	i++
	if i == len(source) || source[i] != ']' {
		return nil, i, filterError(i, false)
	}
	attribute.compileProjection()
	return &equalityFilter{attribute: attribute, literal: literal, operand: operand, literalStart: literalStart, literalEnd: literalEnd}, i + 1, nil
}

// filterString handles the inner query literal, after the outer .hurl string
// has been decoded. Normalize only the quote delimiter; JSON decodes escapes.
func filterString(source string, start int) (string, int, error) {
	quote := source[start]
	var encoded strings.Builder
	encoded.WriteByte('"')
	for i := start + 1; i < len(source); i++ {
		c := source[i]
		if c == quote {
			encoded.WriteByte('"')
			var value string
			if err := json.Unmarshal([]byte(encoded.String()), &value); err != nil {
				return "", i, filterError(i, false)
			}
			return value, i + 1, nil
		}
		if c == '\\' {
			i++
			if i == len(source) {
				return "", i, filterError(i, false)
			}
			if source[i] == '\'' && quote == '\'' {
				encoded.WriteByte('\'')
			} else {
				encoded.WriteByte('\\')
				encoded.WriteByte(source[i])
			}
		} else {
			if c == '"' {
				encoded.WriteByte('\\')
			}
			encoded.WriteByte(c)
		}
	}
	return "", len(source), filterError(len(source), false)
}

// HasProtectedLiteral lets the presentation boundary inspect decoded operands
// without exporting the filter plan or relying on textual matches of escapes.
func (p Path) HasProtectedLiteral(protected func(any) bool) bool {
	for _, s := range p.steps {
		if s.filter != nil && protected(s.filter.literal) {
			return true
		}
	}
	return false
}

// RedactLiterals removes filter operands before a query enters diagnostics. It
// must receive the same source used by Parse. This also conceals escaped forms
// of secrets that a textual sanitizer alone cannot recognize.
func (p Path) RedactLiterals(source string) string {
	var display strings.Builder
	start := 0
	for _, s := range p.steps {
		if s.filter == nil {
			continue
		}
		f := s.filter
		if f.literalStart < start || f.literalEnd > len(source) {
			return "[consulta com filtro]"
		}
		display.WriteString(source[start:f.literalStart])
		display.WriteString("[literal oculto]")
		start = f.literalEnd
	}
	display.WriteString(source[start:])
	return display.String()
}

func (f equalityFilter) selectItems(array []any) ([]any, error) {
	var selected []any
	for _, item := range array {
		actual, present, err := f.attribute.selectProjection(item)
		if err != nil {
			return nil, err
		}
		if present && JSONEqual(f.literal, actual) {
			selected = append(selected, item)
		}
	}
	return selected, nil
}
