package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
	"capimgrosso.com/httpgo/internal/template"
)

func TestInvocationProvisionalProtectionAndResolution(t *testing.T) {
	c, _ := template.New([]string{"token=old-private"}, []string{"token"})
	protection := &executionProtection{initial: func(s string) string { return s }}
	renderer := sessionRenderer{c, protection}
	session := renderer.NewSession()
	for _, valid := range []bool{false, true} {
		batch := []template.Capture{{Name: "token", Value: json.RawMessage(`"new-private"`)}, {Name: "fresh", Value: json.RawMessage(`42`)}}
		if !valid {
			batch = append(batch, template.Capture{Name: "fresh", Value: json.RawMessage(`43`)})
		}
		view, err := session.Stage(batch)
		if (err == nil) != valid {
			t.Fatalf("valid=%v err=%v", valid, err)
		}
		if valid {
			value, _ := view.Render("{{token}}")
			if value.Value != "new-private" {
				t.Fatalf("view=%+v", value)
			}
			view.ProtectValue("prefix-new-private-suffix")
			view.MarkSecret("fresh")
		}
		if _, err := session.Render("{{fresh}}"); err == nil {
			t.Fatal("view published variable")
		}
		old, _ := session.Render("{{token}}")
		if old.Value != "old-private" {
			t.Fatalf("discarded overlay changed session=%+v", old)
		}
	}
	for _, secret := range []string{"old-private", "new-private", "prefix-new-private-suffix", "42"} {
		if protection.sanitize(secret) != "[oculto]" {
			t.Fatalf("lost protection %q", secret)
		}
	}
	fresh := renderer.NewSession()
	value, _ := fresh.Render("{{token}}")
	if value.Value != "old-private" || fresh.IsSecret("fresh") {
		t.Fatal("file session shared resolution/declarations")
	}
	if fresh.Sanitize("new-private") != "[oculto]" {
		t.Fatal("invocation lost discarded protection")
	}
}

func TestCaptureRenewalWriterFailures(t *testing.T) {
	t.Setenv("HTTPGO_RENEWAL_SECRET", "old-private")
	config := writeConfig(t, "version: 1\nvariables:\n  token: ${HTTPGO_RENEWAL_SECRET}\n")
	for _, mode := range []string{"success", "mismatch", "capture error"} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/short=%v", mode, short), func(t *testing.T) {
				source := "GET http://example.test\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\n"
				if mode == "capture error" {
					source += "broken: jsonpath \"$.missing\"\n"
				}
				source += "[Asserts]\nheader \"X\" == \"prefix-{{token}}-suffix\"\nGET http://example.test/never"
				calls := 0
				command := NewRootCommand(executorFunc(func(context.Context, runner.Request) (runner.Response, error) {
					calls++
					value := "prefix-new-private-suffix"
					if mode == "mismatch" {
						value = "unrelated-private"
					}
					return runner.Response{Status: 200, Body: []byte(`{"token":"new-private"}`), Headers: []query.HeaderValue{{Name: "X", Value: value}}}, nil
				}), BuildInfo{})
				command.SetArgs([]string{"test", writeScenario(t, source), "--config", config, "--render-asserts"})
				var stderr bytes.Buffer
				command.SetErr(&stderr)
				writer := &failAfterWriter{cause: errors.New("writer new-private")}
				if short {
					command.SetOut(shortWriter{})
				} else {
					command.SetOut(writer)
				}
				code := command.ExecuteContext(context.Background())
				if code != 2 || calls != 1 || !short && writer.calls != 1 || !strings.Contains(stderr.String(), "escrever o resultado") {
					t.Fatalf("code=%d calls=%d stderr=%s", code, calls, &stderr)
				}
				for _, secret := range []string{"old-private", "new-private", "unrelated-private"} {
					if strings.Contains(stderr.String(), secret) {
						t.Fatalf("writer failure exposed %s", secret)
					}
				}
			})
		}
	}
}
