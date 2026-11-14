package assertion

import "testing"

func TestStatus(t *testing.T) {
	for _, tc := range []struct {
		name               string
		expected, received int
		want               bool
	}{
		{"success", 200, 200, true},
		{"not found", 404, 404, true},
		{"server error", 503, 503, true},
		{"mismatch", 200, 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Status(tc.expected, tc.received); got != tc.want {
				t.Fatalf("Status(%d, %d) = %v, want %v", tc.expected, tc.received, got, tc.want)
			}
		})
	}
}
