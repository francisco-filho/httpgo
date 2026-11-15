package template

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestCaptureStringsAndIsolation(t *testing.T) {
	initial, _ := New([]string{"base=initial"}, []string{"auth.token"})
	for _, value := range []string{"123", "null", "true", "", `{"a":1}`, `{{missing}}`, "quotes\"\\\n雪"} {
		s := initial.NewSession()
		if err := s.Publish([]Capture{{"auth.token", captureString(value)}}); err != nil {
			t.Fatal(err)
		}
		text, err := s.Render("{{auth.token}}")
		if err != nil || text.Value != value || text.Display != "[oculto]" {
			t.Fatalf("%q: %+v %v", value, text, err)
		}
		body, err := s.RenderJSON([]byte(`{"full":{{auth.token}},"text":"pre{{auth.token}}"}`))
		var got map[string]any
		if err != nil || json.Unmarshal(body, &got) != nil || got["full"] != value || got["text"] != "pre"+value {
			t.Fatalf("%q: %s %v", value, body, err)
		}
		if _, err := initial.Render("{{auth.token}}"); err == nil {
			t.Fatal("initial context mutated")
		}
		if _, err := initial.NewSession().Render("{{auth.token}}"); err == nil {
			t.Fatal("new session retained capture")
		}
	}
}

func TestCapturePublicationAtomicConflicts(t *testing.T) {
	for _, initialValue := range []string{"text", `null`, `[]`, `{"x":1}`} {
		for _, names := range [][]string{{"base"}, {"base.child"}, {"auth"}, {"auth.token.child"}, {"new", "new"}, {"new", "new.child"}, {"new.child", "new"}} {
			initial, _ := New([]string{"base=" + initialValue}, nil)
			s := initial.NewSession()
			if err := s.Publish([]Capture{{"auth.token", captureString("original")}}); err != nil {
				t.Fatal(err)
			}
			batch := []Capture{{"good", captureString("unpublished")}}
			for _, name := range names {
				batch = append(batch, Capture{name, captureString("replacement")})
			}
			var conflict *ConflictError
			if err := s.Publish(batch); !errors.As(err, &conflict) {
				t.Fatalf("%v: %v", names, err)
			}
			if _, err := s.Render("{{good}}"); err == nil {
				t.Fatal("partially published batch")
			}
			if got, _ := s.Render("{{auth.token}}"); got.Value != "original" {
				t.Fatal("capture overwritten")
			}
		}
	}
	c, _ := New(nil, []string{"token"})
	s := c.NewSession()
	s.ProtectCapture("token", captureString("discovered"))
	if strings.Contains(s.Sanitize("discovered"), "discovered") {
		t.Fatal("unpublished secret exposed")
	}
}
