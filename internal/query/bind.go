package query

import (
	"encoding/json"
	"fmt"
	"slices"

	"capimgrosso.com/httpgo/internal/template"
)

// filterOperand contains validated source only; substituted content never enters
// the JSONPath parser. A complete reference is typed, a composed literal is text.
type filterOperand struct {
	source     string
	complete   bool
	references []template.Reference
}

// HasProtectedReference inspects validated provenance without resolving operands.
// It lets presentation conceal pending secret captures without evaluating them.
func (p Path) HasProtectedReference(protected func(string) bool) bool {
	for _, s := range p.steps {
		if s.filter == nil || s.filter.operand == nil {
			continue
		}
		for _, ref := range s.filter.operand.references {
			if protected(ref.Name) {
				return true
			}
		}
	}
	return false
}

// Bind resolves every dynamic operand before selection, including operands in
// unreachable branches. It returns a detached plan without mutating the source.
// The consumer owns resolution, secret provenance and lifetime of bound values.
func (p Path) Bind(resolve func(source string, complete bool) (any, error)) (Path, error) {
	bound := p
	bound.steps = slices.Clone(p.steps)
	for i, s := range p.steps {
		if s.filter == nil || s.filter.operand == nil {
			continue
		}
		f := *s.filter
		value, err := resolve(f.operand.source, f.operand.complete)
		if err != nil {
			return Path{}, err
		}
		switch value.(type) {
		case nil, string, bool, json.Number:
		default:
			return Path{}, fmt.Errorf("operando de filtro JSONPath exige valor escalar")
		}
		f.literal, f.operand = value, nil
		bound.steps[i].filter = &f
	}
	return bound, nil
}
