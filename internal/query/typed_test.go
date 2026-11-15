package query

import (
	"encoding/json"
	"testing"
)

func TestTypedSelectionPrecisionAndPresence(t *testing.T) {
	for _, raw := range []string{`null`, `""`, `"{{literal}}"`, `true`, `false`, `[]`, `{}`, `9007199254740993`, `1.2300e+99`, `{"a":[1.2300e+99,null],"n":9007199254740993}`, `[9007199254740993,{"n":1.2300e+99}]`} {
		for _, tc := range []struct{ path, body string }{{"$", raw}, {"$.value", `{"value":` + raw + `}`}, {"$[0].value[0]", `[{"value":[` + raw + `]}]`}} {
			t.Run(tc.path+raw, func(t *testing.T) {
				p, err := Parse(tc.path)
				if err != nil {
					t.Fatal(err)
				}
				value, found, err := p.Select([]byte(tc.body))
				if err != nil || !found {
					t.Fatalf("got %v %v", found, err)
				}
				encoded, err := json.Marshal(value)
				if err != nil || string(encoded) != raw {
					t.Fatalf("got %s, %v; want %s", encoded, err, raw)
				}
			})
		}
	}
	for _, tc := range []struct{ path, body string }{{"$.missing", `{"value":null}`}, {"$[1]", `[null]`}, {"$.value.child", `{"value":null}`}, {"$.value[0]", `{"value":{}}`}} {
		p, _ := Parse(tc.path)
		if _, found, err := p.Select([]byte(tc.body)); err != nil || found {
			t.Fatalf("%+v: found=%v err=%v", tc, found, err)
		}
	}
}
