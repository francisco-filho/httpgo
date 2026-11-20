package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/template"
)

type optionPublishSession struct {
	template.Session
	publications int
}

func (s *optionPublishSession) Publish(batch []template.Capture) error {
	s.publications++
	return s.Session.Publish(batch)
}

// GO-578/584: the option batch is published once, not once per attempt.
func TestOptionVariablePollingConfirmation(t *testing.T) {
	for _, mode := range []string{"approved", "exhausted", "capture error", "cancel wait", "cancel before next"} {
		t.Run(mode, func(t *testing.T) {
			c, _ := template.New([]string{"id=initial"}, nil)
			session := &optionPublishSession{Session: c.NewSession()}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := dynamicScenario(t, `POST http://example.test/{{id}}
[Options]
variable: id=assigned
retry: 2
retry-interval: 0
{"id":"{{id}}"}
HTTP 200
[Captures]
id: jsonpath "$.id"
[Asserts]
jsonpath "$.id" == {{id}}
jsonpath "$.ready" == true
GET http://example.test/{{id}}
[Options]
variable: next=must-not-publish
HTTP 200`)
			calls, waits, callbacks := 0, 0, 0
			r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				want := "assigned"
				if calls == 3 && mode == "approved" {
					want = "accepted"
				}
				if req.URL != "http://example.test/"+want || calls < 3 && string(req.Body) != `{"id":"assigned"}` {
					t.Fatalf("request=%+v", req)
				}
				if mode == "capture error" {
					return Response{Status: 200, Body: []byte(`{}`)}, nil
				}
				ready := calls == 2 && mode == "approved" || mode == "cancel before next"
				return Response{Status: 200, Body: []byte(`{"id":"accepted","ready":` + map[bool]string{false: "false", true: "true"}[ready] + `}`)}, nil
			}), observedHeaderSession{session}, func(EntryResult) error {
				callbacks++
				if mode == "cancel before next" {
					cancel()
				}
				return nil
			})
			r.wait = func(ctx context.Context, _ time.Duration) error {
				waits++
				value, _ := session.Render("{{id}}")
				if value.Value != "assigned" {
					t.Fatalf("rejected capture changed assigned value=%+v", value)
				}
				if mode == "cancel wait" {
					cancel()
				}
				return ctx.Err()
			}
			got := r.Run(ctx, s)
			wantCalls, wantWaits, wantCallbacks, wantID, wantPublish := 1, 0, 1, "assigned", 1
			switch mode {
			case "approved":
				wantCalls, wantWaits, wantCallbacks, wantID, wantPublish = 3, 1, 2, "accepted", 3
			case "exhausted":
				wantCalls, wantWaits = 3, 2
			case "cancel wait":
				wantWaits = 1
			case "cancel before next":
				wantID, wantPublish = "accepted", 2
			}
			value, _ := session.Render("{{id}}")
			if calls != wantCalls || waits != wantWaits || callbacks != wantCallbacks || session.publications != wantPublish || value.Value != wantID || got.Attempts.Started != calls || got.Summary.Started != wantCallbacks {
				t.Fatalf("calls=%d waits=%d callbacks=%d publications=%d value=%+v result=%+v", calls, waits, callbacks, session.publications, value, got)
			}
			if strings.HasPrefix(mode, "cancel") && !errors.Is(got.Err, context.Canceled) || mode == "exhausted" && !got.Diverged || mode == "approved" && (got.Err != nil || got.Diverged) {
				t.Fatalf("result=%+v", got)
			}
			if mode != "approved" {
				if _, err := session.Render("{{next}}"); err == nil {
					t.Fatal("next option published after failure/cancellation")
				}
			}
		})
	}
}

// GO-582: pre-register all marked literals, even overwritten and future ones.
func TestOptionVariableProtectionBeforeCallbacks(t *testing.T) {
	for _, mode := range []string{"success", "mismatch", "conflict", "future sensitive capture"} {
		t.Run(mode, func(t *testing.T) {
			secrets := []string{"token"}
			if mode == "future sensitive capture" {
				secrets = nil
			}
			c, _ := template.New([]string{"token=initial-private", "root.child=initial"}, secrets)
			session := c.NewSession()
			source := "GET http://example.test/{{token}}\n[Options]\nvariable: token=discarded-private\nvariable: token=assigned-private\n"
			if mode == "conflict" {
				source += "variable: root=conflict-private\n"
			}
			source += "HTTP 200\n[Asserts]\njsonpath \"$.value\" == \"expected\"\nGET http://example.test\n[Options]\nvariable: token=future-private\nHTTP 200"
			if mode == "future sensitive capture" {
				source += "\n[Captures]\ntoken: header \"Authorization\""
			}
			if mode == "success" {
				source = strings.Replace(source, `== "expected"`, `== "assigned-private"`, 1)
			}
			calls, callbacks := 0, 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(`{"value":"assigned-private"}`)}, nil
			}), observedHeaderSession{session}, func(e EntryResult) error {
				callbacks++
				for _, secret := range []string{"initial-private", "discarded-private", "assigned-private", "future-private"} {
					if session.Sanitize(secret) != "[oculto]" {
						t.Fatalf("unprotected %s", secret)
					}
				}
				raw, _ := json.Marshal(e)
				if strings.Contains(string(raw), "private") || e.Entry.Options.Variables != nil {
					t.Fatalf("unsafe callback=%s", raw)
				}
				return nil
			}).Run(context.Background(), dynamicScenario(t, source))
			wantCalls := 1
			if mode == "success" {
				wantCalls = 2
			}
			if mode == "conflict" {
				wantCalls = 0
			}
			if calls != wantCalls || callbacks == 0 {
				t.Fatalf("calls=%d callbacks=%d result=%+v", calls, callbacks, got)
			}
			raw, _ := json.Marshal(got.Entries)
			if strings.Contains(string(raw), "private") {
				t.Fatalf("unsafe entries=%s", raw)
			}
		})
	}
}

func TestOptionVariablesCancelledBeforePublication(t *testing.T) {
	c, _ := template.New([]string{"id=initial"}, nil)
	session := &optionPublishSession{Session: c.NewSession()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
		t.Fatal("HTTP after cancellation")
		return Response{}, nil
	}), observedHeaderSession{session}, nil).Run(ctx, pollingScenario(t, "GET http://example.test\n[Options]\nvariable: id=assigned"))
	value, _ := session.Render("{{id}}")
	if !errors.Is(got.Err, context.Canceled) || session.publications != 0 || value.Value != "initial" {
		t.Fatalf("result=%+v publications=%d value=%+v", got, session.publications, value)
	}
}
