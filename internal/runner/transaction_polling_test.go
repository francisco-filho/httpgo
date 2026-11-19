package runner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// GO-466,470,472: every retry reads confirmed input; only the approved overlay
// reaches the next entry. Category 2 is terminal, regardless of retry budget.
func TestTransactionalPolling(t *testing.T) {
	for _, mode := range []string{"approved", "exhausted", "capture error", "evaluation error", "cancel wait"} {
		t.Run(mode, func(t *testing.T) {
			source := "POST http://example.test/{{id}}\n[Options]\nretry: 2\nretry-interval: 0\n{\"id\":{{id}}}\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"\nfresh: jsonpath \"$.id\"\n[Asserts]\njsonpath \"$.id\" == {{id}}\njsonpath \"$.status\" == \"READY\"\nGET http://example.test/{{id}}\nX-Fresh: {{fresh}}"
			if mode == "evaluation error" {
				source = strings.Replace(source, `jsonpath "$.status" == "READY"`, `jsonpath "$.status" == {{missing}}`, 1)
			}
			s := dynamicScenario(t, source)
			before := dynamicScenario(t, source)
			c, _ := template.New([]string{"id=old"}, nil)
			session := c.NewSession()
			var requests []Request
			reports, waits := 0, 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
				requests = append(requests, req)
				n := len(requests)
				if n == 3 && mode == "approved" {
					if req.URL != "http://example.test/accepted" || req.Headers[0].Value != "accepted" {
						t.Fatalf("next=%+v", req)
					}
					return Response{Status: 200}, nil
				}
				id, status := "rejected", "PENDING"
				if mode == "approved" && n == 2 {
					id, status = "accepted", "READY"
				}
				body := `{"id":"` + id + `","status":"` + status + `"}`
				if mode == "capture error" {
					body = `{"status":"PENDING"}`
				}
				return Response{Status: 200, Body: []byte(body)}, nil
			}), observedHeaderSession{session}, func(EntryResult) error { reports++; return nil })
			r.wait = func(ctx context.Context, _ time.Duration) error {
				waits++
				got, _ := session.Render("{{id}}")
				if got.Value != "old" {
					t.Fatal("rejected overlay changed retry input")
				}
				if _, err := session.Render("{{fresh}}"); err == nil {
					t.Fatal("rejected overlay published fresh")
				}
				if mode == "cancel wait" {
					cancel()
				}
				return ctx.Err()
			}
			got := r.Run(ctx, s)
			wantCalls, wantWaits, wantReports, wantID := 1, 0, 1, "old"
			switch mode {
			case "approved":
				wantCalls, wantWaits, wantReports, wantID = 3, 1, 2, "accepted"
			case "exhausted":
				wantCalls, wantWaits = 3, 2
			case "cancel wait":
				wantWaits = 1
			}
			if len(requests) != wantCalls || waits != wantWaits || reports != wantReports || got.Attempts.Started != wantCalls || got.Attempts.Completed != wantCalls {
				t.Fatalf("requests=%d waits=%d reports=%d result=%+v", len(requests), waits, reports, got)
			}
			for i, req := range requests {
				if mode == "approved" && i == 2 {
					break
				}
				if req.URL != "http://example.test/old" || string(req.Body) != `{"id":"old"}` {
					t.Fatalf("retry %d=%+v", i, req)
				}
			}
			value, _ := session.Render("{{id}}")
			if value.Value != wantID || !reflect.DeepEqual(s, before) {
				t.Fatalf("confirmed=%+v scenario mutated=%v", value, !reflect.DeepEqual(s, before))
			}
			if mode == "approved" && (got.Err != nil || got.Diverged) || mode == "exhausted" && (got.Err != nil || !got.Diverged) || mode == "capture error" && got.Entries[0].Outcome != CaptureFailed || mode == "evaluation error" && got.Entries[0].Outcome != JSONFailed || mode == "cancel wait" && !errors.Is(got.Err, context.Canceled) {
				t.Fatalf("result=%+v", got)
			}
		})
	}
}

type cancelAfterStage struct {
	template.Session
	cancel context.CancelFunc
}

func (s cancelAfterStage) Stage(batch []template.Capture) (template.Session, error) {
	view, err := s.Session.Stage(batch)
	s.cancel()
	return view, err
}

func TestCancelBeforeCaptureConfirmation(t *testing.T) {
	for _, assertions := range []string{"", "\n[Asserts]\njsonpath \"$.id\" == {{id}}"} {
		c, _ := template.New([]string{"id=old"}, []string{"id"})
		base := c.NewSession()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		s := dynamicScenario(t, "GET http://example.test\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"\nfresh: jsonpath \"$.id\""+assertions+"\nGET http://example.test/never")
		calls := 0
		got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			return Response{Status: 200, Body: []byte(`{"id":"new-private"}`)}, nil
		}), observedHeaderSession{cancelAfterStage{base, cancel}}, nil).Run(ctx, s)
		if !errors.Is(got.Err, context.Canceled) || got.Diverged || calls != 1 || got.Entries[0].Outcome != Cancelled {
			t.Fatalf("result=%+v calls=%d", got, calls)
		}
		value, _ := base.Render("{{id}}")
		if value.Value != "old" || base.Sanitize("new-private") != "[oculto]" {
			t.Fatalf("value=%+v", value)
		}
		if _, err := base.Render("{{fresh}}"); err == nil {
			t.Fatal("cancel confirmed fresh")
		}
	}
}

// GO-460,472–473: changing capture sources/types never loses precision or
// reinterprets text, and fresh runs still start from the immutable input.
func TestRenewalCaptureSourcesAndRunIsolation(t *testing.T) {
	for _, tc := range []struct {
		name, decl, body, want string
		headers                []query.HeaderValue
	}{
		{"JSON null", `value: jsonpath "$"`, `null`, `null`, nil},
		{"JSON precision", `value: jsonpath "$"`, `1.00000000000000000000002`, `1.00000000000000000000002`, nil},
		{"JSON collection", `value: jsonpath "$"`, `{"x":[9007199254740993]}`, `{"x":[9007199254740993]}`, nil},
		{"header", `value: header "X"`, `bad`, `"123"`, []query.HeaderValue{{Name: "X", Value: "123"}}},
		{"header empty", `value: header "X"`, `bad`, `""`, []query.HeaderValue{{Name: "X", Value: ""}}},
		{"regex", `value: regex "id=(.*)"`, `id={{missing}}`, `"{{missing}}"`, nil},
		{"count", `value: jsonpath "$" count`, `[null,true]`, `2`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := template.New([]string{`value={"initial":true}`}, nil)
			s := parseCaptureScenario(t, "POST http://example.test/capture\n{{value}}\nHTTP *\n[Captures]\n"+tc.decl+"\nPOST http://example.test/echo\n{{value}}\nHTTP 200")
			calls := 0
			r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				want := tc.want
				if calls%2 == 1 {
					want = `{"initial":true}`
				}
				if string(req.Body) != want || !json.Valid(req.Body) {
					t.Fatalf("body=%s want=%s", req.Body, want)
				}
				return Response{Status: 200, Body: []byte(tc.body), Headers: tc.headers}, nil
			}), c, nil)
			for range 2 {
				got := r.Run(context.Background(), s)
				if got.Err != nil || got.Diverged {
					t.Fatalf("result=%+v", got)
				}
			}
			if calls != 4 {
				t.Fatal(calls)
			}
		})
	}
}
