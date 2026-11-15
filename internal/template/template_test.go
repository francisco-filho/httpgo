package template

import (
	"strings"
	"testing"
)

func TestContextText(t *testing.T) {
	c, err := New([]string{"api.baseUrl=http://example.test", "empty=", "eq=a=b=c", "num=42", "bool=true", "raw={{missing}}", "token=fake /?%"}, []string{"token", "future", "empty"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Render("{{ api.baseUrl }}/{{empty}}/{{eq}}/{{num}}/{{ bool }}-{{raw}}-{{token}}")
	if err != nil || got.Value != "http://example.test//a=b=c/42/true-{{missing}}-fake /?%" || strings.Contains(got.Display, "fake") {
		t.Fatalf("render=%+v, err=%v", got, err)
	}
	if _, err := c.Render("{{future}}"); err == nil {
		t.Fatal("missing secret resolved")
	}
	if got := c.Sanitize("plain text"); got != "plain text" {
		t.Fatalf("empty secret corrupted text: %q", got)
	}
}

func TestInvalidContext(t *testing.T) {
	for _, args := range [][]string{{"missing"}, {"=value"}, {"a..b=x"}, {"1a=x"}, {"é=x"}, {"a-b=x"}, {"a=x", "a=x"}, {"a=x", "a=y"}, {"a=x", "a.b=y"}, {"a.b=x", "a=y"}} {
		if _, err := New(args, nil); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
	if _, err := New(nil, []string{"bad name"}); err == nil {
		t.Fatal("accepted invalid secret")
	}
	for _, name := range []string{"a", "_", "a0._B", "api.baseUrl"} {
		if !ValidName(name) {
			t.Fatalf("rejected %q", name)
		}
	}
}

func TestSyntaxAndOffsets(t *testing.T) {
	for _, text := range []string{"xx{{bad-name}}", "xx{{ }}", "xx{{name", "xx}}", "xx{{{{a}}}}", "xx{{a b}}"} {
		_, err := Parse(text)
		located, ok := err.(*Error)
		if !ok || located.Offset != 2 || strings.Contains(err.Error(), "bad-name") {
			t.Fatalf("Parse(%q)=%v", text, err)
		}
	}
	refs, err := Parse("x{{ a.b }}{{c}}")
	if err != nil || len(refs) != 2 || refs[0] != (Reference{1, 10, "a.b"}) {
		t.Fatalf("refs=%+v, err=%v", refs, err)
	}
}

func FuzzRender(f *testing.F) {
	for _, seed := range []string{"{{ x }}", "{{x}}{{x}}", "{{", "}}", "{{invalid-name}}", "plain"} {
		f.Add(seed, "{{notRecursive}}")
	}
	for _, value := range []string{`"001"`, `"line\n雪"`, `null`, `true`, `9007199254740993123456789`, `[]`, `{"x":1}`} {
		f.Add("{{x}}", value)
	}
	f.Fuzz(func(t *testing.T, source, value string) {
		c, err := New([]string{"x=" + value}, []string{"x"})
		if err != nil {
			t.Fatal(err)
		}
		s := c.NewSession()
		if err := s.Publish([]Capture{{"captured", value}}); err != nil {
			t.Fatal(err)
		}
		if got, err := s.Render("{{captured}}"); err != nil || got.Value != value {
			t.Fatal("capture was reinterpreted")
		}
		_, err = c.Render(source)
		if err != nil && strings.Contains(err.Error(), "notRecursive") {
			t.Fatal("unsafe error")
		}
	})
}
