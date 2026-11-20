package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"capimgrosso.com/httpgo/internal/template"
)

// References retain only their name and source position, never source content.
type processReference struct {
	name, file   string
	line, column int
}

func parseProcessReference(file string, node *yaml.Node, key string) (processReference, error) {
	name, opened := strings.CutPrefix(node.Value, "${")
	name, closed := strings.CutSuffix(name, "}")
	if node.Tag != "!!str" || !opened || !closed || !template.ValidName(name) || strings.Contains(name, ".") {
		return processReference{}, failure(file, node, fmt.Sprintf("referência inválida em variables.%s; use somente ${NOME}, com nome [A-Za-z_][A-Za-z0-9_]*", key), nil)
	}
	return processReference{name: name, file: file, line: node.Line, column: node.Column}, nil
}

func rejectReferences(file string, node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode && strings.Contains(node.Value, "${") {
		return failure(file, node, "referências ${NOME} são suportadas somente como valor string completo de variables, fora de listas", nil)
	}
	for _, child := range node.Content {
		if err := rejectReferences(file, child); err != nil {
			return err
		}
	}
	return nil
}

// Resolve consults only effective references after exact CLI overrides. Available
// values are retained for protection even on failure, never for partial execution.
// It does not mutate its source and never enumerates the process environment.
func (s Settings) Resolve(ctx context.Context, assignments []string) (resolved Settings, err error) {
	defer func() { resolved.resolveErr = err }()
	if s.resolveErr != nil {
		return s, s.resolveErr
	}
	values := make(map[string]json.RawMessage, len(s.Variables))
	for name, value := range s.Variables {
		values[name] = value
	}
	s.Variables = values
	overridden := make(map[string]bool)
	for _, assignment := range assignments {
		name, _, ok := strings.Cut(assignment, "=")
		if ok {
			overridden[name] = true
		}
	}
	lookup := s.lookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	names := make([]string, 0, len(s.references))
	for name := range s.references {
		names = append(names, name)
	}
	sort.Strings(names)
	var firstErr error
	for _, key := range names {
		reference := s.references[key]
		position := &yaml.Node{Line: reference.line, Column: reference.column}
		if err := ctx.Err(); err != nil {
			return s, failure(reference.file, position, "resolução de referências cancelada", err)
		}
		if overridden[key] {
			delete(values, key)
			continue
		}
		value, present := lookup(reference.name)
		if !present {
			value, present = s.envValues[reference.name]
		}
		if !present {
			if firstErr == nil {
				sources := "no ambiente do processo"
				if s.envFile != "" {
					sources += " ou no arquivo selecionado por --env-file"
				}
				firstErr = failure(reference.file, position, fmt.Sprintf("referência ${%s} ausente para variables.%s; forneça a variável %s", reference.name, key, sources), nil)
			}
			continue
		}
		values[key], _ = json.Marshal(value)
	}
	if err := ctx.Err(); err != nil {
		return s, failure("", nil, "resolução de referências cancelada", err)
	}
	s.references = nil
	return s, firstErr
}
