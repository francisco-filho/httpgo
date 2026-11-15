// Package config loads the explicitly supported HTTPGo YAML extension.
package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"capimgrosso.com/httpgo/internal/template"
)

const MaxBytes = 1048576
const DefaultTimeout = 10 * time.Second

// Settings contains exact variable names and canonical typed JSON values.
// A hierarchy is flattened only in variables, never inside array values.
type Settings struct {
	Variables map[string]json.RawMessage
	Secrets   []string
	Timeout   time.Duration
}

// Error preserves an I/O cause but displays only controlled diagnostics.
type Error struct {
	File         string
	Line, Column int
	Detail       string
	cause        error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%q:%d:%d: configuração: %s", e.File, e.Line, e.Column, e.Detail)
}
func (e *Error) Unwrap() error { return e.cause }

func failure(file string, node *yaml.Node, detail string, cause error) error {
	line, column := 1, 1
	if node != nil && node.Line > 0 {
		line, column = node.Line, node.Column
	}
	return &Error{File: file, Line: line, Column: column, Detail: detail, cause: cause}
}

// Load discovers only .httpgo.yaml in the current directory if path is empty.
func Load(ctx context.Context, path string) (Settings, error) {
	return load(ctx, path, func(name string) (io.ReadCloser, error) { return os.Open(name) })
}

func load(ctx context.Context, path string, open func(string) (io.ReadCloser, error)) (Settings, error) {
	empty := Settings{Variables: make(map[string]json.RawMessage), Timeout: DefaultTimeout}
	optional := path == ""
	if optional {
		path = ".httpgo.yaml"
	}
	if err := ctx.Err(); err != nil {
		return empty, failure(path, nil, "carga cancelada", err)
	}
	input, err := open(path)
	if err != nil {
		if optional && errors.Is(err, os.ErrNotExist) {
			return empty, nil
		}
		return empty, failure(path, nil, "não foi possível abrir o arquivo; verifique caminho e permissões", err)
	}
	data, readErr := io.ReadAll(io.LimitReader(input, MaxBytes+1))
	closeErr := input.Close()
	if err := errors.Join(readErr, closeErr, ctx.Err()); err != nil {
		return empty, failure(path, nil, "não foi possível ler e fechar o arquivo", err)
	}
	if len(data) > MaxBytes {
		return empty, failure(path, nil, "arquivo excede o limite de 1 MiB", nil)
	}
	settings, err := decode(path, data)
	if err != nil {
		return empty, err
	}
	return settings, nil
}

func decode(file string, data []byte) (Settings, error) {
	result := Settings{Variables: make(map[string]json.RawMessage), Timeout: DefaultTimeout}
	empty := true
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.Trim(line, " \t\r")
		if line != "" && !strings.HasPrefix(line, "#") {
			empty = false
			break
		}
	}
	if empty {
		return result, nil
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		return result, failure(file, nil, "YAML inválido", nil)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return result, failure(file, &extra, "é permitido somente um documento YAML", nil)
	}
	if err := validateTree(file, &document); err != nil {
		return result, err
	}
	if len(document.Content) != 1 {
		return result, failure(file, &document, "raiz deve ser um mapa com version: 1", nil)
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return result, failure(file, root, "raiz deve ser um mapa com version: 1", nil)
	}
	version := false
	for i := 0; i < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		switch key.Value {
		case "version":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!int" || value.Value != "1" {
				return result, failure(file, value, "version deve ser o inteiro 1", nil)
			}
			version = true
		case "variables":
			if value.Kind != yaml.MappingNode {
				return result, failure(file, value, "variables deve ser um mapa", nil)
			}
			if err := flatten(file, value, "", result.Variables); err != nil {
				return result, err
			}
		case "secrets":
			if value.Kind != yaml.SequenceNode {
				return result, failure(file, value, "secrets deve ser uma lista de nomes", nil)
			}
			for _, name := range value.Content {
				if name.Kind != yaml.ScalarNode || name.Tag != "!!str" || !template.ValidName(name.Value) {
					return result, failure(file, name, "nome inválido em secrets", nil)
				}
				result.Secrets = append(result.Secrets, name.Value)
			}
		case "defaults":
			if value.Kind != yaml.MappingNode {
				return result, failure(file, value, "defaults deve ser um mapa", nil)
			}
			for j := 0; j < len(value.Content); j += 2 {
				if value.Content[j].Value != "timeout" {
					return result, failure(file, value.Content[j], "campo não suportado em defaults", nil)
				}
				timeout, err := timeoutValue(value.Content[j+1])
				if err != nil {
					return result, failure(file, value.Content[j+1], "timeout deve ser número de segundos entre 0.001 e 86400, em milissegundos inteiros", nil)
				}
				result.Timeout = timeout
			}
		default:
			return result, failure(file, key, "campo não suportado na raiz", nil)
		}
	}
	if !version {
		return result, failure(file, root, "version: 1 é obrigatória", nil)
	}
	return result, nil
}

func validateTree(file string, node *yaml.Node) error {
	if node.Anchor != "" || node.Kind == yaml.AliasNode || node.Style&yaml.TaggedStyle != 0 {
		return failure(file, node, "âncoras, aliases e tags explícitas não são suportados", nil)
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return failure(file, key, "chave deve ser string", nil)
			}
			if seen[key.Value] {
				return failure(file, key, "chave repetida", nil)
			}
			seen[key.Value] = true
		}
	}
	if node.Kind == yaml.ScalarNode && bytes.Contains([]byte(node.Value), []byte("${")) {
		return failure(file, node, "referências ${NOME} ao ambiente do processo ainda não são suportadas", nil)
	}
	for _, child := range node.Content {
		if err := validateTree(file, child); err != nil {
			return err
		}
	}
	return nil
}

func flatten(file string, node *yaml.Node, prefix string, values map[string]json.RawMessage) error {
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		name := key.Value
		if prefix != "" {
			name = prefix + "." + name
		}
		if !template.ValidName(name) {
			return failure(file, key, "nome de variável inválido", nil)
		}
		if value.Kind == yaml.MappingNode {
			if err := flatten(file, value, name, values); err != nil {
				return err
			}
			continue
		}
		for existing := range values {
			if template.NamesConflict(name, existing) {
				return failure(file, key, "caminhos de variáveis repetidos ou em conflito", nil)
			}
		}
		converted, err := jsonValue(file, value)
		if err != nil {
			return err
		}
		raw, err := json.Marshal(converted)
		if err != nil {
			return failure(file, value, "valor não representável em JSON", nil)
		}
		values[name] = raw
	}
	return nil
}

// Assignments returns sorted canonical JSON assignments for template.New.
func (s Settings) Assignments() []string {
	names := make([]string, 0, len(s.Variables))
	for name := range s.Variables {
		names = append(names, name)
	}
	sort.Strings(names)
	assignments := make([]string, 0, len(names))
	for _, name := range names {
		assignments = append(assignments, name+"="+string(s.Variables[name]))
	}
	return assignments
}
