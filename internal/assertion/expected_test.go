package assertion

import (
	"encoding/json"
	"testing"
)

func TestValidateJSONExpected(t *testing.T) {
	for _, tc := range []struct {
		op    JSONOperator
		count bool
		value any
		valid bool
	}{
		{JSONEqualOperator, false, map[string]any{"x": true}, true},
		{JSONGreater, false, json.Number("9007199254740993"), true},
		{JSONGreater, false, "1", false},
		{JSONEqualOperator, true, json.Number("1e999999999999999999"), true},
		{JSONEqualOperator, true, json.Number("1.0"), true},
		{JSONEqualOperator, true, json.Number("-1"), false},
		{JSONEqualOperator, true, json.Number("0.5"), false},
		{JSONContains, false, nil, true}, {JSONNotContains, false, []any{}, false},
		{JSONOperator("unknown"), false, nil, false},
	} {
		if err := ValidateJSONExpected(tc.op, tc.count, tc.value); (err == nil) != tc.valid {
			t.Errorf("%s count=%v value=%v err=%v", tc.op, tc.count, tc.value, err)
		}
	}
}
