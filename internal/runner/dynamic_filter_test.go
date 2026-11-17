package runner

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

// GO-397–400,404–405: resolution is typed, nonrecursive and precedes decode/selection.
func TestDynamicFilterValuesAndOrder(t *testing.T) {
	for _, tc := range []struct {
		name, operand, value, body string
		outcome                    Outcome
	}{
		{"number", `{{v}}`, `9007199254740993`, `[{"n":9007199254740993,"id":1}]`, Passed},
		{"string number", `{{v}}`, `"1"`, `[{"n":1,"id":1}]`, JSONMismatch},
		{"boolean", `{{v}}`, `true`, `[{"n":true,"id":1}]`, Passed},
		{"null", `{{v}}`, `null`, `[{"n":null,"id":1}]`, Passed},
		{"absent not null", `{{v}}`, `null`, `[{"id":1}]`, JSONMismatch},
		{"composed", `'Order {{v}}'`, `42`, `[{"n":"Order 42","id":1}]`, Passed},
		{"double quotes", `"Order {{v}}"`, `false`, `[{"n":"Order false","id":1}]`, Passed},
		{"empty", `'{{v}}'`, `""`, `[{"n":"","id":1}]`, Passed},
		{"text null", `'{{v}}'`, `null`, `[{"n":"null","id":1}]`, Passed},
		{"escapes", `'{{v}}'`, `"a\"'\\[]雪"`, `[{"n":"a\"'\\[]雪","id":1}]`, Passed},
		{"injection", `{{v}}`, `"x')].id[?(@.n=='other"`, `[{"n":"x')].id[?(@.n=='other","id":1},{"n":"other","id":2}]`, Passed},
		{"nonrecursive", `'{{v}}'`, `"{{missing}}"`, `[{"n":"{{missing}}","id":1}]`, Passed},
		{"array", `{{v}}`, `[]`, `[]`, JSONFailed}, {"object", `{{v}}`, `{}`, `[]`, JSONFailed},
		{"array text", `'{{v}}'`, `[]`, `[]`, JSONFailed}, {"object text", `'{{v}}'`, `{}`, `[]`, JSONFailed},
		{"missing empty", `{{missing}}`, `1`, `[]`, JSONFailed}, {"missing before decode", `{{missing}}`, `1`, `bad`, JSONFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := `$[?(@.n==` + tc.operand + `)].id`
			source := "GET http://example.test/first\nHTTP 200\n[Asserts]\njsonpath " + strconv.Quote(path) + " == 1\n[Captures]\nselected: jsonpath " + strconv.Quote(path) + "\nGET http://example.test/{{selected}}"
			s := dynamicScenario(t, source)
			c, _ := template.New([]string{"v=" + tc.value}, nil)
			calls := 0
			result := NewWithRenderer(executorFunc(func(_ context.Context, r Request) (Response, error) {
				calls++
				if calls == 2 && r.URL != "http://example.test/1" {
					t.Fatal(r.URL)
				}
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), c, nil).Run(context.Background(), s)
			want := 1
			if tc.outcome == Passed {
				want = 2
			}
			if calls != want || result.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v err=%v", calls, result, result.Err)
			}
		})
	}
}

func TestDynamicFilterFirstFailure(t *testing.T) {
	for _, tc := range []struct {
		name, prefix, expected string
		status                 int
		outcome                Outcome
		detail                 string
	}{
		{"status", "", `1`, 503, Mismatch, ""},
		{"earlier assertion", "jsonpath \"$\" == 1\n", `1`, 200, JSONMismatch, ""},
		{"expected first", "", `{{expected_missing}}`, 200, JSONFailed, "esperado JSON"},
		{"operand before selection", "", `1`, 200, JSONFailed, "operando JSONPath"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "GET http://example.test/first\nHTTP 200\n[Asserts]\n" + tc.prefix + `jsonpath "$[?(@.n=={{missing}})].id" == ` + tc.expected + "\nGET http://example.test/next"
			calls := 0
			result := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: tc.status, Body: []byte(`[]`)}, nil
			}), nil).Run(context.Background(), dynamicScenario(t, source))
			if calls != 1 || result.Entries[0].Outcome != tc.outcome || tc.detail != "" && (result.Err == nil || !strings.Contains(result.Err.Error(), tc.detail)) {
				t.Fatalf("calls=%d result=%+v err=%v", calls, result, result.Err)
			}
		})
	}
}

// GO-409: secret provenance hides summaries/expected values, not merely matching text.
func TestDynamicFilterSecret(t *testing.T) {
	s := dynamicScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\n"+`jsonpath "$[?(@.n=='Order {{v}}')].id" == 2`)
	c, _ := template.New([]string{`v="fake-private"`}, []string{"v"})
	reports := 0
	result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		return Response{Status: 200, Body: []byte(`[{"n":"Order fake-private","id":1}]`)}, nil
	}), c, func(e EntryResult) error {
		reports++
		text := fmt.Sprintf("%+v %+v", e.Entry.Assertions[0].JSON, e.JSONFailure)
		if strings.Contains(text, "fake-private") || strings.Contains(text, "{{") {
			t.Fatal(text)
		}
		return nil
	}).Run(context.Background(), s)
	if reports != 1 || result.Entries[0].JSONFailure.Actual != "[oculto]" || result.Entries[0].JSONFailure.Expected != "[oculto]" {
		t.Fatalf("result=%+v", result)
	}
}
