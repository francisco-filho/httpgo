package template

import "testing"

func TestSecretDeclarationDoesNotResolveCapture(t *testing.T) {
	c, err := New(nil, []string{"future"})
	if err != nil {
		t.Fatal(err)
	}
	s := c.NewSession()
	if !s.IsSecret("future") || s.IsSecret("other") {
		t.Fatal("secret declaration lost")
	}
	if _, err := s.Render("{{future}}"); err == nil {
		t.Fatal("inspection published a capture")
	}
}
