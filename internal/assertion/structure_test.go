package assertion

import (
	"encoding/json"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
)

func TestJSONPresenceAndCount(t *testing.T) {
	for _, body := range []string{`null`, `""`, `false`, `0`, `[]`, `{}`, `[null]`} {
		value, _ := query.Decode([]byte(body))
		for _, present := range []bool{false, true} {
			got, err := EvaluateJSON(JSONExists, false, nil, value, present)
			if err != nil || got.Matched != present {
				t.Fatalf("exists(%s,%v)=%+v err=%v", body, present, got, err)
			}
		}
	}
	for _, tc := range []struct {
		body, expected string
		op             JSONOperator
		matched        bool
	}{
		{`[]`, "0", JSONEqualOperator, true},
		{`[null,0,false]`, "3", JSONEqualOperator, true},
		{`[]`, "0", JSONGreater, false}, {`[]`, "0", JSONGreaterEqual, true},
		{`[]`, "0", JSONLess, false}, {`[]`, "0", JSONLessEqual, true},
		{`[null]`, "0", JSONGreater, true}, {`[]`, "1", JSONLess, true},
		{`[null]`, "900719925474099300000000000000000000000000", JSONLess, true},
		{`[null]`, "1e9999999999999999999999999999", JSONGreaterEqual, false},
	} {
		value, _ := query.Decode([]byte(tc.body))
		got, err := EvaluateJSON(tc.op, true, json.Number(tc.expected), value, true)
		if err != nil || got.Matched != tc.matched || JSONType(got.Actual) != "número" {
			t.Fatalf("count(%s) %s %s=%+v err=%v", tc.body, tc.op, tc.expected, got, err)
		}
	}
	for _, body := range []string{`null`, `"private"`, `1`, `true`, `{"private":1}`} {
		value, _ := query.Decode([]byte(body))
		got, err := EvaluateJSON(JSONEqualOperator, true, json.Number("0"), value, true)
		if err != nil || got.Matched || got.Detail == "" || got.Actual != nil {
			t.Fatalf("count(%s)=%+v err=%v", body, got, err)
		}
	}
	got, err := EvaluateJSON(JSONEqualOperator, true, json.Number("0"), nil, false)
	if err != nil || got.Matched || got.Detail != "" {
		t.Fatalf("absent count=%+v err=%v", got, err)
	}
}

func TestValidCount(t *testing.T) {
	for _, tc := range []struct {
		source string
		valid  bool
	}{
		{"0", true}, {"-0", true}, {"1.0", true}, {"1e99999999999999999999999", true},
		{"100e-2", true}, {"0e-999999999999999999", true},
		{"-1", false}, {"0.1", false}, {"1e-999999999999999999", false}, {`"1"`, false}, {"null", false},
	} {
		value, _ := query.Decode([]byte(tc.source))
		if got := ValidCount(value); got != tc.valid {
			t.Fatalf("ValidCount(%s)=%v want %v", tc.source, got, tc.valid)
		}
	}
}
