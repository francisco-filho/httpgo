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
	wildcard bool
}
type Path struct{ steps []step }

// Expansive reports whether the path projects array elements.
func (p Path) Expansive() bool {
	for _, s := range p.steps {
		if s.wildcard {
			return true
		}
	}
	return false
}

func Parse(source string) (Path, error) {
	fail := func(offset int, unsupported bool) (Path, error) {
		detail := "consulta JSONPath inválida"
		if unsupported {
			detail = "construção JSONPath não suportada; use raiz, propriedades por ponto, índices não negativos e [*] de arrays"
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
			if i < len(source) && source[i] == '*' {
				i++
				if i == len(source) || source[i] != ']' {
					return fail(i, false)
				}
				p.steps = append(p.steps, step{array: true, wildcard: true})
				i++
				continue
			}
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

// Decode reads exactly one JSON value, preserving numbers without float conversion.
func Decode(body []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("resposta não contém JSON válido")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("resposta não contém JSON válido")
	}
	return value, nil
}

// Select preserves the legacy capture contract on simple paths only:
// incompatible traversal counts as absence. Projections propagate traversal errors.
func (p Path) Select(body []byte) (any, bool, error) {
	value, err := Decode(body)
	if err != nil {
		return nil, false, err
	}
	value, present, err := p.SelectValue(value)
	if err != nil && !p.Expansive() {
		return nil, false, nil
	}
	return value, present, err
}

// SelectValue distinguishes absence from traversal through an incompatible type.
// A present null is a value; traversing through it is an evaluation error.
func (p Path) SelectValue(value any) (any, bool, error) {
	nodes := []any{value}
	for _, s := range p.steps {
		var next []any
		for _, node := range nodes {
			if s.array {
				array, ok := node.([]any)
				if !ok {
					return nil, false, fmt.Errorf("percurso JSONPath exige array")
				}
				if s.wildcard {
					next = append(next, array...)
				} else if s.index < len(array) {
					next = append(next, array[s.index])
				}
			} else {
				object, ok := node.(map[string]any)
				if !ok {
					return nil, false, fmt.Errorf("percurso JSONPath exige objeto")
				}
				if selected, ok := object[s.property]; ok {
					next = append(next, selected)
				}
			}
		}
		nodes = next
	}
	switch len(nodes) {
	case 0:
		return nil, false, nil
	case 1:
		return nodes[0], true, nil
	default:
		return nodes, true, nil
	}
}
