package query

import (
	"encoding/json"
	"testing"
)

// GO-408: dynamic filters and jp navigation preserve entity association and
// ordered multiplicity, reducing only at the very end without flattening arrays.
func TestDynamicFilterComposition(t *testing.T) {
	const body = `{"groups":[{"items":[{"meta":{"keys":["pizza"]},"active":false,"variants":[{"code":"x","ids":["p1","p2"]},{"code":"y","ids":["wrong"]}]},{"meta":{"keys":["juice"]},"active":true,"variants":[{"code":"x","ids":["wrong"]}]}]},{"items":[{"meta":{"keys":["pizza"]},"active":true,"variants":[{"code":"x","ids":["p1"]}]}]},{}]}`
	for _, tc := range []struct{ path, want string }{
		{`$.groups[*].items[?(@.meta.keys[0]=={{slug}})].active`, `[false,true]`},
		{`$.groups[*].items[?(@.meta.keys[0]=='{{slug}}')].variants[?(@.code=={{code}})].ids`, `[["p1","p2"],["p1"]]`},
		{`$.groups[*].items[?(@.meta.keys[0]=={{slug}})].variants[?(@.code=='{{code}}')].ids[*]`, `["p1","p2","p1"]`},
	} {
		plan, err := ParseWithOptions(tc.path, true)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		bound, err := plan.Bind(func(source string, _ bool) (any, error) {
			calls++
			if source == "{{slug}}" {
				return "pizza", nil
			}
			return "x", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		value, present, err := bound.Select([]byte(body))
		encoded, _ := json.Marshal(value)
		if err != nil || !present || string(encoded) != tc.want || calls < 1 {
			t.Fatalf("path=%s value=%s present=%v err=%v", tc.path, encoded, present, err)
		}
	}
}
