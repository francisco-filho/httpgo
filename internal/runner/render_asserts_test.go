package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func dynamicScenario(t *testing.T, source string) parser.Scenario {
	t.Helper()
	s, err := parser.ParseWithOptions("dynamic.hurl", strings.NewReader(source), parser.ParseOptions{RenderAsserts: true})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// GO-378,380,381,384: exact types, scalar interpolation and applicability.
func TestRenderAssertsJSONValues(t *testing.T) {
	for _, tc := range []struct {
		name, operand, value, body string
		outcome                    Outcome
	}{
		{"number", `== {{v}}`, `9007199254740993`, `9007199254740993`, Passed},
		{"number differs", `== {{v}}`, `9007199254740993`, `9007199254740992`, JSONMismatch},
		{"string number", `== "{{v}}"`, `9007199254740993`, `"9007199254740993"`, Passed},
		{"types differ", `== "{{v}}"`, `9007199254740993`, `9007199254740993`, JSONMismatch},
		{"null", `== {{v}}`, `null`, `null`, Passed}, {"boolean", `== {{v}}`, `true`, `true`, Passed},
		{"nested", `== {"a":[{{v}}]}`, `{"x":[9007199254740993,true,null]}`, `{"a":[{"x":[9007199254740993,true,null]}]}`, Passed},
		{"escapes", `== "{{v}}"`, `"a\"\\\n雪"`, `"a\"\\\n雪"`, Passed},
		{"no recursion", `== {{v}}`, `"{{missing}}"`, `"{{missing}}"`, Passed},
		{"no recursion string", `== "{{v}}"`, `"{{missing}}"`, `"{{missing}}"`, Passed},
		{"ordered", `> {{v}}`, `9007199254740992`, `9007199254740993`, Passed},
		{"ordered type", `> {{v}}`, `"1"`, `2`, JSONFailed},
		{"actual type", `> {{v}}`, `1`, `"2"`, JSONMismatch},
		{"count zero", `count == {{v}}`, `0.0`, `[]`, Passed},
		{"count exponent", `count == {{v}}`, `1e0`, `[null]`, Passed},
		{"count huge", `count < {{v}}`, `1e999999999999999999`, `[]`, Passed},
		{"count negative", `count == {{v}}`, `-1`, `[]`, JSONFailed},
		{"count fraction", `count == {{v}}`, `0.5`, `[]`, JSONFailed},
		{"composite text", `== "{{v}}"`, `[]`, `"[]"`, JSONFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := dynamicScenario(t, "GET http://example.test/first\nHTTP 200\n[Asserts]\njsonpath \"$\" "+tc.operand+"\nGET http://example.test/next")
			before := dynamicScenario(t, "GET http://example.test/first\nHTTP 200\n[Asserts]\njsonpath \"$\" "+tc.operand+"\nGET http://example.test/next")
			c, _ := template.New([]string{"v=" + tc.value}, nil)
			calls := 0
			result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(tc.body)}, nil
			}), c, nil).Run(context.Background(), s)
			want := 1
			if tc.outcome == Passed {
				want = 2
			}
			if calls != want || result.Entries[0].Outcome != tc.outcome || !reflect.DeepEqual(s, before) {
				t.Fatalf("calls=%d result=%+v mutated=%v", calls, result, !reflect.DeepEqual(s, before))
			}
			d := result.Entries[0].Entry.Assertions[0].JSON
			if d.ExpectedTemplate != nil || d.Expected != nil || d.Compiled != nil {
				t.Fatalf("operational metadata: %+v", d)
			}
		})
	}
}

// GO-385,386: only the reached assertion prepares an expected value, before query.
func TestRenderAssertsJSONOrder(t *testing.T) {
	for _, tc := range []struct {
		name, assertions, body, capture string
		status                          int
		outcome                         Outcome
		detail                          string
	}{
		{"status", `jsonpath "$" == {{missing}}`, `bad`, "", 503, Mismatch, ""},
		{"earlier assertion", "jsonpath \"$\" == 2\njsonpath \"$\" == {{missing}}", `1`, "", 200, JSONMismatch, ""},
		{"missing before decode", `jsonpath "$" == {{missing}}`, `bad`, "", 200, JSONFailed, "variável ausente"},
		{"own capture", `jsonpath "$" == {{own}}`, `1`, "\n[Captures]\nown: jsonpath \"$\"", 200, JSONFailed, "variável ausente"},
		{"future capture", `jsonpath "$" == {{future}}`, `1`, "", 200, JSONFailed, "variável ausente"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := dynamicScenario(t, "GET http://example.test/first\nHTTP 200\n[Asserts]\n"+tc.assertions+tc.capture+"\nGET http://example.test/next\n[Captures]\nfuture: jsonpath \"$\"")
			calls := 0
			result := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: tc.status, Body: []byte(tc.body)}, nil
			}), nil).Run(context.Background(), s)
			if calls != 1 || result.Entries[0].Outcome != tc.outcome {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
			if tc.detail != "" && (result.Err == nil || !strings.Contains(result.Err.Error(), tc.detail)) {
				t.Fatalf("error=%v", result.Err)
			}
		})
	}
}

// GO-390: provenance conceals even partially overlapping actual values, not just replacements.
func TestRenderAssertsJSONSecret(t *testing.T) {
	source := "GET http://example.test/\nHTTP 200\n[Asserts]\njsonpath \"$\" == {\"x\":\"prefix-{{v}}-suffix\"}"
	s := dynamicScenario(t, source)
	c, _ := template.New([]string{`v="fake-private"`}, []string{"v"})
	reports := 0
	result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		return Response{Status: 200, Body: []byte(`{"x":"prefix-partial-suffix"}`)}, nil
	}), c, func(e EntryResult) error {
		reports++
		text := fmt.Sprintf("%+v %+v", e.Entry.Assertions[0].JSON, e.JSONFailure)
		if strings.Contains(text, "fake-private") || strings.Contains(text, "partial") || strings.Contains(text, "{{") {
			t.Fatalf("unsafe callback: %s", text)
		}
		return nil
	}).Run(context.Background(), s)
	if reports != 1 || !result.Diverged || result.Entries[0].JSONFailure.Expected != "[oculto]" || result.Entries[0].JSONFailure.Actual != "[oculto]" {
		t.Fatalf("result=%+v", result)
	}
	data, _ := json.Marshal(result)
	if strings.Contains(string(data), "fake-private") || strings.Contains(string(data), "{{") {
		t.Fatalf("unsafe result: %s", data)
	}
}
