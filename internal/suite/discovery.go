// Package suite discovers scenarios before executing any HTTP request.
package suite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/runner"
)

// File separates the operational absolute path from its normalized display name.
type File struct{ Path, Name string }

type scenarioFile interface {
	io.ReadCloser
	Stat() (fs.FileInfo, error)
}

// Suite owns filesystem test seams per instance, never process-wide.
type Suite struct {
	runner  *runner.Runner
	open    func(string) (scenarioFile, error)
	lstat   func(string) (fs.FileInfo, error)
	readDir func(string) ([]os.DirEntry, error)
	now     func() time.Time
	protect func(string) string
}

func New(r *runner.Runner) *Suite {
	return NewWithProtection(r, func(text string) string { return text })
}

// NewWithProtection applies invocation-scoped protection before collection and
// callbacks. Operational paths and request values are never rewritten.
func NewWithProtection(r *runner.Runner, protect func(string) string) *Suite {
	return &Suite{runner: r, open: func(path string) (scenarioFile, error) { return os.Open(path) }, lstat: os.Lstat, readDir: os.ReadDir, now: time.Now, protect: protect}
}

type protectedError struct {
	message string
	cause   error
}

func (e *protectedError) Error() string { return e.message }
func (e *protectedError) Unwrap() error { return e.cause }

func (s *Suite) protectError(err error) error {
	if err == nil {
		return nil
	}
	return &protectedError{message: s.protect(err.Error()), cause: err}
}

// Error preserves the I/O cause without echoing operating-system error messages.
type Error struct {
	File, Detail string
	cause        error
}

func (e *Error) Error() string { return fmt.Sprintf("%q: %s", e.File, e.Detail) }
func (e *Error) Unwrap() error { return e.cause }

func displayName(cwd, absolute string) string {
	relative, err := filepath.Rel(cwd, absolute)
	if err != nil {
		relative = absolute
	}
	return filepath.ToSlash(relative)
}

// checkExplicit inspects uncleaned components too: link/../file still crosses a
// link and must not become accepted as a side effect of filepath.Clean.
func (s *Suite) checkExplicit(ctx context.Context, cwd, path string) error {
	if !filepath.IsAbs(path) {
		path = cwd + string(filepath.Separator) + path
	}
	volume := filepath.VolumeName(path)
	current := volume + string(filepath.Separator)
	components := strings.FieldsFunc(strings.TrimPrefix(path, volume), func(r rune) bool { return r == '/' || r == rune(filepath.Separator) })
	if len(path) > 0 && os.IsPathSeparator(path[len(path)-1]) {
		components = append(components, ".")
	}
	for i, component := range components {
		if err := ctx.Err(); err != nil {
			return err
		}
		current = filepath.Join(current, component)
		info, err := s.lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("caminho atravessa link simbólico")
		}
		if !info.IsDir() && i < len(components)-1 {
			return fmt.Errorf("componente não é diretório")
		}
	}
	return nil
}

type candidate struct {
	File
	info fs.FileInfo
}

