package template

import (
	"strings"
	"testing"
)

func TestKnownSecretVariants(t *testing.T) {
	for _, value := range []string{`"fictional\"\\\n雪"`, `9007199254740993123456789`, `true`, `null`, `["fictional\nvalue",987654321]`, `{"privateKey":"fictional\nvalue"}`} {
		c, err := New([]string{"x=" + value}, []string{"x"})
		if err != nil {
			t.Fatal(err)
		}
		for _, variant := range SecretVariants(value) {
			if variant != "" && strings.Contains(c.Sanitize("prefix "+variant+" suffix"), variant) {
				t.Fatalf("unprotected known variant %q of %q", variant, value)
			}
		}
	}
	c, _ := New([]string{`x=""`}, []string{"x"})
	if got := c.Sanitize("ordinary text"); got != "ordinary text" {
		t.Fatalf("empty protection corrupted text: %q", got)
	}
	if got := Protector([]string{"secret", "secret longer"})("secret longer secret"); got != "[oculto] [oculto]" {
		t.Fatalf("overlap=%q", got)
	}
}
