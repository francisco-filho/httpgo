package config

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEnvironmentSelection(t *testing.T) {
	for _, name := range []string{"local", "Staging_01-x", "0", "", ".", "../x", "a/b", `a\b`, "a.b", "a b", "é"} {
		t.Run(name, func(t *testing.T) {
			valid := name == "local" || name == "Staging_01-x" || name == "0"
			if ValidEnvironmentName(name) != valid {
				t.Fatalf("name=%q valid=%v", name, valid)
			}
			var opened []string
			s, err := loadEnvironment(context.Background(), "project/base.yaml", name, func(path string) (io.ReadCloser, error) {
				opened = append(opened, path)
				if path == "project/base.yaml" {
					return io.NopCloser(strings.NewReader("")), nil
				}
				return io.NopCloser(strings.NewReader("variables: {price: 4500}")), nil
			})
			if name == "" {
				if err != nil || len(opened) != 1 {
					t.Fatalf("opened=%v err=%v", opened, err)
				}
				return
			}
			if !valid {
				if err == nil || len(opened) != 1 {
					t.Fatalf("opened=%v err=%v", opened, err)
				}
				return
			}
			want := []string{"project/base.yaml", filepath.Join("project", "environments", name+".yaml")}
			if err != nil || !reflect.DeepEqual(opened, want) || string(s.Variables["price"]) != "4500" {
				t.Fatalf("opened=%v settings=%+v err=%v", opened, s, err)
			}
		})
	}
}

func TestEnvironmentDocuments(t *testing.T) {
	for _, source := range []string{"", " \t\n", "# comment", "variables: {a: 1}", "version: 1\nvariables: {a: 1}"} {
		if _, err := decodeDocument("local.yaml", []byte(source), true); err != nil {
			t.Fatalf("source=%q err=%v", source, err)
		}
	}
	for _, source := range []string{"version: 2", "version: '1'", "[]", "---", "variables: null", "variables: {a: 'prefix-${TOKEN}'}", "variables: {a: &x 1}", "variables: {a: !!str 1}", "variables: {a: 1, a: 2}", "variables: {bad-name: 1}", "unknown: hidden-fixture", "variables: [", "variables: {}\n---\n"} {
		if _, err := decodeDocument("local.yaml", []byte(source), true); err == nil || strings.Contains(err.Error(), "hidden-fixture") {
			t.Fatalf("source=%q err=%v", source, err)
		}
	}
}

func TestEnvironmentReadFailuresAndLimit(t *testing.T) {
	sentinel := errors.New("hidden-fixture")
	for _, mode := range []string{"missing", "permission", "read", "close", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input := &failingInput{readErr: io.EOF}
			s, err := loadEnvironment(ctx, "base.yaml", "local", func(path string) (io.ReadCloser, error) {
				if path == "base.yaml" {
					return io.NopCloser(strings.NewReader("version: 1\nvariables: {base: true}")), nil
				}
				switch mode {
				case "missing":
					return nil, os.ErrNotExist
				case "permission":
					return nil, os.ErrPermission
				case "read":
					input.readErr = sentinel
				case "close":
					input.closeErr = sentinel
				case "canceled":
					cancel()
				}
				return input, nil
			})
			if err == nil || len(s.Variables) != 1 || strings.Contains(err.Error(), "hidden-fixture") {
				t.Fatalf("settings=%+v err=%v", s, err)
			}
			if mode != "missing" && mode != "permission" && !input.closed {
				t.Fatal("environment not closed")
			}
		})
	}
	for _, prefix := range []string{"#", "variables: {}\n#"} {
		for _, extra := range []int{0, 1} {
			source := prefix + strings.Repeat("x", MaxBytes-len(prefix)+extra)
			_, err := loadEnvironment(context.Background(), "base.yaml", "local", func(path string) (io.ReadCloser, error) {
				if path == "base.yaml" {
					return io.NopCloser(strings.NewReader("")), nil
				}
				return io.NopCloser(strings.NewReader(source)), nil
			})
			if (err != nil) != (extra == 1) {
				t.Fatalf("size=%d err=%v", len(source), err)
			}
		}
	}
}
