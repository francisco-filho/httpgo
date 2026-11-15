package config

import (
	"strings"

	"capimgrosso.com/httpgo/internal/template"
)

// Compose validates CLI declarations independently, then replaces exact names.
// It never merges opaque CLI objects or permits ancestor/descendant collisions.
func (s Settings) Compose(assignments, secrets []string) (*template.Context, error) {
	if _, err := template.New(assignments, secrets); err != nil {
		return nil, err
	}
	values := make(map[string]string, len(s.Variables)+len(assignments))
	for name, value := range s.Variables {
		values[name] = string(value)
	}
	for _, assignment := range assignments {
		name, value, _ := strings.Cut(assignment, "=")
		values[name] = value
	}
	effective := make([]string, 0, len(values))
	for name, value := range values {
		effective = append(effective, name+"="+value)
	}
	context, err := template.New(effective, append(append([]string(nil), s.Secrets...), secrets...))
	if err != nil {
		return nil, err
	}
	for _, value := range s.protectedValues(assignments, secrets) {
		context.ProtectValue(value)
	}
	return context, nil
}

// Protection intentionally works before argument/context validation, including
// overridden base values and malformed CLI declarations for protected names.
func (s Settings) Protection(assignments, secrets []string) func(string) string {
	var variants []string
	for _, value := range s.protectedValues(assignments, secrets) {
		variants = append(variants, template.SecretVariants(value)...)
	}
	return template.Protector(variants)
}

func (s Settings) protectedValues(assignments, secrets []string) []string {
	marked := make(map[string]bool)
	for _, name := range s.Secrets {
		marked[name] = true
	}
	for _, name := range secrets {
		marked[name] = true
	}
	var values []string
	for name, value := range s.Variables {
		if marked[name] {
			values = append(values, string(value))
		}
	}
	for _, assignment := range assignments {
		name, value, ok := strings.Cut(assignment, "=")
		if ok && marked[name] {
			values = append(values, value)
		}
	}
	return values
}
