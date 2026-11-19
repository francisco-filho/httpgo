package query

import (
	"regexp"
	"testing"
)

func TestSelectRegex(t *testing.T) {
	for _, tc := range []struct {
		pattern, text, value string
		present, failed      bool
	}{
		{`(a)(b)`, "ab ab", "a", true, false},
		{`(a)?b`, "b ab", "", false, false},
		{`(a*)b`, "b ab", "", true, false},
		{`(x)`, "absent", "", false, false},
		{`x`, "x", "", false, true},
	} {
		t.Run(tc.pattern+tc.text, func(t *testing.T) {
			compiled := regexp.MustCompile(tc.pattern)
			value, present, err := SelectRegex(compiled, tc.text)
			if value != tc.value || present != tc.present || (err != nil) != tc.failed {
				t.Fatalf("value=%q present=%v err=%v", value, present, err)
			}
			if !tc.present {
				if _, err := Regex(compiled, tc.text); err == nil {
					t.Fatal("capture accepted absence/error")
				}
			}
		})
	}
	if _, _, err := SelectRegex(nil, ""); err == nil {
		t.Fatal("nil expression accepted")
	}
}
