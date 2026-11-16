package assertion

import "testing"

func TestBodyLiteralSubstring(t *testing.T) {
	for _, tc := range []struct {
		name, actual, expected string
		contains               bool
	}{
		{"plain", "customer details", "customer", true},
		{"absent", "customer", "passwordHash", false},
		{"case", "Customer", "customer", false},
		{"spaces", "a  b", "a b", false},
		{"newline", "a\nb", "a\nb", true},
		{"unicode", "e\u0301", "é", false},
		{"empty both", "", "", true},
		{"empty actual", "", "x", false},
		{"empty expected", "text", "", true},
		{"JSON original", `{"x": "\u0061"}`, `\u0061`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Body(BodyContains, tc.expected, tc.actual); got != tc.contains {
				t.Fatalf("contains(%q, %q)=%v want %v", tc.actual, tc.expected, got, tc.contains)
			}
			if got := Body(BodyNotContains, tc.expected, tc.actual); got == tc.contains {
				t.Fatalf("not contains(%q, %q)=%v", tc.actual, tc.expected, got)
			}
		})
	}
	if Body("unknown", "", "") {
		t.Fatal("unknown operator approved")
	}
}
