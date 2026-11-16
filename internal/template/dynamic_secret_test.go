package template

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDynamicSecretDeclarationsAreSessionLocal(t *testing.T) {
	context, err := New(nil, []string{"explicit"})
	if err != nil {
		t.Fatal(err)
	}
	first := context.NewSession()
	first.MarkSecret("captured")
	value := `quote"\\/雪{{missing}}`
	raw, _ := json.Marshal(value)
	first.ProtectCapture("captured", raw)
	if err := first.Publish([]Capture{{Name: "captured", Value: raw}}); err != nil {
		t.Fatal(err)
	}
	rendered, err := first.Render("prefix{{captured}}suffix")
	if err != nil || rendered.Value != "prefix"+value+"suffix" || strings.Contains(rendered.Display, value) || !strings.Contains(rendered.Display, "[oculto]") {
		t.Fatalf("render=%+v err=%v", rendered, err)
	}
	for _, session := range []Session{context, context.NewSession()} {
		if session.IsSecret("captured") || !session.IsSecret("explicit") || strings.Contains(session.Sanitize(value), "[oculto]") {
			t.Fatal("dynamic protection escaped session")
		}
		if _, err := session.Render("{{captured}}"); err == nil {
			t.Fatal("capture escaped session")
		}
	}
}
