package config

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

func TestEnvFileProtectionAndSnapshots(t *testing.T) {
	s := referenceSettings(t, "variables: {token: '${AVAILABLE}', missing: '${ABSENT}'}")
	const value = "fiction-value\"雪<&"
	path := writeEnvFile(t, "AVAILABLE='"+value+"'")
	s.lookupEnv = func(string) (string, bool) { return "", false }
	prepared, err := s.WithEnvFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := prepared.Resolve(context.Background(), nil)
	if err == nil {
		t.Fatal("missing reference accepted")
	}
	for _, variant := range template.SecretVariants(value) {
		if variant != "" && strings.Contains(resolved.Protection(nil, nil)(variant), variant) {
			t.Fatal("variant leaked before missing reference diagnostic")
		}
	}
	if _, err := resolved.Compose(nil, nil); err == nil {
		t.Fatal("partial context published")
	}
	// Overrides discard references but not their destination marks.
	renderer, err := prepared.Compose([]string{"missing=non-secret-fiction"}, []string{"future"})
	if err != nil {
		t.Fatal(err)
	}
	session := renderer.NewSession()
	for _, name := range []string{"token", "missing", "future"} {
		if !session.IsSecret(name) {
			t.Fatalf("lost mark %s", name)
		}
	}
	capture, _ := json.Marshal("derived-" + value)
	if err := session.Publish([]template.Capture{{Name: "future", Value: capture}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(session.Sanitize("derived-"+value), value) || strings.Contains(renderer.Sanitize("non-secret-fiction"), "non-secret-fiction") {
		t.Fatal("substitute or derived value leaked")
	}
	if len(s.envValues) != 0 || s.envFile != "" || len(prepared.references) != 2 {
		t.Fatal("source mutated by preparation or resolution")
	}
}
