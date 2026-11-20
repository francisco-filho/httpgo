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

func writeEnvFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "local.env")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvFileLiteralProfile(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"comments", " \r\n # comment\r\n\t", map[string]string{}},
		{"spaces and CRLF", " A = value \r\n_B2=\r\nS='  inner  '\r\nD=\" spaced \"\r\n", map[string]string{"A": "value", "_B2": "", "S": "  inner  ", "D": " spaced "}},
		{"literal", "A=a=b#c\nB=${OTHER}\nC=$(command)\nD=\\n\nE={{other}}\nF=# text\nG='double\"quote'\nH=\"single'quote\"", map[string]string{"A": "a=b#c", "B": "${OTHER}", "C": "$(command)", "D": `\n`, "E": "{{other}}", "F": "# text", "G": "double\"quote", "H": "single'quote"}},
		{"large line", "A=" + strings.Repeat("v", MaxBytes+1), map[string]string{"A": strings.Repeat("v", MaxBytes+1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := (Settings{}).WithEnvFile(context.Background(), writeEnvFile(t, tc.source))
			if err != nil || !reflect.DeepEqual(s.envValues, tc.want) || len(s.Variables) != 0 {
				t.Fatalf("profile %s: err=%v keys=%d", tc.name, err, len(s.envValues))
			}
		})
	}
}

func TestEnvFileInvalid(t *testing.T) {
	for _, line := range []string{"A=one", "A=fiction", "no-assignment", "=fiction", "1BAD=fiction", "bad.name=fiction", "bad-name=fiction", "Á=fiction", "export B=fiction", "B='fiction", `B="fiction`, `B="fiction" tail`, `B='fiction' # comment`, "B='first\nsecond'"} {
		t.Run(line, func(t *testing.T) {
			s, err := (Settings{}).WithEnvFile(context.Background(), writeEnvFile(t, "A=one\n"+line))
			var diagnostic *Error
			if !errors.As(err, &diagnostic) || diagnostic.Line != 2 || diagnostic.Column != 1 || len(s.envValues) != 0 || strings.Contains(diagnostic.Detail, "fiction") || strings.Contains(diagnostic.Detail, "first") {
				t.Fatalf("invalid input not rejected safely: %v", err)
			}
			if _, err := s.Compose(nil, nil); err == nil {
				t.Fatal("published context after invalid file")
			}
		})
	}
}

func TestEnvFileReferencePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		present     bool
	}{
		{"fallback", "from-file", false}, {"process", "from-process", true}, {"empty process", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := referenceSettings(t, "variables: {token: '${OWNER}', sibling: true}")
			s, err := s.WithEnvFile(context.Background(), writeEnvFile(t, "OWNER=from-file\nEXTRA=unused\nHTTPGO_TIMEOUT=0.001\nNO_COLOR=1"))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			s.lookupEnv = func(name string) (string, bool) {
				calls++
				if name != "OWNER" {
					t.Fatalf("unexpected lookup %q", name)
				}
				return tc.value, tc.present
			}
			resolved, err := s.Resolve(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			renderer, err := resolved.Compose(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			got, err := renderer.Render("{{token}}/{{sibling}}")
			if err != nil || got.Value != tc.value+"/true" || calls != 1 || !renderer.NewSession().IsSecret("token") {
				t.Fatalf("precedence: value=%q calls=%d err=%v", got.Value, calls, err)
			}
			if _, err := renderer.Render("{{EXTRA}}"); err == nil {
				t.Fatal("extra key imported")
			}
			// An exact CLI override eliminates lookup, not protection.
			calls = 0
			renderer, err = s.Compose([]string{"token=substitute-fiction"}, nil)
			if err != nil || calls != 0 || !renderer.NewSession().IsSecret("token") {
				t.Fatalf("override calls=%d err=%v", calls, err)
			}
			if len(s.references) != 1 || s.Variables["token"] != nil {
				t.Fatal("source mutated")
			}
		})
	}
}

// Faults never depend on filesystem permissions or the executing user's UID.
type envFileReader struct {
	io.Reader
	readErr, closeErr error
	closed            int
	onRead            func()
}

func (r *envFileReader) Read(p []byte) (int, error) {
	if r.onRead != nil {
		r.onRead()
	}
	n, err := r.Reader.Read(p)
	if errors.Is(err, io.EOF) && r.readErr != nil {
		err = r.readErr
	}
	return n, err
}

func (r *envFileReader) Close() error { r.closed++; return r.closeErr }

func TestEnvFileResources(t *testing.T) {
	cause := errors.New("private-file-content")
	for _, mode := range []string{"open", "partial read", "close", "syntax and close", "cancel before", "cancel reading", "cancel closing"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader := &envFileReader{Reader: strings.NewReader("A=fiction\nB=second")}
			wantCause := cause
			switch mode {
			case "partial read":
				reader.readErr = cause
			case "close", "syntax and close":
				reader.closeErr = cause
				if mode == "syntax and close" {
					reader.Reader = strings.NewReader("A=fiction\nbad")
				}
			case "cancel before":
				cancel()
				wantCause = context.Canceled
			case "cancel reading":
				reader.onRead = cancel
				wantCause = context.Canceled
			case "cancel closing":
				reader.closeErr = context.Canceled
				wantCause = context.Canceled
			}
			opens := 0
			s, err := (Settings{}).withEnvFile(ctx, "local.env", func(string) (io.ReadCloser, error) {
				opens++
				if mode == "open" {
					return nil, cause
				}
				return reader, nil
			})
			wantClosed := 1
			if mode == "open" || mode == "cancel before" {
				wantClosed = 0
			}
			if !errors.Is(err, wantCause) || len(s.envValues) != 0 || reader.closed != wantClosed || strings.Contains(err.Error(), cause.Error()) {
				t.Fatalf("mode=%s closed=%d opens=%d err=%v", mode, reader.closed, opens, err)
			}
		})
	}
}

func TestEnvFileMissingReference(t *testing.T) {
	s := referenceSettings(t, "variables: {unused: '${ABSENT}', token: '${OWNER}'}")
	s, err := s.WithEnvFile(context.Background(), writeEnvFile(t, "OWNER=fiction"))
	if err != nil {
		t.Fatal(err)
	}
	s.lookupEnv = func(string) (string, bool) { return "", false }
	resolved, err := s.Resolve(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "--env-file") || !strings.Contains(err.Error(), "ABSENT") || resolved.Protection(nil, nil)("fiction") != "[oculto]" {
		t.Fatalf("missing reference: %v", err)
	}
	if _, err := resolved.Compose(nil, nil); err == nil {
		t.Fatal("published partial context")
	}
}
