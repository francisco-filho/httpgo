package query

import "testing"

func TestHeader(t *testing.T) {
	headers := []HeaderValue{{"X-Multi", "first,whole"}, {"x-MULTI", "second"}, {"X-Empty", ""}}
	for _, tc := range []struct {
		name, value string
		present     bool
	}{{"x-multi", "first,whole", true}, {"X-EMPTY", "", true}, {"Missing", "", false}} {
		value, present := Header(headers, tc.name)
		if value != tc.value || present != tc.present {
			t.Fatalf("Header(%q)=%q,%v want=%q,%v", tc.name, value, present, tc.value, tc.present)
		}
	}
}
