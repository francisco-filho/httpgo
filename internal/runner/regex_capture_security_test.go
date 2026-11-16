package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

func TestRegexCaptureProtectionAndAtomicityOnFailure(t *testing.T) {
	const secret = `private"\\雪{{missing}}`
	for _, origin := range []string{"body", "header"} {
		for _, mode := range []string{"success", "mixed", "collision", "status", "pending assertion", "transport", "report"} {
			t.Run(origin+"/"+mode, func(t *testing.T) {
				marks := []string(nil)
				decl := `x: header "sEt-CoOkIe" regex "refresh=([^;]+)"`
				if origin == "body" {
					marks = []string{"x"}
					decl = `x: regex "value=(.*)"`
				}
				renderer, _ := template.New([]string{"initial=original"}, marks)
				session := renderer.NewSession()
				source := "GET http://example.test/first\nHTTP 200\n[Captures]\n" + decl + "\n"
				if mode == "mixed" {
					source += "bad: regex \"(absent)\"\n"
				}
				if mode == "collision" {
					source += "initial: header \"X\"\n"
				}
				if mode == "pending assertion" {
					source += "[Asserts]\njsonpath \"$\" == \"pending-sensitive\"\n"
				}
				source += "GET http://example.test/next\nX-Value: prefix{{x}}suffix"
				scenario, before := parseCaptureScenario(t, source), parseCaptureScenario(t, source)
				calls, reports := 0, 0
				reportCause := errors.New("write failed")
				result := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
					calls++
					if calls == 2 {
						if req.Headers[0].Value != "prefix"+secret+"suffix" {
							t.Fatal("operational value changed")
						}
						return Response{Status: 200}, nil
					}
					response := Response{Status: 200, Body: []byte("value=" + secret), Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "refresh=" + secret + "; HttpOnly"}, {Name: "X", Value: "literal"}}}
					if mode == "pending assertion" {
						response.Body = []byte(`"not-pending-sensitive"`)
					}
					if mode == "status" {
						response.Status = 503
					}
					if mode == "transport" {
						return response, errors.New("read failed")
					}
					return response, nil
				}), observedHeaderSession{session}, func(e EntryResult) error {
					reports++
					encoded, _ := json.Marshal(e)
					if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "refresh=([^;]+)") || e.Entry.Captures != nil || e.Entry.Body != nil {
						t.Fatalf("unsafe metadata: %s", encoded)
					}
					if mode == "pending assertion" && (e.JSONFailure.Expected != "[oculto]" || e.JSONFailure.Actual != "[oculto]") {
						t.Fatal("pending operands exposed")
					}
					if mode == "report" {
						return reportCause
					}
					return nil
				}).Run(context.Background(), scenario)
				wantCalls := 1
				if mode == "success" {
					wantCalls = 2
				}
				if calls != wantCalls || reports != wantCalls || !reflect.DeepEqual(scenario, before) {
					t.Fatalf("%+v calls=%d reports=%d", result, calls, reports)
				}
				evaluated := mode == "success" || mode == "mixed" || mode == "collision" || mode == "report"
				if evaluated {
					for _, variant := range template.SecretVariants(secret) {
						if strings.Contains(session.Sanitize(variant), variant) {
							t.Fatal("lost group protection")
						}
					}
				}
				if mode != "success" && mode != "report" {
					if _, err := session.Render("{{x}}"); err == nil {
						t.Fatal("partial publication")
					}
				}
				if mode == "report" && (!errors.Is(result.ReportErr, reportCause) || result.Summary.Completed != 1 || result.Summary.NotStarted != 1) {
					t.Fatalf("%+v", result)
				}
				fresh := renderer.NewSession()
				if fresh.Sanitize(secret) != secret {
					t.Fatal("dynamic protection escaped invocation")
				}
			})
		}
	}
}

func TestRegexCaptureReusedRunnerAndEarlyReferences(t *testing.T) {
	renderer, _ := template.New(nil, nil)
	source := "GET http://example.test/first\n[Captures]\nx: header \"Set-Cookie\" regex \"refresh=([^;]+)\"\nGET http://example.test/next\nX-Value: {{x}}"
	scenario, before := parseCaptureScenario(t, source), parseCaptureScenario(t, source)
	calls := 0
	r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
		calls++
		if strings.HasSuffix(req.URL, "/next") && req.Headers[0].Value != "private-token" {
			t.Fatal("value changed")
		}
		return Response{Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "refresh=private-token; Path=/"}}}, nil
	}), renderer, nil)
	for range 2 {
		if result := r.Run(context.Background(), scenario); result.Err != nil {
			t.Fatal(result.Err)
		}
	}
	early := r.Run(context.Background(), parseCaptureScenario(t, "GET http://example.test/early\nX-Value: {{x}}\nGET http://example.test/later\n[Captures]\nx: regex \"(.*)\""))
	if early.Err == nil || calls != 4 || !reflect.DeepEqual(scenario, before) || renderer.IsSecret("x") {
		t.Fatalf("result=%+v calls=%d scenario=%s", early, calls, fmt.Sprint(scenario))
	}
}
