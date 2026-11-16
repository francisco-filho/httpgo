package query

import (
	"regexp"
	"testing"
)

func TestRegexFirstGroup(t *testing.T) {
	for _, tc := range []struct {
		pattern, text, want string
		fail                bool
	}{
		{`code=(\d+)-(\w+)`, "code=001-one code=002-two", "001", false},
		{`(a*)b`, "b", "", false},
		{`(a)?(b)`, "b ab", "", true},
		{`b`, "b", "", true},
		{`(a)`, "b", "", true},
		{`(?m)^é: (.+)$`, "before\né: 雪\nafter", "雪", false},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			got, err := Regex(regexp.MustCompile(tc.pattern), tc.text)
			if (err != nil) != tc.fail || got != tc.want {
				t.Fatalf("got=%q err=%v want=%q fail=%v", got, err, tc.want, tc.fail)
			}
		})
	}
	if _, err := Regex(nil, "private"); err == nil {
		t.Fatal("accepted missing compiled expression")
	}
}
