package template

import (
	"encoding/json"
	"testing"
)

// GO-460–461,465,467: overlays preserve exact JSON and never mutate the base.
func TestStagedCaptureReplacement(t *testing.T) {
	for _, raw := range []string{`null`, `true`, `""`, `"{{missing}}"`, `9007199254740993`, `1.000000000000000000002`, `1e999999999999999999999`, `[]`, `{"child":[null]}`} {
		t.Run(raw, func(t *testing.T) {
			initial, _ := New([]string{`value="original"`}, []string{"value"})
			s := initial.NewSession()
			for range 2 { // Replace initial and then a previously confirmed capture.
				batch := []Capture{{"value", json.RawMessage(raw)}, {"fresh", json.RawMessage(`42`)}}
				view, err := s.Stage(batch)
				if err != nil {
					t.Fatal(err)
				}
				got, err := view.RenderJSON([]byte(`{{value}}`))
				if err != nil || string(got) != raw {
					t.Fatalf("got=%s err=%v want=%s", got, err, raw)
				}
				if _, err := view.Render("{{value.child}}"); err == nil {
					t.Fatal("object expanded into variables")
				}
				if _, err := s.Render("{{fresh}}"); err == nil {
					t.Fatal("staging published fresh variable")
				}
				if err := s.Publish(batch[:1]); err != nil {
					t.Fatal(err)
				}
			}
			got, err := s.RenderJSON([]byte(`{{value}}`))
			if err != nil || string(got) != raw {
				t.Fatalf("committed=%s err=%v", got, err)
			}
			original, _ := initial.Render("{{value}}")
			clean, _ := initial.NewSession().Render("{{value}}")
			if original.Value != "original" || clean.Value != "original" {
				t.Fatal("initial context mutated")
			}
			if !s.IsSecret("value") || s.Sanitize("original") != "[oculto]" {
				t.Fatal("replacement lost protection")
			}
		})
	}
}

func TestDiscardedOverlaySharesOnlyProtection(t *testing.T) {
	c, _ := New([]string{"value=old-private"}, []string{"value"})
	s := c.NewSession()
	view, err := s.Stage([]Capture{{"value", json.RawMessage(`"new-private"`)}, {"fresh", json.RawMessage(`true`)}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := s.ProtectionValues()
	view.ProtectValue("prefix-new-private-suffix")
	view.MarkSecret("fresh")
	if !s.IsSecret("fresh") || s.Sanitize("prefix-new-private-suffix") != "[oculto]" {
		t.Fatal("overlay protection not shared")
	}
	if Protector(snapshot)("prefix-new-private-suffix") == "[oculto]" {
		t.Fatal("snapshot was mutated")
	}
	old, _ := s.Render("{{value}}")
	if old.Value != "old-private" {
		t.Fatal("discard changed confirmed resolution")
	}
	if _, err := s.Render("{{fresh}}"); err == nil {
		t.Fatal("discard published fresh")
	}
	for _, bad := range [][]Capture{
		{{"value", json.RawMessage(`1`)}, {"fresh", json.RawMessage(`2`)}, {"fresh", json.RawMessage(`3`)}},
		{{"value", json.RawMessage(`1`)}, {"value.child", json.RawMessage(`2`)}},
		{{"value", json.RawMessage(`1`)}, {"fresh", json.RawMessage(`invalid`)}},
	} {
		if _, err := s.Stage(bad); err == nil {
			t.Fatal("invalid batch accepted")
		}
		got, _ := s.Render("{{value}}")
		if got.Value != "old-private" {
			t.Fatal("invalid batch partially applied")
		}
	}
}
