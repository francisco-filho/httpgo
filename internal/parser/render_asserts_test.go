package parser

import (
	"strings"
	"testing"
)

// GO-378–381,387: opt-in never postpones statically known errors.
func TestRenderAssertsJSONParsing(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		for _, tc := range []struct {
			operand          string
			dynamic, literal bool
		}{
			{`== {{value}}`, true, false}, {`== {"x":[{{value}},"{{text}}"]}`, true, false},
			{`== "{{missing}}"`, true, true}, {`count >= {{value}}`, true, false},
			{`> "{{value}}"`, false, false}, {`count == [{{value}}]`, false, false},
			{`== {{bad-name}}`, false, false}, {`== "{{broken"`, false, true},
			{`== {{value}} null`, false, false}, {`== "\q{{value}}"`, false, false},
			{`count == -1`, false, false}, {`contains [{{value}}]`, false, false},
			{`== {"{{key}}":1}`, false, true},
		} {
			for _, enabled := range []bool{false, true} {
				source := strings.ReplaceAll("GET http://example.test/first\nHTTP 200\nGET http://example.test/late\nHTTP *\n[Asserts]\n  jsonpath \"$\" "+tc.operand, "\n", nl)
				s, err := ParseWithOptions("profile.hurl", strings.NewReader(source), ParseOptions{RenderAsserts: enabled})
				want := tc.literal
				if enabled {
					want = tc.dynamic
				}
				if (err == nil) != want {
					t.Fatalf("enabled=%v operand=%s err=%v", enabled, tc.operand, err)
				}
				if err != nil {
					if e, ok := err.(*Error); !ok || e.Position.Line != 6 || e.Position.Column != 3 {
						t.Fatalf("location: %v", err)
					}
					continue
				}
				d := s.Entries[1].Assertions[0].JSON
				if !enabled && d.ExpectedTemplate != nil {
					t.Fatal("literal acquired template")
				}
			}
		}
	}
}
