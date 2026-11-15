package config

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func referenceSettings(t *testing.T, source string) Settings {
	t.Helper()
	s, err := decode("base.yaml", []byte("version: 1\n"+source))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestProcessReferenceStrings(t *testing.T) {
	for _, value := range []string{"123", "true", "null", "", "a\"雪\n\\", "${OTHER}", "{{other}}", `{"key":true}`} {
		t.Run(value, func(t *testing.T) {
			s := referenceSettings(t, "variables: {auth: {token: '${_Owner1}'}, items: [1, null, {x: true}]}")
			calls := 0
			s.lookupEnv = func(name string) (string, bool) {
				calls++
				if name != "_Owner1" {
					t.Fatalf("unexpected lookup %q", name)
				}
				return value, true
			}
			resolved, err := s.Resolve(context.Background(), nil)
			if err != nil || calls != 1 || len(s.references) != 1 || len(resolved.references) != 0 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
			renderer, err := resolved.Compose(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, err := renderer.NewSession().RenderJSON([]byte(`{"token":{{auth.token}},"items":{{items}}}`))
			var got struct {
				Token string
				Items []any
			}
			if err != nil || json.Unmarshal(body, &got) != nil || got.Token != value || !renderer.NewSession().IsSecret("auth.token") {
				t.Fatalf("string type/content or mark lost; err=%v", err)
			}
			if calls != 1 {
				t.Fatalf("second expansion: calls=%d", calls)
			}
		})
	}
}

func TestProcessReferencePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, environment, cli, want, lookup string
	}{
		{"base", "", "", "base-value", "BASE"},
		{"environment value", "variables: {token: env-value}", "", "env-value", ""},
		{"environment reference", "variables: {token: '${ENV}'}", "", "env-value", "ENV"},
		{"CLI", "variables: {token: '${ENV}'}", "token=cli-fiction", "cli-fiction", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := environmentSettings(t, "version: 1\nvariables: {token: '${BASE}', sibling: true}", tc.environment)
			if err != nil {
				t.Fatal(err)
			}
			var calls []string
			s.lookupEnv = func(name string) (string, bool) {
				calls = append(calls, name)
				if name == "BASE" {
					return "base-value", true
				}
				return "env-value", true
			}
			var assignments []string
			if tc.cli != "" {
				assignments = []string{tc.cli}
			}
			renderer, err := s.Compose(assignments, nil)
			if err != nil {
				t.Fatal(err)
			}
			text, err := renderer.Render("{{token}}/{{sibling}}")
			if err != nil || text.Value != tc.want+"/true" || strings.Contains(text.Display, tc.want) {
				t.Fatalf("value/mark lost: err=%v", err)
			}
			var wantCalls []string
			if tc.lookup != "" {
				wantCalls = []string{tc.lookup}
			}
			if !reflect.DeepEqual(calls, wantCalls) {
				t.Fatalf("calls=%v want=%v", calls, wantCalls)
			}
		})
	}
}

func TestProcessReferenceValidationPerSource(t *testing.T) {
	for _, value := range []string{"Bearer ${TOKEN}", "${A}${B}", "${}", "${ TOKEN}", "${A B}", "${1A}", "${a.b}", "${A-B}", "${Á}", "${A", "${A}}", "${A}suffix"} {
		_, err := decode("invalid.yaml", []byte("version: 1\nvariables:\n  token: '"+value+"'"))
		var diagnostic *Error
		if !errors.As(err, &diagnostic) || diagnostic.Line != 3 || diagnostic.Column != 10 || !strings.Contains(err.Error(), "variables.token") {
			t.Fatalf("invalid reference %q: %v", value, err)
		}
	}
	for _, source := range []string{
		"variables: {token: ['${TOKEN}']}",
		"variables: {token: [{x: ['${TOKEN}']}]}",
		"variables: {token: [{'${TOKEN}': 1}]}",
		"defaults: {timeout: '${TOKEN}'}", "secrets: ['${TOKEN}']",
	} {
		if _, err := decode("invalid.yaml", []byte("version: 1\n"+source)); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}
	for _, name := range []string{"a", "A", "_", "_abc1", "Owner_TOKEN2"} {
		referenceSettings(t, "variables: {token: '${"+name+"}'}")
	}
	// A valid override cannot hide an invalid declaration in either YAML source.
	for _, env := range []bool{false, true} {
		base, overlay := "version: 1\nvariables: {token: 'Bearer ${TOKEN}'}", "variables: {token: fiction}"
		if env {
			base, overlay = "version: 1\nvariables: {token: fiction}", "variables: {token: ['${TOKEN}']}"
		}
		b, err := decode("base.yaml", []byte(base))
		if err == nil {
			_, err = decodeDocument("local.yaml", []byte(overlay), true)
		}
		if err == nil {
			t.Fatalf("override hid invalid source: %v", b)
		}
	}
}

func TestProcessReferenceMissingAndCancellation(t *testing.T) {
	s := referenceSettings(t, "variables: {unused: '${ABSENT}', token: '${PRESENT}'}")
	s.lookupEnv = func(name string) (string, bool) { return "fiction", name == "PRESENT" }
	resolved, err := s.Resolve(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "ABSENT") || !strings.Contains(err.Error(), "variables.unused") || !strings.Contains(err.Error(), "base.yaml") {
		t.Fatalf("err=%v", err)
	}
	if _, err := s.Compose(nil, nil); err == nil {
		t.Fatal("published partial context")
	}
	if resolved.Protection(nil, nil)("fiction") != "[oculto]" {
		t.Fatal("lost available value")
	}
	for _, during := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		s.lookupEnv = func(string) (string, bool) { calls++; cancel(); return "fiction", true }
		if !during {
			cancel()
		}
		_, err := s.Resolve(ctx, nil)
		cancel()
		if !errors.Is(err, context.Canceled) || calls > 1 || !during && calls != 0 {
			t.Fatalf("during=%v calls=%d err=%v", during, calls, err)
		}
	}
}
