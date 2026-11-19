package query

import (
	"encoding/json"
	"testing"
)

func TestCountArray(t *testing.T) {
	for _, tc := range []struct {
		source, want string
		ok           bool
	}{
		{`[]`, "0", true}, {`[null,null,1]`, "3", true},
		{`[[1,2],[]]`, "2", true}, {`null`, "", false},
		{`"abc"`, "", false}, {`{}`, "", false}, {`1`, "", false}, {`true`, "", false},
	} {
		t.Run(tc.source, func(t *testing.T) {
			value, err := Decode([]byte(tc.source))
			if err != nil {
				t.Fatal(err)
			}
			number, ok := CountArray(value)
			if ok != tc.ok || number != json.Number(tc.want) {
				t.Fatalf("CountArray(%s)=(%q,%v), want (%q,%v)", tc.source, number, ok, tc.want, tc.ok)
			}
		})
	}
}
