package assertion

import "testing"

func TestHeader(t *testing.T) {
	for _, tc := range []struct {
		operator         HeaderOperator
		expected, actual string
		present, want    bool
	}{
		{HeaderEqual, "a", "a", true, true}, {HeaderEqual, "A", "a", true, false},
		{HeaderContains, "a", "cat", true, true}, {HeaderContains, "A", "cat", true, false},
		{HeaderExists, "", "", true, true}, {HeaderEqual, "", "", true, true},
		{HeaderContains, "", "", true, true}, {HeaderContains, "", "value", true, true},
		{HeaderEqual, "", "", false, false}, {HeaderContains, "", "", false, false}, {HeaderExists, "", "", false, false},
		{HeaderEqual, "a,b", "a,b", true, true}, {HeaderEqual, "a", "a,b", true, false},
		{HeaderContains, "{{x}}", "{{x}}", true, true}, {HeaderOperator("unknown"), "", "", true, false},
	} {
		if got := Header(tc.operator, tc.expected, tc.actual, tc.present); got != tc.want {
			t.Fatalf("Header(%+v)=%v", tc, got)
		}
	}
}
