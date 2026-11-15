// Package query validates and evaluates the documented JSONPath subset.
package query

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// Error never includes the query or response content.
type Error struct {
	Offset      int
	Unsupported bool
	Detail      string
}

func (e *Error) Error() string { return e.Detail }

type step struct {
	property string
	index    int
	array    bool
}
type Path struct{ steps []step }

func Parse(source string) (Path, error) {
	fail := func(offset int, unsupported bool) (Path, error) {
		detail := "consulta JSONPath inválida"
		if unsupported {
			detail = "construção JSONPath não suportada; use raiz, propriedades por ponto e índices não negativos"
		}
		return Path{}, &Error{offset, unsupported, detail}
	}
	if len(source) == 0 || source[0] != '$' {
		return fail(0, false)
	}
	var p Path
	for i := 1; i < len(source); {
		switch source[i] {
		case '.':
			i++
			start := i
			for i < len(source) && (source[i] == '_' || source[i] >= 'a' && source[i] <= 'z' || source[i] >= 'A' && source[i] <= 'Z' || source[i] >= '0' && source[i] <= '9') {
				i++
			}
			if start == i {
				return fail(i, i < len(source))
			}
			p.steps = append(p.steps, step{property: source[start:i]})
		case '[':
			i++
			start := i
			for i < len(source) && source[i] >= '0' && source[i] <= '9' {
				i++
			}
			if start == i {
				return fail(i, i < len(source))
			}
			if i == len(source) {
				return fail(i, false)
			}
			if source[i] != ']' {
				return fail(i, true)
			}
			index, err := strconv.Atoi(source[start:i])
			if err != nil {
				return fail(start, true)
			}
			p.steps = append(p.steps, step{index: index, array: true})
			i++
		default:
			return fail(i, true)
		}
	}
	return p, nil
}

// Select preserves JSON numbers and distinguishes a missing path from JSON null.
func (p Path) Select(body []byte) (any, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, false, fmt.Errorf("resposta não contém JSON válido")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, false, fmt.Errorf("resposta não contém JSON válido")
	}
	for _, s := range p.steps {
		if s.array {
			array, ok := value.([]any)
			if !ok || s.index >= len(array) {
				return nil, false, nil
			}
			value = array[s.index]
		} else {
			object, ok := value.(map[string]any)
			if !ok {
				return nil, false, nil
			}
			value, ok = object[s.property]
			if !ok {
				return nil, false, nil
			}
		}
	}
	return value, true, nil
}
