package query

import (
	"encoding/json"
	"testing"
)

// GO-397–400: a bound scalar never becomes expression source, and binding is immutable.
func TestBindFilterOperands(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   any
		actual  string
		present bool
	}{
		{"number", json.Number("9007199254740993"), `9007199254740993`, true},
		{"numeric string", "1", `1`, false},
		{"boolean", true, `true`, true}, {"null", nil, `null`, true},
		{"decimal", json.Number("1.00000000000000000002"), `1.00000000000000000001`, false},
		{"exponent", json.Number("1e999999999999999999999"), `10e999999999999999999998`, true},
		{"injection", `x')].id[?(@.v=='other`, `"x')].id[?(@.v=='other"`, true},
		{"nonrecursive", `{{missing}}`, `"{{missing}}"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := `$[?(@.v=={{v}})].id`
			plan, err := ParseWithOptions(source, true)
			if err != nil {
				t.Fatal(err)
			}
			bound, err := plan.Bind(func(s string, complete bool) (any, error) {
				if s != "{{v}}" || !complete {
					t.Fatalf("operand=%q complete=%v", s, complete)
				}
				return tc.value, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			value, present, err := bound.Select([]byte(`[{"v":` + tc.actual + `,"id":"selected"},{"v":"other","id":"wrong"}]`))
			if err != nil || present != tc.present || present && value != "selected" {
				t.Fatalf("value=%v present=%v err=%v", value, present, err)
			}
			if _, _, err := plan.Select([]byte(`[]`)); err == nil {
				t.Fatal("source plan was mutated or unresolved operand ignored")
			}
			if got := bound.RedactLiterals(source); got != `$[?(@.v==[literal oculto])].id` {
				t.Fatal(got)
			}
		})
	}
	for _, value := range []any{[]any{}, map[string]any{}} {
		plan, _ := ParseWithOptions(`$[?(@.v=={{v}})]`, true)
		if _, err := plan.Bind(func(string, bool) (any, error) { return value, nil }); err == nil {
			t.Fatalf("accepted %T", value)
		}
	}
}

// GO-401–403: opt-in never admits structural interpolation or a wider dialect.
func TestDynamicFilterValidation(t *testing.T) {
	for _, source := range []string{`{{path}}`, `$.{{field}}`, `$[{{index}}]`, `$[?(@.{{field}}==1)]`, `$[?(@.x {{op}} 1)]`, `$[?(@.x=={{v}}tail)]`, `$[?(@.x=={{broken)]`, `$[?(@.x=='{{bad-name}}')]`, `$[?(@.x!='{{v}}')]`} {
		if _, err := ParseWithOptions(source, true); err == nil {
			t.Errorf("accepted %s", source)
		}
	}
	// US-31 explicitly replaces US-28's permission for placeholder-looking literals.
	for _, source := range []string{`$[?(@.x=={{v}})]`, `$[?(@.x=='{{v}}')]`} {
		if _, err := Parse(source); err == nil {
			t.Errorf("accepted without opt-in: %s", source)
		}
	}
}