// Discover finishes all access checks before returning a plan. It never parses
// scenarios or follows symbolic links. This is not a concurrent-filesystem sandbox.
func (s *Suite) Discover(ctx context.Context, paths []string) (plan []File, discoveryErr error) {
	defer func() { discoveryErr = s.protectError(discoveryErr) }()
	cwd, err := os.Getwd()
	if err != nil {
		return nil, &Error{Detail: "não foi possível determinar o diretório corrente", cause: err}
	}
	var candidates []candidate
	for _, path := range paths {
		if err := s.checkExplicit(ctx, cwd, path); err != nil {
			return nil, &Error{File: path, Detail: "caminho inválido, inacessível ou atravessando link simbólico", cause: err}
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, &Error{File: path, Detail: "caminho inválido", cause: err}
		}
		before := len(candidates)
		if err := s.discover(ctx, cwd, absolute, true, &candidates); err != nil {
			return nil, err
		}
		if before == len(candidates) {
			return nil, &Error{File: displayName(cwd, absolute), Detail: "diretório sem arquivos .hurl elegíveis"}
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	var unique []candidate
	for _, file := range candidates {
		duplicate := false
		for _, existing := range unique {
			if os.SameFile(existing.info, file.info) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			unique = append(unique, file)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, &Error{Detail: "descoberta cancelada", cause: err}
	}
	files := make([]File, len(unique))
	for i, file := range unique {
		files[i] = file.File
	}
	return files, nil
}

func (s *Suite) discover(ctx context.Context, cwd, path string, explicit bool, files *[]candidate) error {
	name := displayName(cwd, path)
	if err := ctx.Err(); err != nil {
		return &Error{File: name, Detail: "descoberta cancelada", cause: err}
	}
	info, err := s.lstat(path)
	if err != nil {
		return &Error{File: name, Detail: "não foi possível inspecionar a entrada", cause: err}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	if info.IsDir() {
		entries, err := s.readDir(path)
		if err != nil {
			return &Error{File: name, Detail: "não foi possível ler o diretório; verifique a permissão de acesso", cause: err}
		}
		for _, entry := range entries {
			if err := s.discover(ctx, cwd, filepath.Join(path, entry.Name()), false, files); err != nil {
				return err
			}
		}
		return nil
	}
	if !explicit && filepath.Ext(path) != ".hurl" {
		return nil
	}
	if !info.Mode().IsRegular() {
		return &Error{File: name, Detail: "entrada deve ser um arquivo regular legível"}
	}
	input, err := s.open(path)
	if err != nil {
		return &Error{File: name, Detail: "não foi possível abrir o arquivo; verifique o caminho e a permissão de leitura", cause: err}
	}
	if err := input.Close(); err != nil {
		return &Error{File: name, Detail: "não foi possível fechar o arquivo", cause: err}
	}
	*files = append(*files, candidate{File{path, name}, info})
	return nil
}

// RunFile loads and closes the entire scenario before calling the runner. A
// partial parse, including a late read/close failure, never reaches HTTP.
func (s *Suite) RunFile(ctx context.Context, file File, options runner.Options) (runner.Result, error) {
	scenario, err := s.load(ctx, file, parser.ParseOptions{RenderAsserts: options.RenderAsserts})
	if err != nil {
		return runner.Result{}, s.protectError(err)
	}
	result := s.runner.RunWithOptions(ctx, scenario, options)
	result.ErrorPosition.File = s.protect(result.ErrorPosition.File)
	// Keep typed safe diagnostics/cause inspection while protecting Error() at
	// the collection boundary, including cancellation before any callback.
	result.Err = s.protectRunError(result.Err)
	return result, nil
}

func (s *Suite) protectRunError(err error) error {
	if err == nil {
		return nil
	}
	detail := "falha de execução HTTP"
	var renderErr *runner.RenderError
	var fileErr *runner.FileError
	var captureErr *runner.CaptureError
	var jsonErr *runner.JSONError
	var bodyErr *runner.BodyError
	var headerErr *runner.HeaderError
	switch {
	case errors.As(err, &headerErr):
		detail = headerErr.Error()
	case errors.As(err, &bodyErr):
		detail = bodyErr.Error()
	case errors.As(err, &renderErr):
		detail = renderErr.Error()
	case errors.As(err, &fileErr):
		detail = fileErr.Error()
	case errors.As(err, &captureErr):
		detail = captureErr.Error()
	case errors.As(err, &jsonErr):
		detail = jsonErr.Error()
	case errors.Is(err, context.Canceled):
		detail = "execução cancelada"
	case errors.Is(err, context.DeadlineExceeded):
		detail = "prazo da requisição excedido"
	}
	return &protectedError{message: s.protect(detail), cause: err}
}

func (s *Suite) load(ctx context.Context, file File, options parser.ParseOptions) (parser.Scenario, error) {
	if err := ctx.Err(); err != nil {
		return parser.Scenario{}, &Error{File: file.Name, Detail: "leitura do cenário cancelada", cause: err}
	}
	input, err := s.open(file.Path)
	if err != nil {
		return parser.Scenario{}, &Error{File: file.Name, Detail: "não foi possível abrir o cenário", cause: err}
	}
	info, statErr := input.Stat()
	if statErr != nil || !info.Mode().IsRegular() {
		closeErr := input.Close()
		return parser.Scenario{}, &Error{File: file.Name, Detail: "cenário deve ser um arquivo regular legível", cause: errors.Join(statErr, closeErr)}
	}
	scenario, parseErr := parser.ParseWithOptions(s.protect(file.Name), input, options)
	closeErr := input.Close()
	if closeErr != nil {
		return parser.Scenario{}, errors.Join(parseErr, &Error{File: file.Name, Detail: "não foi possível fechar o cenário", cause: closeErr})
	}
	if parseErr != nil {
		return parser.Scenario{}, parseErr
	}
	if err := ctx.Err(); err != nil {
		return parser.Scenario{}, &Error{File: file.Name, Detail: "leitura do cenário cancelada", cause: err}
	}
	scenario.OriginDir = filepath.Dir(file.Path)
	return scenario, nil
}
