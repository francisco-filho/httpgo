package config

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestTypedHierarchy(t *testing.T) {
	settings, err := decode("project.yaml", []byte(`version: 1
variables:
  api:
    baseUrl: http://localhost:8080
  product:
    price: 9007199254740993
    code: "001"
    enabled: true
    nothing: null
    items: [1.2500, {name: Pizza, nested: [false, null]}]
  escaped: "quote\"\n雪"
  hex: 0xff
  octal: 0755
  fraction: .5
secrets: [future.token]
`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"api.baseUrl": `"http://localhost:8080"`, "product.price": "9007199254740993", "product.code": `"001"`, "product.enabled": "true", "product.nothing": "null", "product.items": `[1.2500,{"name":"Pizza","nested":[false,null]}]`, "escaped": `"quote\"\n雪"`, "hex": "255", "octal": "493", "fraction": "0.5"}
	if len(settings.Variables) != len(want) {
		t.Fatalf("variables=%v", settings.Variables)
	}
	for name, value := range want {
		if string(settings.Variables[name]) != value {
			t.Errorf("%s got=%s want=%s", name, settings.Variables[name], value)
		}
	}
	if settings.Timeout != 10*time.Second || !reflect.DeepEqual(settings.Secrets, []string{"future.token"}) {
		t.Fatalf("settings=%+v", settings)
	}
	for _, assignment := range settings.Assignments() {
		_, value, _ := strings.Cut(assignment, "=")
		if !json.Valid([]byte(value)) {
			t.Fatal(assignment)
		}
	}
}

func TestRejectedDocuments(t *testing.T) {
	cases := []struct{ name, source, detail string }{
		{"malformed", "version: [", "YAML inválido"},
		{"invalid comment encoding", "#\xff", "YAML inválido"},
		{"missing version", "variables: {}", "version: 1"},
		{"version", "version: 2", "version"},
		{"version string", "version: '1'", "version"},
		{"unknown", "version: 1\nprivate-fixture: hidden-fixture", "campo"},
		{"root list", "[1,2]", "raiz"},
		{"null document", "---\n", "raiz"},
		{"two documents", "version: 1\n---\nversion: 1", "somente um documento"},
		{"empty second document", "version: 1\n---", "somente um documento"},
		{"duplicate", "version: 1\nversion: 1", "repetida"},
		{"nested duplicate", "version: 1\nvariables: {a: [{b: 1, b: 2}]}", "repetida"},
		{"variables scalar", "version: 1\nvariables: x", "variables"},
		{"bad name", "version: 1\nvariables: {bad-name: hidden-fixture}", "nome"},
		{"flatten duplicate", "version: 1\nvariables: {a.b: 1, a: {b: 2}}", "conflito"},
		{"descendant", "version: 1\nvariables: {a: 1, a.b: 2}", "conflito"},
		{"ancestor", "version: 1\nvariables: {a.b: 2, a: 1}", "conflito"},
		{"date", "version: 1\nvariables: {a: 2026-01-01}", "tipo"},
		{"infinite", "version: 1\nvariables: {a: .inf}", "finito"},
		{"nan", "version: 1\nvariables: {a: .nan}", "finito"},
		{"key number", "version: 1\nvariables: {a: [{1: x}]}", "chave"},
		{"key huge number", "version: 1\nvariables: {a: [{1e400: x}]}", "chave"},
		{"secret type", "version: 1\nsecrets: x", "lista"},
		{"secret name", "version: 1\nsecrets: [bad-name]", "nome"},
		{"defaults type", "version: 1\ndefaults: null", "defaults"},
		{"defaults field", "version: 1\ndefaults: {retries: 3}", "campo"},
		{"environment", "version: 1\nvariables: {a: 'prefix-${TOKEN}'}", "referências"},
		{"environment nested", "version: 1\nvariables: {a: [{b: '${TOKEN}'}]}", "referências"},
		{"anchor", "version: 1\nvariables: {a: &a hidden-fixture}", "âncoras"},
		{"alias", "version: 1\nvariables: {a: &a [1], b: *a}", "âncoras"},
		{"tag", "version: 1\nvariables: {a: !!str hidden-fixture}", "tags"},
		{"nested tag", "version: 1\nvariables: {a: [{b: !external hidden-fixture}]}", "tags"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decode("safe.yaml", []byte(tc.source))
			var diagnostic *Error
			if !errors.As(err, &diagnostic) || !strings.Contains(err.Error(), tc.detail) || diagnostic.Line < 1 || diagnostic.Column < 1 {
				t.Fatalf("got=%v want=%s", err, tc.detail)
			}
			if strings.Contains(err.Error(), "hidden-fixture") || strings.Contains(err.Error(), "private-fixture") {
				t.Fatalf("leak=%v", err)
			}
		})
	}
}

func TestOptionalEmptyAndLimit(t *testing.T) {
	for _, source := range []string{"", " \t\n", "# comments\n"} {
		s, err := decode("empty.yaml", []byte(source))
		if err != nil || len(s.Variables) != 0 || s.Timeout != DefaultTimeout {
			t.Fatalf("source=%q settings=%+v err=%v", source, s, err)
		}
	}
	for _, base := range []string{"version: 1\n#", "#"} {
		for _, extra := range []int{0, 1} {
			source := base + strings.Repeat("x", MaxBytes-len(base)+extra)
			s, err := load(context.Background(), "selected.yaml", func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(source)), nil })
			if (err != nil) != (extra == 1) || s.Timeout != DefaultTimeout {
				t.Fatalf("size=%d err=%v", len(source), err)
			}
		}
	}
	missing := func(string) (io.ReadCloser, error) { return nil, os.ErrNotExist }
	if _, err := load(context.Background(), "", missing); err != nil {
		t.Fatal(err)
	}
	if _, err := load(context.Background(), "explicit.yaml", missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
	path := filepath.Join(t.TempDir(), "project.yaml")
	if err := os.WriteFile(path, []byte("version: 1"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(context.Background(), path); err != nil {
		t.Fatal(err)
	}
}

type failingInput struct {
	readErr, closeErr error
	closed            bool
}

func (r *failingInput) Read(p []byte) (int, error) {
	return copy(p, "version: 1\nvariables: {a: 1}"), r.readErr
}
func (r *failingInput) Close() error { r.closed = true; return r.closeErr }

func TestResourceFailures(t *testing.T) {
	sentinel := errors.New("hidden-fixture")
	for _, mode := range []string{"open", "read", "close", "cancel after read", "already canceled"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input := &failingInput{readErr: io.EOF}
			opened := false
			if mode == "read" {
				input.readErr = sentinel
			}
			if mode == "close" {
				input.closeErr = sentinel
			}
			if mode == "already canceled" {
				cancel()
			}
			settings, err := load(ctx, "safe.yaml", func(string) (io.ReadCloser, error) {
				opened = true
				if mode == "open" {
					return nil, sentinel
				}
				if mode == "cancel after read" {
					cancel()
				}
				return input, nil
			})
			if err == nil || len(settings.Variables) != 0 || strings.Contains(err.Error(), "hidden-fixture") {
				t.Fatalf("settings=%+v err=%v", settings, err)
			}
			if mode == "already canceled" {
				if opened {
					t.Fatal("opened canceled input")
				}
			} else if mode != "open" && !input.closed {
				t.Fatal("not closed")
			}
			if mode == "open" || mode == "read" || mode == "close" {
				if !errors.Is(err, sentinel) {
					t.Fatalf("cause lost: %v", err)
				}
			}
		})
	}
}
