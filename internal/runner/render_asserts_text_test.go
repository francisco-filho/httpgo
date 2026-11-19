package runner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-381–386,393: textual conversion, typed membership, final regex and absence.
func TestRenderAssertsTextEvaluation(t *testing.T) {
	for _, tc := range []struct {
		name, line, value, body, header string
		present                         bool
		outcome                         Outcome
	}{
		{"header equality", `header "X" == "{{v}}"`, `"a"`, `bad`, `a`, true, Passed},
		{"header first", `header "X" contains "{{v}}"`, `"second"`, `bad`, `first`, true, HeaderMismatch},
		{"header substring", `header "X" contains "{{v}}"`, `12`, `bad`, `x12y`, true, Passed},
		{"header negation absent", `header "X" not contains "{{v}}"`, `null`, `bad`, ``, false, Passed},
		{"header absent missing", `header "X" not contains "{{missing}}"`, `0`, `bad`, ``, false, HeaderFailed},
		{"header structure", `header "X" == "{{v}}"`, `[]`, `bad`, `[]`, true, HeaderFailed},
		{"body substring", `body contains "{{v}}"`, `true`, `xtruey`, ``, false, Passed},
		{"body negation", `body not contains "{{v}}"`, `"other"`, `text`, ``, false, Passed},
		{"body empty", `body contains "{{v}}"`, `""`, ``, ``, false, Passed},
		{"body structure", `body not contains "{{v}}"`, `{}`, `text`, ``, false, BodyFailed},
		{"body missing", `body not contains "{{missing}}"`, `0`, `text`, ``, false, BodyFailed},
		{"body nonrecursive", `body contains "{{v}}"`, `"{{missing}}"`, `{{missing}}`, ``, false, Passed},
		{"header nonrecursive", `header "X" == "{{v}}"`, `"{{missing}}"`, `bad`, `{{missing}}`, true, Passed},
		{"JSON membership number", `jsonpath "$" contains {{v}}`, `9007199254740993`, `[9007199254740993]`, ``, false, Passed},
		{"JSON membership type", `jsonpath "$" contains {{v}}`, `"9007199254740993"`, `[9007199254740993]`, ``, false, JSONMismatch},
		{"JSON substring", `jsonpath "$" contains "prefix-{{v}}"`, `12`, `"prefix-12"`, ``, false, Passed},
		{"JSON not contains absent", `jsonpath "$.missing" not contains {{v}}`, `null`, `{}`, ``, false, Passed},
		{"JSON invalid expected before absent", `jsonpath "$.missing" not contains {{v}}`, `[]`, `{}`, ``, false, JSONFailed},
		{"JSON missing before decode", `jsonpath "$" not contains {{missing}}`, `0`, `bad`, ``, false, JSONFailed},
		{"JSON regex expression", `jsonpath "$" matches "^{{v}}$"`, `"a|b"`, `"b"`, ``, false, Passed},
		{"header regex expression", `header "X" matches "^{{v}}$"`, `"a[0-9]+"`, `bad`, `a123`, true, Passed},
		{"regex empty", `jsonpath "$" matches "{{v}}"`, `""`, `""`, ``, false, Passed},
		{"regex absent", `header "X" matches "{{v}}"`, `""`, `bad`, ``, false, HeaderMismatch},
		{"regex null text", `header "X" matches "^{{v}}$"`, `null`, `bad`, `null`, true, Passed},
		{"regex invalid before decode", `jsonpath "$" matches "{{v}}"`, `"["`, `bad`, ``, false, JSONFailed},
		{"regex unsupported", `header "X" matches "{{v}}"`, `"(?=secret)"`, `bad`, ``, false, HeaderFailed},
		{"regex structure", `jsonpath "$" matches "{{v}}"`, `[]`, `"[]"`, ``, false, JSONFailed},
		{"regex nonrecursive", `header "X" matches "{{v}}"`, `"{{missing}}"`, `bad`, `{{missing}}`, true, Passed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := dynamicScenario(t, "GET http://example.test/first\nHTTP 200\n[Asserts]\n"+tc.line+"\n[Captures]\nchecked: header \"Y\"\nGET http://example.test/next")
			c, _ := template.New([]string{"v=" + tc.value}, nil)
			calls := 0
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				headers := []query.HeaderValue{{Name: "Y", Value: "captured"}}
				if tc.present {
					headers = append(headers, query.HeaderValue{Name: "X", Value: tc.header}, query.HeaderValue{Name: "x", Value: "second"})
				}
				return Response{Status: 200, Body: []byte(tc.body), Headers: headers}, nil
			}), c, nil).Run(context.Background(), s)
			want := 1
			if tc.outcome == Passed {
				want = 2
			}
			if calls != want || result.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v error=%v", calls, result, result.Err)
			}
			if result.Err != nil && result.ErrorPosition.Line != 4 {
				t.Fatalf("position=%+v", result.ErrorPosition)
			}
		})
	}
}

func TestRenderAssertsTextSecrets(t *testing.T) {
	for _, line := range []string{
		`header "X" == "prefix-{{v}}-suffix"`, `header "Set-Cookie" matches "prefix-{{v}}-suffix"`,
		`body contains "prefix-{{v}}-suffix"`, `jsonpath "$" matches "prefix-{{v}}-suffix"`,
	} {
		t.Run(line, func(t *testing.T) {
			s := dynamicScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\n"+line)
			c, _ := template.New([]string{`v="fake-credential"`}, []string{"v"})
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				return Response{Status: 200, Body: []byte(`"partial-private"`), Headers: []query.HeaderValue{{Name: "X", Value: "partial-private"}, {Name: "Set-Cookie", Value: "partial-private"}}}, nil
			}), c, func(e EntryResult) error {
				text := fmt.Sprintf("%+v %+v %+v %+v %+v", e.Entry.Assertions[0].Header, e.Entry.Assertions[0].JSON, e.HeaderFailure, e.JSONFailure, e.BodyFailure)
				for _, bad := range []string{"fake-credential", "partial-private", "{{v}}"} {
					if strings.Contains(text, bad) {
						t.Fatalf("leak %s", text)
					}
				}
				return nil
			}).Run(context.Background(), s)
			if result.Err != nil || !result.Diverged {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}
