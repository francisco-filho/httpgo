package assertion

import (
	"testing"

	"capimgrosso.com/httpgo/internal/query"
)

func TestJSONEqual(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		equal bool
	}{
		{`""`, `""`, true}, {`"4500"`, `4500`, false},
		{`true`, `true`, true}, {`false`, `0`, false}, {`null`, `null`, true},
		{`[]`, `[]`, true}, {`{}`, `{}`, true}, {`[]`, `{}`, false},
		{`1`, `1.0`, true}, {`1e0`, `0.1e1`, true}, {`-12.50`, `-125e-1`, true},
		{`0`, `-0.000e99999999999999999999999`, true},
		{`1e99999999999999999999999`, `10e99999999999999999999998`, true},
		{`1e-99999999999999999999999`, `2e-99999999999999999999999`, false},
		{`9007199254740993`, `9007199254740992`, false},
		{`1.0000000000000000000001`, `1`, false},
		{`[1,2]`, `[2,1]`, false}, {`{"a":1,"b":[null,true]}`, `{"b":[null,true],"a":1e0}`, true},
		{`{"a":[9007199254740993]}`, `{"a":[9007199254740992]}`, false},
		{`{"a":null}`, `{"b":null}`, false}, {`[true]`, `["true"]`, false},
	} {
		t.Run(tc.a+"/"+tc.b, func(t *testing.T) {
			a, err := query.Decode([]byte(tc.a))
			if err != nil {
				t.Fatal(err)
			}
			b, err := query.Decode([]byte(tc.b))
			if err != nil {
				t.Fatal(err)
			}
			if got := JSONEqual(a, b); got != tc.equal {
				t.Fatalf("equal(%s,%s)=%v, want %v", tc.a, tc.b, got, tc.equal)
			}
		})
	}
}
