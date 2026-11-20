package config

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"capimgrosso.com/httpgo/internal/template"
)

// WithEnvFile prepares an explicit, invocation-local fallback for references.
// The complete file is validated and closed before any values are published.
func (s Settings) WithEnvFile(ctx context.Context, path string) (Settings, error) {
	return s.withEnvFile(ctx, path, func(name string) (io.ReadCloser, error) { return os.Open(name) })
}

func (s Settings) withEnvFile(ctx context.Context, path string, open func(string) (io.ReadCloser, error)) (Settings, error) {
	values, err := loadEnvFile(ctx, path, open)
	if err != nil {
		s.resolveErr = err
		return s, err
	}
	s.envFile, s.envValues = path, values
	return s, nil
}

func envFileFailure(path string, line int, detail string, cause error) error {
	return &Error{File: path, Line: line, Column: 1, Detail: detail, cause: cause}
}

func loadEnvFile(ctx context.Context, path string, open func(string) (io.ReadCloser, error)) (values map[string]string, err error) {
	if err := ctx.Err(); err != nil {
		return nil, envFileFailure(path, 1, "carga de --env-file cancelada", err)
	}
	input, err := open(path)
	if err != nil {
		return nil, envFileFailure(path, 1, "não foi possível abrir --env-file; verifique caminho e permissões", err)
	}
	defer func() {
		if cause := errors.Join(input.Close(), ctx.Err()); cause != nil {
			err = errors.Join(err, envFileFailure(path, 1, "não foi possível concluir e fechar --env-file", cause))
		}
		if err != nil {
			values = nil
		}
	}()
	values = make(map[string]string)
	reader := bufio.NewReader(input)
	for line := 1; ; line++ {
		if err := ctx.Err(); err != nil {
			return nil, envFileFailure(path, line, "carga de --env-file cancelada", err)
		}
		text, readErr := reader.ReadString('\n') // No scanner token or YAML size limit.
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, envFileFailure(path, line, "não foi possível ler --env-file", readErr)
		}
		if err := parseEnvLine(path, line, text, values); err != nil {
			return nil, err
		}
		if errors.Is(readErr, io.EOF) {
			return values, nil
		}
	}
}

func parseEnvLine(path string, line int, text string, values map[string]string) error {
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "#") {
		return nil
	}
	name, value, assigned := strings.Cut(text, "=")
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	detail := ""
	switch {
	case !assigned:
		detail = "atribuição inválida em --env-file; use NOME=valor"
	case !template.ValidName(name) || strings.Contains(name, "."):
		detail = "nome inválido em --env-file; use [A-Za-z_][A-Za-z0-9_]*, sem export"
	default:
		if _, duplicate := values[name]; duplicate {
			detail = "nome repetido em --env-file"
		}
	}
	if detail != "" {
		return envFileFailure(path, line, detail, nil)
	}
	if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
		end := strings.IndexByte(value[1:], value[0])
		if end < 0 || end+2 != len(value) {
			return envFileFailure(path, line, "aspas inválidas em --env-file; delimite todo o valor, sem texto após a aspa final", nil)
		}
		value = value[1 : len(value)-1]
	}
	values[name] = value
	return nil
}
