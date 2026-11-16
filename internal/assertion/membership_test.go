package assertion

import (
	"testing"

	"capimgrosso.com/httpgo/internal/query"
)

func TestJSONMembership(t *testing.T) {
	for _, tc := range []struct {
		name, body, expected  string
		contains, notContains bool
	}{
		{"substring", `"Mini Pizza"`, `"Pizza"`, true, false},
		{"case", `"Pizza"`, `"pizza"`, false, true},
		{"space", `"a  b"`, `"a b"`, false, true},
		{"escape", `"a\nb\"雪"`, `"\nb\"雪"`, true, false},
		{"unicode", `"é"`, `"é"`, false, true},
		{"empty", `""`, `""`, true, false},
		{"empty substring", `"abc"`, `""`, true, false},
		{"empty array", `[]`, `""`, false, true},
		{"direct empty", `[""]`, `""`, true, false},
		{"typed string", `[1,true,null]`, `"1"`, false, true},
		{"typed bool", `[1,true,null]`, `true`, true, false},
		{"typed null", `[1,true,null]`, `null`, true, false},
		{"nested", `[[1],{"x":1}]`, `1`, false, true},
		{"numeric text", `["1"]`, `1`, false, true},
		{"decimal", `[1.00]`, `1e0`, true, false},
		{"signed zero", `[-0.00]`, `0e9999999999999999999999`, true, false},
		{"precision", `[9007199254740993]`, `9007199254740992`, false, true},
		{"precise decimal", `[1.00000000000000000001]`, `1.00000000000000000002`, false, true},
		{"huge exponent", `[1e9999999999999999999999]`, `10e9999999999999999999998`, true, false},
		{"object", `{"private":[1]}`, `1`, false, false},
		{"null", `null`, `null`, false, false},
		{"number", `1`, `1`, false, false},
		{"boolean", `true`, `true`, false, false},
		{"string incompatible expected", `"1"`, `1`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := query.Decode([]byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			expected, err := query.Decode([]byte(tc.expected))
			if err != nil {
				t.Fatal(err)
			}
			for _, op := range []JSONOperator{JSONContains, JSONNotContains} {
				want := tc.contains
				if op == JSONNotContains {
					want = tc.notContains
				}
				got, err := EvaluateJSON(op, false, expected, actual, true)
				if err != nil || got.Matched != want || got.Actual != nil {
					t.Fatalf("%s %s: got=%+v err=%v want=%v", tc.body, op, got, err, want)
				}
			}
		})
	}
	for _, op := range []JSONOperator{JSONContains, JSONNotContains} {
		got, err := EvaluateJSON(op, false, nil, nil, false)
		if err != nil || got.Matched != (op == JSONNotContains) {
			t.Fatalf("absent %s: %+v %v", op, got, err)
		}
		if _, err := EvaluateJSON(op, false, []any{}, nil, true); err == nil {
			t.Fatal("accepted composite expected")
		}
		if _, err := EvaluateJSON(op, true, nil, nil, true); err == nil {
			t.Fatal("accepted count membership")
		}
	}
}

func TestJSONNotExists(t *testing.T) {
	for _, body := range []string{`null`, `""`, `[]`, `{}`, `false`, `0`} {
		actual, _ := query.Decode([]byte(body))
		for _, present := range []bool{true, false} {
			got, err := EvaluateJSON(JSONNotExists, false, nil, actual, present)
			if err != nil || got.Matched == present || got.Actual != nil {
				t.Fatalf("%s present=%v: %+v %v", body, present, got, err)
			}
		}
	}
}

func TestHeaderNotContains(t *testing.T) {
	for _, tc := range []struct {
		actual, expected string
		present, want    bool
	}{
		{"", "", false, true}, {"", "x", true, true}, {"", "", true, false},
		{"Domain=x", "Domain", true, false}, {"domain=x", "Domain", true, true},
		{"a,b", "a,b", true, false},
	} {
		if got := Header(HeaderNotContains, tc.expected, tc.actual, tc.present); got != tc.want {
			t.Fatalf("%+v: got %v", tc, got)
		}
	}
}
