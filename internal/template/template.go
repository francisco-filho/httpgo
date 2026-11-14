// Package template resolves textual variables without recursive interpretation.
package template

import (
	"fmt"
	"sort"
	"strings"
)

// Error exposes controlled text and a byte offset, never source values or names.
type Error struct {
	Offset int
	Detail string
}

func (e *Error) Error() string { return e.Detail }

type Reference struct {
	Start, End int
	Name       string
}

func ValidName(name string) bool {
	for _, segment := range strings.Split(name, ".") {
		if segment == "" {
			return false
		}
		for i, c := range []byte(segment) {
			valid := c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (i > 0 && c >= '0' && c <= '9')
			if !valid {
				return false
			}
		}
	}
	return true
}

// Parse validates all delimiters before any resolution is attempted.
func Parse(source string) ([]Reference, error) {
	var refs []Reference
	for i := 0; i < len(source); {
		if strings.HasPrefix(source[i:], "}}") {
			return nil, &Error{i, "placeholder inválido: fechamento sem abertura"}
		}
		if !strings.HasPrefix(source[i:], "{{") {
			i++
			continue
		}
		start := i
		close := strings.Index(source[i+2:], "}}")
		if close < 0 {
			return nil, &Error{start, "placeholder inválido: fechamento ausente"}
		}
		end := i + 2 + close
		name := strings.Trim(source[i+2:end], " \t")
		if !ValidName(name) {
			return nil, &Error{start, "nome de variável inválido no placeholder"}
		}
		refs = append(refs, Reference{start, end + 2, name})
		i = end + 2
	}
	return refs, nil
}

// Text keeps operational content separate from a provenance-protected display.
type Text struct{ Value, Display string }

type Context struct {
	values  map[string]string
	secrets map[string]bool
}

func New(assignments, secrets []string) (*Context, error) {
	c := &Context{values: make(map[string]string), secrets: make(map[string]bool)}
	for _, name := range secrets {
		if !ValidName(name) {
			return nil, fmt.Errorf("nome inválido em --secret")
		}
		c.secrets[name] = true
	}
	for _, assignment := range assignments {
		name, value, ok := strings.Cut(assignment, "=")
		if !ok {
			return nil, fmt.Errorf("--var exige nome=valor")
		}
		if !ValidName(name) {
			return nil, fmt.Errorf("nome inválido em --var")
		}
		for existing := range c.values {
			if name == existing {
				return nil, fmt.Errorf("variável repetida em --var")
			}
			if strings.HasPrefix(name, existing+".") || strings.HasPrefix(existing, name+".") {
				return nil, fmt.Errorf("conflito entre variável escalar e descendente")
			}
		}
		c.values[name] = value
	}
	return c, nil
}

func (c *Context) Render(source string) (Text, error) {
	refs, err := Parse(source)
	if err != nil {
		return Text{}, err
	}
	var value, display strings.Builder
	last := 0
	for _, ref := range refs {
		text, ok := c.values[ref.Name]
		if !ok {
			return Text{}, &Error{ref.Start, "variável ausente; forneça seu valor com --var"}
		}
		value.WriteString(source[last:ref.Start])
		value.WriteString(text)
		display.WriteString(source[last:ref.Start])
		if c.secrets[ref.Name] {
			display.WriteString("[oculto]")
		} else {
			display.WriteString(text)
		}
		last = ref.End
	}
	value.WriteString(source[last:])
	display.WriteString(source[last:])
	return Text{Value: value.String(), Display: c.Sanitize(display.String())}, nil
}

// Sanitize protects known nonempty values before URL escaping/normalization.
// Longest matches first avoid exposing suffixes of overlapping secret values.
func (c *Context) Sanitize(text string) string {
	var values []string
	for name := range c.secrets {
		if value := c.values[name]; value != "" {
			values = append(values, value)
		}
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	for _, value := range values {
		text = strings.ReplaceAll(text, value, "[oculto]")
	}
	return text
}
