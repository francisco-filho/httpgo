package assertion

import (
	"encoding/json"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
)

func TestExactNumericOrder(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		order int
	}{
		{"0", "-0.0e999999999999999999999999", 0},
		{"-1", "0", -1}, {"0", "1", -1}, {"-2", "-1", -1}, {"1", "2", -1},
		{"-1.25", "-1.2500", 0}, {"-1.2500000000000000000001", "-1.25", -1},
		{"9007199254740992", "9007199254740993", -1},
		{"1.0000000000000000000001", "1.0000000000000000000002", -1},
		{"1.1", "1.11", -1}, {"12", "1.2e1", 0}, {"99", "100", -1},
		{"-100", "-99", -1}, {"-1.11", "-1.1", -1},
		{"1e99999999999999999999999", "10e99999999999999999999998", 0},
		{"1e99999999999999999999999", "1e99999999999999999999998", 1},
		{"1e-99999999999999999999999", "1e-99999999999999999999998", -1},
		{"-1e-99999999999999999999999", "0", -1},
		{"0", "1e-99999999999999999999999", -1},
		{"-1e99999999999999999999999", "-1e99999999999999999999998", -1},
	} {
		t.Run(tc.a+"/"+tc.b, func(t *testing.T) {
			a, b := json.Number(tc.a), json.Number(tc.b)
			if got := compareNumber(a, b); got != tc.order {
				t.Fatalf("compare(%s,%s)=%d want %d", a, b, got, tc.order)
			}
			if got := compareNumber(b, a); got != -tc.order {
				t.Fatalf("compare(%s,%s)=%d want %d", b, a, got, -tc.order)
			}
			for _, op := range []JSONOperator{JSONGreater, JSONGreaterEqual, JSONLess, JSONLessEqual} {
				want := op == JSONGreater && tc.order > 0 || op == JSONGreaterEqual && tc.order >= 0 || op == JSONLess && tc.order < 0 || op == JSONLessEqual && tc.order <= 0
				got, err := EvaluateJSON(op, false, b, a, true)
				if err != nil || got.Matched != want {
					t.Fatalf("%s %s %s=%+v err=%v want %v", a, op, b, got, err, want)
				}
			}
		})
	}
}

func TestOrderedJSONRequiresNumber(t *testing.T) {
	for _, body := range []string{`"1"`, `true`, `null`, `[]`, `{}`} {
		value, _ := query.Decode([]byte(body))
		for _, op := range []JSONOperator{JSONGreater, JSONGreaterEqual, JSONLess, JSONLessEqual} {
			got, err := EvaluateJSON(op, false, json.Number("0"), value, true)
			if err != nil || got.Matched || got.Detail == "" || got.Actual != nil {
				t.Fatalf("%s %s 0=%+v err=%v", body, op, got, err)
			}
			got, err = EvaluateJSON(op, false, json.Number("0"), value, false)
			if err != nil || got.Matched || got.Detail != "" {
				t.Fatalf("absent %s: %+v err=%v", op, got, err)
			}
		}
	}
}

func TestJSONEvaluationUnsupported(t *testing.T) {
	for _, tc := range []struct {
		op    JSONOperator
		count bool
	}{
		{"not ==", false}, {JSONNotEqualOperator, true}, {JSONExists, true},
	} {
		if _, err := EvaluateJSON(tc.op, tc.count, nil, []any{}, true); err == nil {
			t.Fatalf("accepted %s count=%v", tc.op, tc.count)
		}
	}
	if _, err := EvaluateJSON(JSONGreater, false, "1", json.Number("1"), true); err == nil {
		t.Fatal("accepted nonnumeric expected")
	}
}
