package runner

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestRegexCaptureStringsAndStaticPatterns(t *testing.T) {
	for _, value := range []string{"001234", "true", "null", `{"x":1}`, `{{missing}}`, `quote"\\雪`, ""} {
		for _, header := range []bool{false, true} {
			t.Run(value+stringBool(header), func(t *testing.T) {
				declaration := `x: regex "(?s)^value=(.*)$"`
				if header {
					declaration = `x: header "x-VALUE" regex "^value=(.*)$"`
				}
				source := "GET http://example.test/first\nHTTP 200\n[Captures]\n" + declaration + "\nPOST http://example.test/next\nX-Value: pre{{x}}suffix\n{\"full\":{{x}},\"text\":\"{{x}}\"}\nHTTP 200"
				scenario, before := parseCaptureScenario(t, source), parseCaptureScenario(t, source)
				calls := 0
				result := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
					calls++
					if calls == 1 {
						if req.RetainBody == header {
							t.Fatalf("retain=%v header=%v", req.RetainBody, header)
						}
						return Response{Status: 200, Body: []byte("value=" + value), Headers: []query.HeaderValue{{Name: "X-Value", Value: "value=" + value}, {Name: "X-Value", Value: "value=ignored"}}}, nil
					}
					var body map[string]string
					if json.Unmarshal(req.Body, &body) != nil || body["full"] != value || body["text"] != value || req.Headers[0].Value != "pre"+value+"suffix" {
						t.Fatalf("request=%+v body=%s", req, req.Body)
					}
					return Response{Status: 200}, nil
				}), nil).Run(context.Background(), scenario)
				if result.Err != nil || result.Diverged || calls != 2 || !reflect.DeepEqual(scenario, before) {
					t.Fatalf("%+v calls=%d", result, calls)
				}
			})
		}
	}
	for _, initial := range [][]string{nil, {"literal=changed"}} {
		renderer, _ := template.New(initial, nil)
		result := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { return Response{Body: []byte("{{literal}}")}, nil }), renderer, nil).Run(context.Background(), parseCaptureScenario(t, "GET http://example.test/\n[Captures]\nx: regex \"({{literal}})\""))
		if result.Err != nil {
			t.Fatal(result.Err)
		}
	}
}

func stringBool(v bool) string {
	if v {
		return " header"
	}
	return " body"
}

func TestRegexCaptureSelectionAndPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, expect, decl, body, contentType string
		status                                int
		headers                               []query.HeaderValue
		want                                  Outcome
	}{
		{name: "first group first match", decl: `x: regex "code=(\\d+)-(\\w+)"`, body: "code=001-one code=002-two", want: Passed},
		{name: "empty group", decl: `x: regex "(a*)b"`, body: "b", want: Passed},
		{name: "optional absent", decl: `x: regex "(a)?(b)"`, body: "b ab", want: CaptureFailed},
		{name: "no group", decl: `x: regex "b"`, body: "b", want: CaptureFailed},
		{name: "no match", decl: `x: regex "(a)"`, body: "b", want: CaptureFailed},
		{name: "utf8", decl: `x: regex "(?m)^é: (.+)$"`, body: "before\né: 雪\nafter", want: Passed},
		{name: "declared utf8", decl: `x: regex "(雪)"`, body: "雪", contentType: "text/plain; charset=utf-8", want: Passed},
		{name: "latin1", decl: `x: regex "(é)"`, body: string([]byte{0xe9}), contentType: "text/plain; charset=iso-8859-1", want: Passed},
		{name: "invalid utf8", decl: `x: regex "(.*)"`, body: string([]byte{0xff}), want: CaptureFailed},
		{name: "unknown charset", decl: `x: regex "(.*)"`, body: "text", contentType: "text/plain; charset=unknown", want: CaptureFailed},
		{name: "header bypasses decoder", decl: `x: header "X" regex "(token)"`, body: string([]byte{0xff}), contentType: "text/plain; charset=unknown", headers: []query.HeaderValue{{Name: "X", Value: "token"}}, want: Passed},
		{name: "header missing", decl: `x: header "X" regex "()"`, want: CaptureFailed},
		{name: "header empty", decl: `x: header "X" regex "()"`, headers: []query.HeaderValue{{Name: "X", Value: ""}}, want: Passed},
		{name: "header first only", decl: `x: header "X" regex "(token)"`, headers: []query.HeaderValue{{Name: "X", Value: "wrong"}, {Name: "x", Value: "token"}}, want: CaptureFailed},
		{name: "status first", status: 503, decl: `x: regex "no-group"`, body: string([]byte{0xff}), want: Mismatch},
		{name: "assertion first", expect: "[Asserts]\nheader \"Missing\" exists\n", decl: `x: regex "no-group"`, body: string([]byte{0xff}), want: HeaderMismatch},
		{name: "JSON assertion first", expect: "[Asserts]\njsonpath \"$\" == 42\n", decl: `x: regex "no-group"`, body: "42", want: CaptureFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			result := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				status := tc.status
				if status == 0 {
					status = 200
				}
				headers := append([]query.HeaderValue(nil), tc.headers...)
				if tc.contentType != "" {
					headers = append(headers, query.HeaderValue{Name: "Content-Type", Value: tc.contentType})
				}
				return Response{Status: status, Body: []byte(tc.body), Headers: headers}, nil
			}), nil).Run(context.Background(), parseCaptureScenario(t, "GET http://example.test/first\nHTTP 200\n"+tc.expect+"[Captures]\n"+tc.decl+"\nGET http://example.test/next"))
			wantCalls := 1
			if tc.want == Passed {
				wantCalls = 2
			}
			if calls != wantCalls || result.Entries[0].Outcome != tc.want {
				t.Fatalf("%+v calls=%d", result, calls)
			}
		})
	}
}

func TestRegexCaptureAtomicMixedBatches(t *testing.T) {
	for _, decl := range []string{
		"x: regex \"(42)\"\ny: header \"Missing\"",
		"x: header \"Set-Cookie\" regex \"refresh=([^;]+)\"\ny: jsonpath \"$.missing\"",
		"x: regex \"(42)\"\nx.child: jsonpath \"$\"",
		"x: regex \"(42)\"\nx: header \"X\"",
		"x: regex \"(42)\"\ninitial.child: regex \"(42)\"",
		"x: regex \"(42)\"\nprevious: regex \"(42)\"",
	} {
		t.Run(decl, func(t *testing.T) {
			renderer, _ := template.New([]string{"initial=original"}, []string{"x"})
			session := renderer.NewSession()
			if err := session.Publish([]template.Capture{{Name: "previous", Value: json.RawMessage(`"prior"`)}}); err != nil {
				t.Fatal(err)
			}
			declarations := parseCaptureScenario(t, "GET http://example.test/\n[Captures]\n"+decl).Entries[0].Captures
			err := captureResponse(session, declarations, Response{Body: []byte("42"), Headers: []query.HeaderValue{{Name: "X", Value: "42"}, {Name: "Set-Cookie", Value: "refresh=42; HttpOnly"}}})
			if err == nil {
				t.Fatal("accepted invalid batch")
			}
			if _, err := session.Render("{{x}}"); err == nil {
				t.Fatal("partial publication")
			}
			got, err2 := session.Render("{{initial}}/{{previous}}")
			if err2 != nil || got.Value != "original/prior" || strings.Contains(session.Sanitize("42"), "42") {
				t.Fatalf("%+v %v", got, err2)
			}
		})
	}
}
