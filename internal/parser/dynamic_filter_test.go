package parser

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// GO-401–403: both declaration kinds are validated in the file profile, before execution.
func TestDynamicFilterFileValidation(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		for _, capture := range []bool{false, true} {
			for _, tc := range []struct {
				path           string
				enabled, valid bool
			}{
				{`$.items[?(@.n=={{future}})].id`, true, true},
				{`$.items[?(@.n=='Order {{suffix}}')].id`, true, true},
				{`$.items[?(@.n=='{{literal}}')].id`, false, false},
				{`$.items[?(@.n=={{future}})].id`, false, false},
				{`$.items[?(@.n=='{{broken')].id`, true, false},
				{`$.items[?(@.n=={{v}})].{{field}}`, true, false},
			} {
				declaration := "[Asserts]\n  jsonpath " + strconv.Quote(tc.path) + " exists"
				col := 13
				if capture {
					declaration = "[Captures]\n  x: jsonpath " + strconv.Quote(tc.path)
					col = 16
				}
				source := strings.ReplaceAll("GET http://example.test/first\nGET http://example.test/late\nHTTP 200\n"+declaration, "\n", nl)
				s, err := ParseWithOptions("dynamic.hurl", strings.NewReader(source), ParseOptions{RenderAsserts: tc.enabled})
				if tc.valid {
					if err != nil {
						t.Fatal(err)
					}
					continue
				}
				var located *Error
				if !errors.As(err, &located) || len(s.Entries) != 0 || located.Position.Line != 5 || located.Position.Column != col {
					t.Fatalf("path=%s scenario=%+v error=%v", tc.path, s, err)
				}
			}
		}
	}
}
