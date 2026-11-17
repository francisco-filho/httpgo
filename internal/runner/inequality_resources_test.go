package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

// GO-425,428,429: transport/report failures preserve causes, counts and safe metadata.
func TestJSONInequalityExternalFailures(t *testing.T) {
	for _, mode := range []string{"transport", "cancel", "expected", "report success", "report mismatch", "report evaluation"} {
		t.Run(mode, func(t *testing.T) {
			expected := `"prefix-{{v}}-suffix"`
			if mode == "expected" || mode == "report evaluation" {
				expected = `{{missing}}`
			}
			capturePath := "$"
			if mode == "report success" {
				capturePath = "$"
			}
			s := dynamicScenario(t, "GET http://example.test\nHTTP 200\n[Asserts]\njsonpath \"$\" != "+expected+"\n[Captures]\nchecked: jsonpath \"$\"\npending: jsonpath \""+capturePath+"\"\nGET http://example.test/never")
			c, _ := template.New([]string{"v=fake-private"}, []string{"v", "pending"})
			session := c.NewSession()
			cause := errors.New("private I/O cause")
			calls, reports := 0, 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				response := Response{Status: 200, Body: []byte(`"prefix-fake-private-suffix"`)}
				if mode == "transport" {
					return response, cause
				}
				if mode == "cancel" {
					return response, context.Canceled
				}
				if mode == "report success" {
					response.Body = []byte(`"different-private"`)
				}
				return response, nil
			}), observedHeaderSession{session}, func(e EntryResult) error {
				reports++
				if e.JSONFailure != nil && (e.JSONFailure.Actual != "[oculto]" || e.JSONFailure.Expected != "[oculto]") {
					t.Fatalf("unsafe callback: %+v", e.JSONFailure)
				}
				data, _ := json.Marshal(e)
				for _, bad := range []string{"fake-private", "different-private", "{{", "private I/O"} {
					if strings.Contains(string(data), bad) {
						t.Fatalf("unsafe callback: %s", data)
					}
				}
				if strings.HasPrefix(mode, "report") {
					return cause
				}
				return nil
			}).Run(context.Background(), s)
			if calls != 1 || reports != 1 || (got.Err == nil && got.ReportErr == nil) || got.Summary.NotStarted != 1 {
				t.Fatalf("result=%+v calls=%d reports=%d", got, calls, reports)
			}
			if strings.HasPrefix(mode, "report") && !errors.Is(got.ReportErr, cause) {
				t.Fatalf("report cause=%v", got.ReportErr)
			}
			if mode == "transport" && (!errors.Is(got.Err, cause) || got.Summary.Completed != 0 || got.Entries[0].JSONError != nil) {
				t.Fatalf("partial evaluated: %+v", got)
			}
			if mode == "cancel" && !errors.Is(got.Err, context.Canceled) {
				t.Fatalf("cancel cause=%v", got.Err)
			}
			if _, err := session.Render("{{checked}}"); (err == nil) != (mode == "report success") {
				t.Fatalf("publication error=%v", err)
			}
			if mode == "expected" && !strings.Contains(got.Err.Error(), "variável ausente") {
				t.Fatalf("error=%v", got.Err)
			}
		})
	}
}

// GO-425: a previously captured secret and its complete derivative stay protected.
func TestJSONInequalityCapturedDerivative(t *testing.T) {
	s := dynamicScenario(t, "GET http://example.test/login\nHTTP 200\n[Captures]\nold: jsonpath \"$\"\nGET http://example.test/renew\nHTTP 200\n[Asserts]\njsonpath \"$\" != \"prefix-{{old}}-suffix\"\nGET http://example.test/never")
	c, _ := template.New(nil, []string{"old"})
	calls := 0
	got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		calls++
		body := `"fake-private"`
		if calls == 2 {
			body = `"prefix-fake-private-suffix"`
		}
		return Response{Status: 200, Body: []byte(body)}, nil
	}), c, func(e EntryResult) error {
		if e.JSONFailure != nil && (e.JSONFailure.Expected != "[oculto]" || e.JSONFailure.Actual != "[oculto]") {
			t.Fatalf("failure=%+v", e.JSONFailure)
		}
		return nil
	}).Run(context.Background(), s)
	if calls != 2 || !got.Diverged || got.Err != nil {
		t.Fatalf("result=%+v calls=%d", got, calls)
	}
	data, _ := json.Marshal(got)
	if strings.Contains(string(data), "fake-private") || strings.Contains(string(data), "{{") {
		t.Fatalf("unsafe result: %s", data)
	}
}
