package config

import (
	"strings"
	"testing"
)

func TestComposeExactOverrides(t *testing.T) {
	base, err := decode("project.yaml", []byte(`version: 1
variables:
  api: {baseUrl: http://base.test, version: v1}
  items: [1, 2]
  nullable: 42
  string: "001"
`))
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := base.Compose([]string{"api.baseUrl=http://override.test", "items=[3]", "nullable=null", "opaque={\"name\":\"Pizza\"}", "extra=true"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := renderer.NewSession().RenderJSON([]byte(`{"url":{{api.baseUrl}},"version":{{api.version}},"items":{{items}},"null":{{nullable}},"string":{{string}},"opaque":{{opaque}},"extra":{{extra}}}`))
	want := `{"url":"http://override.test","version":"v1","items":[3],"null":null,"string":"001","opaque":{"name":"Pizza"},"extra":true}`
	if err != nil || string(body) != want {
		t.Fatalf("body=%s want=%s err=%v", body, want, err)
	}
	if _, err := renderer.Render("{{opaque.name}}"); err == nil {
		t.Fatal("opaque property available")
	}
	if string(base.Variables["items"]) != "[1,2]" || string(base.Variables["api.baseUrl"]) != `"http://base.test"` {
		t.Fatal("base mutated")
	}
	for _, assignments := range [][]string{{"api=1"}, {"api.version.child=1"}, {"x=1", "x=2"}, {"x=1", "x.y=2"}, {"missingEquals"}, {"bad-name=1"}} {
		if _, err := base.Compose(assignments, nil); err == nil {
			t.Fatalf("accepted %q", assignments)
		}
	}
}

func TestProtectionUnionBeforeAndAfterCompose(t *testing.T) {
	base, err := decode("project.yaml", []byte(`version: 1
variables:
  token: base-fake-token
  viaCLI: base-fake-cli
secrets: [token, future]
`))
	if err != nil {
		t.Fatal(err)
	}
	assignments := []string{`token="override-fake\n雪"`, `viaCLI={"nested":["fake-object-secret"]}`}
	secrets := []string{"viaCLI"}
	renderer, err := base.Compose(assignments, secrets)
	if err != nil {
		t.Fatal(err)
	}
	session := renderer.NewSession()
	if !session.IsSecret("future") || !session.IsSecret("token") || !session.IsSecret("viaCLI") {
		t.Fatal("marks not accumulated")
	}
	for _, protect := range []func(string) string{base.Protection(assignments, secrets), renderer.Sanitize, session.Sanitize} {
		for _, value := range []string{"base-fake-token", "base-fake-cli", "override-fake\n雪", `override-fake\n雪`, "fake-object-secret"} {
			if text := protect("prefix-" + value + "-suffix"); strings.Contains(text, value) {
				t.Fatalf("leaked %q: %q", value, text)
			}
		}
	}
	if _, err := session.Render("{{future}}"); err == nil {
		t.Fatal("missing secret resolved")
	}
	if _, err := base.Compose([]string{"token=one", "token=two"}, nil); err == nil {
		t.Fatal("CLI duplicate accepted as override")
	}
	if text := base.Protection([]string{"token=fake-invalid-one", "token=fake-invalid-two"}, nil)("fake-invalid-one fake-invalid-two"); strings.Contains(text, "fake-invalid") {
		t.Fatal(text)
	}
}
