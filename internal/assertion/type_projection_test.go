package assertion

import (
	"testing"

	"capimgrosso.com/httpgo/internal/query"
)

func TestJSONTypeAfterReduction(t *testing.T) {
	path, err := query.Parse("$[*]")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		body             string
		operator         JSONOperator
		matched, present bool
	}{
		{`[]`, JSONIsEmpty, false, false},
		{`[""]`, JSONIsString, true, true},
		{`["a","b"]`, JSONIsString, false, true},
		{`[9007199254740993]`, JSONIsInteger, true, true},
		{`[1,2]`, JSONIsInteger, false, true},
		{`[false]`, JSONIsBoolean, true, true},
		{`[false,true]`, JSONIsBoolean, false, true},
		{`[[]]`, JSONIsEmpty, true, true},
		{`[[],[]]`, JSONIsEmpty, false, true},
	} {
		t.Run(string(tc.operator)+tc.body, func(t *testing.T) {
			actual, present, err := path.Select([]byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			got, err := EvaluateJSON(tc.operator, false, nil, actual, present)
			if err != nil || present != tc.present || got.Matched != tc.matched || got.Actual != nil {
				t.Fatalf("%s: present=%v %+v %v", tc.body, present, got, err)
			}
		})
	}
}
