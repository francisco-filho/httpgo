package query

import (
	"encoding/json"
	"testing"
)

// GO-347–348: alternate jp navigation and equality without intermediate reduction,
// entity mixing, sorting, deduplication or flattening selected arrays.
func TestFilterCompositionAndCardinality(t *testing.T) {
	const catalog = `{"groups":[{"products":[{"meta":{"keys":["pizza"]},"active":false,"variants":[{"code":"x","ids":["p1","p2"]},{"code":"y","ids":["wrong"]}]},{"meta":{"keys":["juice"]},"active":true,"variants":[{"code":"x","ids":["wrong"]}]}]},{"products":[{"meta":{"keys":["pizza"]},"active":true,"variants":[{"code":"x","ids":["p1"]}]}]},{}]}`
	for _, tc := range []struct {
		path, body, want string
		present          bool
	}{
		{`$.groups[*].products[?(@.meta.keys[0]=='pizza')].active`, catalog, `[false,true]`, true},
		{`$.groups[*].products[?(@.meta.keys[0]=='pizza')].variants[?(@.code=='x')].ids`, catalog, `[["p1","p2"],["p1"]]`, true},
		{`$.groups[*].products[?(@.meta.keys[0]=='pizza')].variants[?(@.code=='x')].ids[*]`, catalog, `["p1","p2","p1"]`, true},
		{`$[?(@.x==true)].v`, `[]`, `null`, false},
		{`$[?(@.x==true)].v`, `[{"x":false,"v":1}]`, `null`, false},
		{`$[?(@.x==true)].v`, `[{"x":true}]`, `null`, false},
		{`$[?(@.x==true)].v`, `[{"x":true,"v":null}]`, `null`, true},
		{`$[?(@.x==true)].v`, `[{"x":true,"v":[]}]`, `[]`, true},
		{`$[?(@.x==true)].v`, `[{"x":true,"v":[1,2]}]`, `[1,2]`, true},
		{`$[?(@.x==true)].v`, `[{"x":true,"v":[1,2]},{"x":true,"v":[]}]`, `[[1,2],[]]`, true},
		{`$[?(@.x==true)].v`, `[{"x":true,"v":1},{"x":true,"v":1},{"x":true,"v":2}]`, `[1,1,2]`, true},
		{`$[?(@.keys[1].enabled == true)].v`, `[{"keys":[{}, {"enabled":true}],"v":9},{"keys":[],"v":8},{}]`, `9`, true},
		{`$[?(@.x==1)][?(@.y==2)]`, `[]`, `null`, false},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			p, err := Parse(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Decode([]byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				value, present, err := p.SelectValue(doc)
				encoded, marshalErr := json.Marshal(value)
				if err != nil || marshalErr != nil || present != tc.present || string(encoded) != tc.want {
					t.Fatalf("value=%s present=%v err=%v", encoded, present, err)
				}
			}
			encoded, _ := json.Marshal(doc)
			original, _ := Decode([]byte(tc.body))
			normalized, _ := json.Marshal(original)
			if string(encoded) != string(normalized) {
				t.Fatalf("source mutated: %s", encoded)
			}
		})
	}
}
