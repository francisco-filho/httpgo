package assertion

import (
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
)

// GO-362–366, GO-371: values are decoded exactly as in the runner, never coerced.
func TestJSONTypePredicates(t *testing.T) {
	for _, tc := range []struct {
		operator         JSONOperator
		passing, failing []string
	}{
		{JSONIsString, []string{`""`, `"abc"`, `" "`}, []string{`null`, `false`, `0`, `[]`, `{}`}},
		{JSONIsInteger, []string{`0`, `-0`, `3`, `-3`, `9007199254740993`, `9999999999999999999999999999999999999999999`}, []string{`3.0`, `3e0`, `3E+0`, `-0.0`, `1.5`, `"3"`, `null`, `false`, `[]`, `{}`}},
		{JSONIsBoolean, []string{`true`, `false`}, []string{`0`, `1`, `"false"`, `null`, `[]`, `{}`}},
		{JSONIsEmpty, []string{`""`, `[]`, `{}`}, []string{`" "`, `"\t"`, `"abc"`, `[null]`, `{"x":null}`, `null`, `0`, `false`}},
	} {
		for _, pass := range []bool{true, false} {
			inputs := tc.failing
			if pass {
				inputs = tc.passing
			}
			for _, input := range inputs {
				t.Run(string(tc.operator)+"/"+input, func(t *testing.T) {
					value, err := query.Decode([]byte(input))
					if err != nil {
						t.Fatal(err)
					}
					got, err := EvaluateJSON(tc.operator, false, nil, value, true)
					if err != nil || got.Matched != pass || got.Actual != nil {
						t.Fatalf("%s: %+v, %v; want matched=%t without operand", input, got, err, pass)
					}
					if !pass && !strings.Contains(got.Detail, JSONType(value)) {
						t.Fatalf("missing categorical type: %+v", got)
					}
				})
			}
		}
		got, err := EvaluateJSON(tc.operator, false, nil, nil, false)
		if err != nil || got.Matched || got.Actual != nil || got.Detail != "valor ausente" {
			t.Fatalf("absence: %+v %v", got, err)
		}
		if _, err := EvaluateJSON(tc.operator, true, nil, nil, true); err == nil {
			t.Fatal("accepted count with unary")
		}
	}
	for _, input := range []string{`3`, `3.0`, `3e0`} {
		value, _ := query.Decode([]byte(input))
		expected, _ := query.Decode([]byte(`3`))
		got, err := EvaluateJSON(JSONEqualOperator, false, expected, value, true)
		if err != nil || !got.Matched {
			t.Fatalf("equality changed for %s: %+v %v", input, got, err)
		}
	}
}
