package assertion

import (
	"testing"

	"capimgrosso.com/httpgo/internal/query"
)

// GO-345: assertions and filters share exactly the same typed equality.
func TestFilterEqualityAndAssertionAgree(t *testing.T) {
	for _, literal := range []string{`9223372036854775809`, `1.0000000000000000000002`, `1e-99999999999999999999999`, `null`, `true`, `"pizza"`} {
		t.Run(literal, func(t *testing.T) {
			p, err := query.Parse("$[?(@.value == " + literal + ")].value")
			if err != nil {
				t.Fatal(err)
			}
			actual, present, err := p.Select([]byte(`[{"value":` + literal + `}]`))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := query.Decode([]byte(literal))
			if err != nil {
				t.Fatal(err)
			}
			result, err := EvaluateJSON(JSONEqualOperator, false, expected, actual, present)
			if err != nil || !result.Matched {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
}
