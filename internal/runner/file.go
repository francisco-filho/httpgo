package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"capimgrosso.com/httpgo/internal/parser"
)

// FileError exposes a controlled cause, never the operational path or payload.
// The original filesystem error remains available through errors.Is/As.
type FileError struct {
	Position parser.Position
	Detail   string
	cause    error
}

func (e *FileError) Error() string { return fmt.Sprintf("erro de arquivo externo: %s", e.Detail) }
func (e *FileError) Unwrap() error { return e.cause }

var errPayloadDirectory = errors.New("payload is a directory")

func fileFailure(pos parser.Position, err error) *FileError {
	detail := "falha ao ler ou fechar o arquivo; verifique o arquivo e as permissões"
	switch {
	case errors.Is(err, context.Canceled):
		detail = "leitura cancelada"
	case errors.Is(err, context.DeadlineExceeded):
		detail = "prazo de leitura excedido"
	case errors.Is(err, fs.ErrNotExist):
		detail = "arquivo inexistente; verifique a referência no cenário"
	case errors.Is(err, fs.ErrPermission):
		detail = "acesso negado; verifique a permissão de leitura"
	case errors.Is(err, errPayloadDirectory):
		detail = "referência aponta para diretório; informe um arquivo legível"
	}
	return &FileError{Position: pos, Detail: detail, cause: err}
}

func readPayload(ctx context.Context, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if info.IsDir() {
		return nil, errors.Join(errPayloadDirectory, file.Close())
	}
	return readAndClosePayload(ctx, file)
}

// Local reads are synchronous; context checks cannot interrupt a blocked OS read.
// Always close before returning, including after a partial read or cancellation.
func readAndClosePayload(ctx context.Context, file io.ReadCloser) ([]byte, error) {
	var body []byte
	err := ctx.Err()
	if err == nil {
		body, err = io.ReadAll(file)
	}
	closeErr := file.Close()
	if err = errors.Join(err, closeErr, ctx.Err()); err != nil {
		return nil, err
	}
	return body, nil
}
