package template

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProtectionValuesDetachedFromOperationalContext(t *testing.T) {
	initial, err := New([]string{"base=public"}, []string{"token"})
	if err != nil {
		t.Fatal(err)
	}
	first := initial.NewSession()
	value := json.RawMessage(`{"nested":["fake-secret",9007199254740993],"empty":""}`)
	first.ProtectCapture("token", value)
	if err := first.Publish([]Capture{{Name: "token", Value: value}}); err != nil {
		t.Fatal(err)
	}
	first.ProtectValue("Bearer fake-secret")
	snapshot := first.ProtectionValues()
	protect := Protector(append([]string(nil), snapshot...))
	for _, text := range []string{"fake-secret", "9007199254740993", "Bearer fake-secret"} {
		if strings.Contains(protect(text), text) {
			t.Fatalf("unprotected %q", text)
		}
	}
	if protect("ordinary text") != "ordinary text" {
		t.Fatal("empty secret damaged output")
	}
	if len(snapshot) == 0 {
		t.Fatal("missing variants")
	}
	snapshot[0] = "mutation"
	if strings.Contains(first.Sanitize("fake-secret"), "fake-secret") {
		t.Fatal("snapshot aliases session")
	}
	second := initial.NewSession()
	if _, err := second.Render("{{token}}"); err == nil {
		t.Fatal("capture crossed sessions")
	}
	if second.Sanitize("fake-secret") != "fake-secret" {
		t.Fatal("snapshot mutated initial protection")
	}
	rendered, err := second.Render("{{base}}")
	if err != nil || rendered.Value != "public" {
		t.Fatalf("initial value=%+v err=%v", rendered, err)
	}
}
