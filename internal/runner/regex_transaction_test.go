package runner

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-492/493: expected validation precedes extraction and decoding. Only the
// expected is rendered; inserted values are never treated as new templates.
func TestRegexExpectedProfiles(t *testing.T) {
	for _, tc := range []struct {
		name, decl, value, body string
		dynamic                 bool
		outcome                 Outcome
		detail                  string
	}{
		{"literal", `regex "(.*)" == "{{v}}"`, `resolved`, `{{v}}`, false, Passed, ""},
		{"dynamic", `regex "(.*)" == "{{v}}"`, `resolved`, `resolved`, true, Passed, ""},
		{"nonrecursive", `regex "(.*)" == "{{v}}"`, `"{{missing}}"`, `{{missing}}`, true, Passed, ""},
		{"static extraction", `regex "({{v}})" == "{{v}}"`, `resolved`, `resolved`, true, RegexMismatch, ""},
		{"static extraction literal", `regex "({{v}})" == "{{v}}"`, `"{{v}}"`, `{{v}}`, true, Passed, ""},
		{"missing before decode", `regex "no-group" not contains "{{missing}}"`, `0`, ``, true, RegexFailed, "variável ausente"},
		{"structure before decode", `regex "no-group" not contains "{{v}}"`, `[]`, ``, true, RegexFailed, "textual"},
		{"invalid matches before decode", `regex "no-group" matches "{{v}}"`, `"["`, ``, true, RegexFailed, "matches inválido"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "GET http://example.test\nHTTP 200\n[Asserts]\n" + tc.decl
			s := parseCaptureScenario(t, source)
			if tc.dynamic {
				s = dynamicScenario(t, source)
			}
			before := s.Entries[0].Assertions[0].Regex
			c, err := template.New([]string{"v=" + tc.value}, nil)
			if err != nil {
				t.Fatal(err)
			}
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				headers := []query.HeaderValue(nil)
				if tc.detail != "" {
					headers = []query.HeaderValue{{Name: "Content-Type", Value: "text/plain; charset=unknown"}}
				}
				return Response{Status: 200, Body: []byte(tc.body), Headers: headers}, nil
			}), c, nil).Run(context.Background(), s)
			if got.Entries[0].Outcome != tc.outcome || tc.detail != "" && (got.Err == nil || !strings.Contains(got.Err.Error(), tc.detail)) || s.Entries[0].Assertions[0].Regex != before {
				t.Fatalf("result=%+v error=%v", got, got.Err)
			}
		})
	}
}

// GO-494/495/500: retry inputs remain confirmed, overlays belong to one
// response, and the same immutable plan can be run repeatedly.
func TestRegexTransactionalPolling(t *testing.T) {
	for _, mode := range []string{"approved", "exhausted", "missing expected", "no group", "decode error", "capture error", "status first", "earlier assertion"} {
		t.Run(mode, func(t *testing.T) {
			decl := `regex "id=([^;]+)" == "{{id}}"` + "\n" + `regex "state=(.*)" == "READY"`
			switch mode {
			case "missing expected":
				decl = `regex "id=(.*)" not contains "{{missing}}"`
			case "no group":
				decl = `regex "id" not contains "x"`
			case "decode error":
				decl = `regex "(x)" not contains "x"`
			case "earlier assertion":
				decl = "header \"Missing\" exists\nregex \"(x)\" == \"{{missing}}\""
			}
			source := "POST http://example.test/{{id}}\n[Options]\nretry: 1\nretry-interval: 0\n{\"id\":\"{{id}}\"}\nHTTP 200\n[Asserts]\n" + decl + "\n[Captures]\nid: header \"X-ID\"\nGET http://example.test/{{id}}"
			scenario := dynamicScenario(t, source)
			before := dynamicScenario(t, source)
			c, _ := template.New([]string{"id=old"}, nil)
			for range 2 {
				session := c.NewSession()
				calls, waits, reports := 0, 0, 0
				r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
					calls++
					if calls == 3 && mode == "approved" {
						if req.URL != "http://example.test/accepted" {
							t.Fatal(req.URL)
						}
						return Response{Status: 200}, nil
					}
					if req.URL != "http://example.test/old" || string(req.Body) != `{"id":"old"}` {
						t.Fatalf("retry input=%+v", req)
					}
					id, state := "rejected", "PENDING"
					if calls == 2 && mode == "approved" {
						id, state = "accepted", "READY"
					}
					headers := []query.HeaderValue{{Name: "X-ID", Value: id}}
					if mode == "capture error" {
						headers = nil
					}
					if mode == "decode error" {
						headers = append(headers, query.HeaderValue{Name: "Content-Type", Value: "text/plain; charset=unknown"})
					}
					status := 200
					if mode == "status first" {
						status = 503
						headers = nil
					}
					return Response{Status: status, Headers: headers, Body: []byte("id=" + id + ";state=" + state)}, nil
				}), observedHeaderSession{session}, func(EntryResult) error { reports++; return nil })
				r.wait = func(context.Context, time.Duration) error {
					waits++
					v, _ := session.Render("{{id}}")
					if v.Value != "old" {
						t.Fatal("provisional capture committed")
					}
					return nil
				}
				got := r.Run(context.Background(), scenario)
				wantCalls, wantWaits, wantReports, wantID := 1, 0, 1, "old"
				switch mode {
				case "approved":
					wantCalls, wantWaits, wantReports, wantID = 3, 1, 2, "accepted"
				case "exhausted", "status first", "earlier assertion":
					wantCalls, wantWaits = 2, 1
				}
				value, _ := session.Render("{{id}}")
				if calls != wantCalls || waits != wantWaits || reports != wantReports || value.Value != wantID || !reflect.DeepEqual(scenario, before) || got.Attempts.Started != calls || got.Attempts.Completed != calls {
					t.Fatalf("calls=%d waits=%d reports=%d id=%s result=%+v", calls, waits, reports, value.Value, got)
				}
				switch mode {
				case "approved":
					if got.Err != nil || got.Diverged {
						t.Fatalf("%+v", got)
					}
				case "exhausted", "status first", "earlier assertion":
					if got.Err != nil || !got.Diverged {
						t.Fatalf("%+v", got)
					}
				default:
					if got.Err == nil || got.Diverged {
						t.Fatalf("%+v", got)
					}
				}
			}
		})
	}
}
