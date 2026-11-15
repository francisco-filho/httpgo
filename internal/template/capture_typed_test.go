package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func captureString(value string) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func TestTypedCaptures(t *testing.T) {
	for _, raw := range []string{`""`, `"123"`, `"null"`, `"{{missing}}"`, `"quote\"\\\n雪"`, `9007199254740993`, `1.2300e+99`, `true`, `false`, `null`, `[]`, `{}`, `[9007199254740993,{"decimal":1.2300e+99,"literal":"{{missing}}"}]`, `{"items":[],"n":9007199254740993}`} {
		t.Run(raw, func(t *testing.T) {
			initial, _ := New(nil, []string{"x"})
			s := initial.NewSession()
			input := json.RawMessage(raw)
			if err := s.Publish([]Capture{{"x", input}}); err != nil {
				t.Fatal(err)
			}
			// Caller-owned buffers must not remain aliased to the session.
			for i := range input {
				input[i] = ' '
			}
			for _, wrapper := range [][2]string{{"", ""}, {`{"x":`, `}`}, {`[`, `]`}} {
				body, err := s.RenderJSON([]byte(wrapper[0] + "{{x}}" + wrapper[1]))
				if err != nil || string(body) != wrapper[0]+raw+wrapper[1] {
					t.Fatalf("got %s, %v", body, err)
				}
			}
			for _, source := range []string{"prefix{{x}}suffix", "http://example.test/{{x}}", "X-Value: {{x}}"} {
				text, err := s.Render(source)
				if raw[0] == '[' || raw[0] == '{' {
					var located *Error
					if !errors.As(err, &located) || located.Offset != strings.Index(source, "{{") {
						t.Fatalf("missing located rejection: %v", err)
					}
				} else {
					want := parseVariable(raw).text
					if err != nil || text.Value != strings.ReplaceAll(source, "{{x}}", want) {
						t.Fatalf("got %+v, %v", text, err)
					}
					body, err := s.RenderJSON([]byte(`"pre{{x}}"`))
					var got string
					if err != nil || json.Unmarshal(body, &got) != nil || got != "pre"+want {
						t.Fatalf("got %s, %v", body, err)
					}
				}
			}
			if _, err := s.Render("{{x.items}}"); err == nil {
				t.Fatal("object properties became variables")
			}
			if _, err := initial.NewSession().RenderJSON([]byte(`{{x}}`)); err == nil {
				t.Fatal("capture leaked")
			}
		})
	}
}

func TestInvalidCapturePayloadAtomicAndProtected(t *testing.T) {
	for _, bad := range []string{"", "not-json-sensitive", `"unterminated`, `null true`} {
		initial, _ := New(nil, []string{"secret"})
		s := initial.NewSession()
		raw := json.RawMessage(`{"privateKey":"hidden\"\\\n雪","id":9007199254740993}`)
		err := s.Publish([]Capture{{"secret", raw}, {"bad", json.RawMessage(bad)}})
		var invalid *CaptureValueError
		if !errors.As(err, &invalid) || invalid.Index != 1 || strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("got %v", err)
		}
		if _, err := s.RenderJSON([]byte(`{{secret}}`)); err == nil {
			t.Fatal("partial publication")
		}
		for _, variant := range SecretVariants(string(raw)) {
			if variant != "" && s.Sanitize(variant) != "[oculto]" {
				t.Fatalf("variant not protected: %q", variant)
			}
		}
		if initial.NewSession().Sanitize("privateKey") != "privateKey" {
			t.Fatal("protection leaked")
		}
	}
}

func TestTypedCaptureConflictProtection(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `9007199254740993`, `[]`, `{"s":"sensitive-part"}`} {
		for _, names := range [][]string{{"base"}, {"base.child"}, {"auth"}, {"auth.token.child"}, {"next", "next"}, {"next", "next.child"}, {"next.child", "next"}} {
			initial, _ := New([]string{`base={}`}, []string{"secret"})
			s := initial.NewSession()
			if err := s.Publish([]Capture{{"auth.token", json.RawMessage(raw)}}); err != nil {
				t.Fatal(err)
			}
			batch := []Capture{{"secret", json.RawMessage(`{"s":"sensitive-part"}`)}}
			for _, name := range names {
				batch = append(batch, Capture{name, json.RawMessage(raw)})
			}
			if err := s.Publish(batch); err == nil {
				t.Fatalf("accepted %v", names)
			}
			if _, err := s.RenderJSON([]byte(`{{secret}}`)); err == nil {
				t.Fatal("partial publication")
			}
			if bytes.Contains([]byte(s.Sanitize("sensitive-part")), []byte("sensitive-part")) {
				t.Fatal("secret exposed")
			}
			got, err := s.RenderJSON([]byte(`{{auth.token}}`))
			if err != nil || string(got) != raw {
				t.Fatalf("original overwritten: %s %v", got, err)
			}
		}
	}
}
