package assertion

import (
	"encoding/json"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
)

// GO-416–419,427: inequality negates exact equality only for a present value.
func TestJSONNotEqual(t *testing.T) {
	literals := []string{`null`, `false`, `0`, `""`, `[]`, `{}`, `true`, `"1"`, `1`}
	for _, expected := range literals {
		for _, actual := range literals {
			t.Run(expected+"/"+actual, func(t *testing.T) {
				a, err := query.Decode([]byte(actual))
				if err != nil {
					t.Fatal(err)
				}
				e, err := query.Decode([]byte(expected))
				if err != nil {
					t.Fatal(err)
				}
				if err := ValidateJSONExpected(JSONNotEqualOperator, false, e); err != nil {
					t.Fatal(err)
				}
				result, err := EvaluateJSON(JSONNotEqualOperator, false, e, a, true)
				if err != nil || result.Matched != (expected != actual) {
					t.Fatalf("result=%+v err=%v", result, err)
				}
			})
		}
	}
	for _, tc := range []struct {
		expected, actual string
		different        bool
	}{
		{`1`, `1.0`, false}, {`1`, `1e0`, false}, {`0`, `-0e999999999999999999`, false},
		{`{"a":1,"b":2}`, `{"b":2.0,"a":1e0}`, false},
		{`[1,2]`, `[2,1]`, true}, {`{"a":[1]}`, `{"a":[2]}`, true},
		{`9007199254740993`, `9007199254740992`, true},
		{`[9007199254740993]`, `[9007199254740992]`, true},
		{`1e999999999999999999`, `2e999999999999999999`, true},
		{`1e-999999999999999999`, `2e-999999999999999999`, true},
	} {
		e, _ := query.Decode([]byte(tc.expected))
		a, _ := query.Decode([]byte(tc.actual))
		result, err := EvaluateJSON(JSONNotEqualOperator, false, e, a, true)
		if err != nil || result.Matched != tc.different {
			t.Fatalf("%s != %s: %+v err=%v", tc.expected, tc.actual, result, err)
		}
	}
	result, err := EvaluateJSON(JSONNotEqualOperator, false, nil, nil, false)
	if err != nil || result.Matched {
		t.Fatalf("absence: %+v %v", result, err)
	}
	if _, err := EvaluateJSON(JSONNotEqualOperator, true, json.Number("0"), []any{}, true); err == nil {
		t.Fatal("accepted count !=")
	}
	if err := ValidateJSONExpected(JSONNotEqualOperator, true, json.Number("0")); err == nil {
		t.Fatal("validated count !=")
	}
}
