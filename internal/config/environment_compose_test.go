package config

import (
	"strings"
	"testing"
	"time"
)

func environmentSettings(t *testing.T, base, env string) (Settings, error) {
	t.Helper()
	b, err := decode("base.yaml", []byte(base))
	if err != nil {
		t.Fatal(err)
	}
	e, err := decodeDocument("local.yaml", []byte(env), true)
	if err != nil {
		t.Fatal(err)
	}
	return mergeEnvironment(b, e)
}

func TestEnvironmentTypedComposition(t *testing.T) {
	s, err := environmentSettings(t, `version: 1
variables:
  api: {baseUrl: http://base.test, version: v1}
  product: {price: 4500, active: true, code: "001"}
  items: [1, 2]
  nullable: 42
  replaced: null
`, `variables:
  api: {baseUrl: http://env.test}
  product: {price: 9007199254740993}
  items: [null, {a: 1.2500}]
  nullable: null
  replaced: "123"
`)
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := s.Compose([]string{"api.baseUrl=http://cli.test", `opaque={"name":"Pizza"}`}, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := renderer.NewSession().RenderJSON([]byte(`{"url":{{api.baseUrl}},"version":{{api.version}},"price":{{product.price}},"active":{{product.active}},"code":{{product.code}},"items":{{items}},"null":{{nullable}},"replaced":{{replaced}},"opaque":{{opaque}}}`))
	want := `{"url":"http://cli.test","version":"v1","price":9007199254740993,"active":true,"code":"001","items":[null,{"a":1.2500}],"null":null,"replaced":"123","opaque":{"name":"Pizza"}}`
	if err != nil || string(body) != want {
		t.Fatalf("body=%s want=%s err=%v", body, want, err)
	}
	if _, err := renderer.Render("{{opaque.name}}"); err == nil {
		t.Fatal("CLI object became hierarchy")
	}
}

func TestEnvironmentStructuralConflicts(t *testing.T) {
	for _, tc := range []struct{ base, env string }{
		{"a: 1", "a: {b: 2}"}, {"a: {b: 2}", "a: null"}, {"a: [1]", "a.b: 2"}, {"a.b: 1", "a: []"},
		{"a: {}", "a: 1"}, {"a: 1", "a: {}"}, {"a.b: {}", "a.b: null"},
	} {
		s, err := environmentSettings(t, "version: 1\nvariables: {"+tc.base+"}", "variables: {"+tc.env+"}")
		if err == nil || !strings.Contains(err.Error(), "local.yaml") || !strings.Contains(err.Error(), "conflito") {
			t.Fatalf("base=%s env=%s err=%v", tc.base, tc.env, err)
		}
		// No partial override was applied even if CLI could replace the leaf.
		if _, ok := s.Variables["a.b"]; ok && tc.base == "a: 1" {
			t.Fatal("partial merge")
		}
	}
	for _, source := range []string{"a: {}, a: 1", "a: 1, a.b: {}", "a.b: {}, a: 1", "a.b: {}, a: {b: {}}"} {
		if _, err := decodeDocument("local.yaml", []byte("variables: {"+source+"}"), true); err == nil {
			t.Fatalf("accepted %s", source)
		}
	}
	for _, source := range []string{"a: {}", "a: {b: {}}", "a.b: 1"} {
		s, err := environmentSettings(t, "version: 1", "variables: {"+source+"}")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Compose([]string{"a=1"}, nil); err == nil {
			t.Fatalf("CLI hid map: %s", source)
		}
	}
}

func TestEnvironmentTimeoutsAndEmptyOverrides(t *testing.T) {
	for _, tc := range []struct {
		base, env string
		want      time.Duration
	}{
		{"", "", DefaultTimeout}, {"defaults: {timeout: 2.5}", "", 2500 * time.Millisecond},
		{"defaults: {timeout: 2.5}", "defaults: {timeout: 0.001}", time.Millisecond},
		{"", "defaults: {timeout: 86400}", 24 * time.Hour},
	} {
		s, err := environmentSettings(t, "version: 1\n"+tc.base, tc.env)
		if err != nil || s.Timeout != tc.want {
			t.Fatalf("case=%+v settings=%+v err=%v", tc, s, err)
		}
	}
	for _, value := range []string{"0", "86400.001", "0.0001", "1.0001", "'2s'", ".inf", "null"} {
		if _, err := decodeDocument("local.yaml", []byte("defaults: {timeout: "+value+"}"), true); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
	for _, env := range []string{"", " \n# comment", "secrets: []\nvariables: {}"} {
		s, err := environmentSettings(t, "version: 1\nvariables: {token: base-fake}\nsecrets: [token]", env)
		if err != nil || s.Protection(nil, nil)("base-fake") != "[oculto]" {
			t.Fatalf("env=%q err=%v", env, err)
		}
	}
}

func TestEnvironmentProtectionAllSources(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		env := "variables: {token: 'env-fake', fromBase: 'env-replaced'}\nsecrets: [token, future]"
		if conflict {
			env = "variables: {token: 'env-fake', fromBase: 'env-replaced', structure: {child: 1}}\nsecrets: [token, future]"
		}
		s, err := environmentSettings(t, "version: 1\nvariables: {token: base-fake, fromBase: base-replaced, viaCLI: base-cli-fake, structure: 1}\nsecrets: [fromBase]", env)
		if (err != nil) != conflict {
			t.Fatalf("conflict=%v err=%v", conflict, err)
		}
		assignments := []string{`token="cli-fake\n雪"`, `viaCLI=cli-replaced`}
		protectors := []func(string) string{s.Protection(assignments, []string{"viaCLI"})}
		if !conflict {
			renderer, err := s.Compose(assignments, []string{"viaCLI"})
			if err != nil {
				t.Fatal(err)
			}
			session := renderer.NewSession()
			if !session.IsSecret("future") || !session.IsSecret("fromBase") {
				t.Fatal("lost marks")
			}
			if _, err := session.Render("{{future}}"); err == nil {
				t.Fatal("mark supplied a value")
			}
			protectors = append(protectors, renderer.Sanitize, session.Sanitize)
		}
		for _, protect := range protectors {
			for _, secret := range []string{"base-fake", "env-fake", "base-replaced", "env-replaced", "base-cli-fake", "cli-replaced", "cli-fake\n雪", `cli-fake\n雪`} {
				if strings.Contains(protect("prefix "+secret+" suffix"), secret) {
					t.Fatalf("conflict=%v leaked=%q", conflict, secret)
				}
			}
		}
	}
}
