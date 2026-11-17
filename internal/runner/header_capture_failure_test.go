package runner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

type observedHeaderSession struct{ session template.Session }

func (r observedHeaderSession) NewSession() template.Session { return r.session }

func TestHeaderCaptureFailuresNeverPublish(t *testing.T) {
	for _, mode := range []string{"status", "assertion", "JSON evaluation", "transport", "mixed", "collision"} {
		t.Run(mode, func(t *testing.T) {
			renderer, _ := template.New([]string{"initial=unchanged"}, nil)
			session := renderer.NewSession()
			declarations := "x: header \"Set-Cookie\"\n"
			if mode == "mixed" {
				declarations += "y: jsonpath \"$.missing\"\n"
			}
			if mode == "collision" {
				declarations += "initial.child: header \"Set-Cookie\"\n"
			}
			source := "GET http://example.test/first\nHTTP 200\n[Captures]\n" + declarations
			if mode == "assertion" {
				source += "[Asserts]\nheader \"Other\" exists\n"
			}
			if mode == "JSON evaluation" {
				source += "[Asserts]\njsonpath \"$\" == 1\n"
			}
			source += "GET http://example.test/next"
			calls := 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				response := Response{Status: 200, Body: []byte(`{}`), Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "private-batch"}}}
				if mode == "status" {
					response.Status = 503
				}
				if mode == "JSON evaluation" {
					response.Body = []byte("invalid")
				}
				if mode == "transport" {
					return response, errors.New("read failed")
				}
				return response, nil
			}), observedHeaderSession{session}, nil).Run(context.Background(), parseCaptureScenario(t, source))
			if calls != 1 || got.Err == nil && !got.Diverged {
				t.Fatalf("%+v calls=%d", got, calls)
			}
			if _, err := session.Render("{{x}}"); err == nil {
				t.Fatal("published after failure")
			}
			if text, err := session.Render("{{initial}}"); err != nil || text.Value != "unchanged" {
				t.Fatal("overwrote initial value")
			}
			if session.Sanitize("private-batch") == "private-batch" {
				t.Fatal("lost protection after rejected batch")
			}
		})
	}
}

func TestHeaderCaptureReportFailurePreservesCounts(t *testing.T) {
	for _, mode := range []string{"success", "mismatch", "missing"} {
		t.Run(mode, func(t *testing.T) {
			source := "GET http://example.test/first\nHTTP 200\n[Captures]\nx: header \"Set-Cookie\"\nGET http://example.test/next"
			calls, reports := 0, 0
			cause := errors.New("write failed")
			got := New(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				response := Response{Status: 200, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "private-cookie"}}}
				if mode == "mismatch" {
					response.Status = 503
				}
				if mode == "missing" {
					response.Headers = nil
				}
				return response, nil
			}), func(EntryResult) error { reports++; return cause }).Run(context.Background(), parseCaptureScenario(t, source))
			validated := 1
			if mode == "mismatch" {
				validated = 0
			}
			if !errors.Is(got.ReportErr, cause) || calls != 1 || reports != 1 || got.Summary != (Summary{2, 1, 1, validated, 0, 1}) {
				t.Fatalf("%+v calls=%d reports=%d", got, calls, reports)
			}
		})
	}
}

func TestHeaderCaptureReusedRunnerStartsClean(t *testing.T) {
	renderer, _ := template.New(nil, []string{"explicit"})
	source := "GET http://example.test/first\n[Captures]\nx: header \"Set-Cookie\"\nGET http://example.test/next\nX-Value: {{x}}"
	scenario := parseCaptureScenario(t, source)
	before := parseCaptureScenario(t, source)
	calls := 0
	r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
		calls++
		if strings.HasSuffix(req.URL, "/next") && req.Headers[0].Value != "private-cookie" {
			t.Fatal("value changed")
		}
		return Response{Status: 200, Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "private-cookie"}}}, nil
	}), renderer, nil)
	for range 2 {
		got := r.Run(context.Background(), scenario)
		if got.Err != nil || got.Diverged {
			t.Fatalf("%+v", got)
		}
		if renderer.IsSecret("x") || renderer.Sanitize("private-cookie") != "private-cookie" {
			t.Fatal("dynamic state escaped")
		}
	}
	if calls != 4 || !reflect.DeepEqual(scenario, before) {
		t.Fatal("scenario mutated or context reused")
	}
	got := r.Run(context.Background(), parseCaptureScenario(t, "GET http://example.test/early\nX-Value: {{x}}\nGET http://example.test/later\n[Captures]\nx: header \"Set-Cookie\""))
	if got.Err == nil || calls != 4 || got.Summary.Started != 0 {
		t.Fatalf("%+v calls=%d", got, calls)
	}
}

func TestMixedHeaderCapturePublishesTypedValues(t *testing.T) {
	renderer, _ := template.New(nil, nil)
	session := renderer.NewSession()
	scenario := parseCaptureScenario(t, "GET http://example.test/\n[Captures]\nx: header \"X\"\ny: jsonpath \"$\"")
	if err := captureResponse(session, scenario.Entries[0].Captures, Response{Body: []byte(`9007199254740993`), Headers: []query.HeaderValue{{Name: "X", Value: "123"}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := session.RenderJSON([]byte(`{"x":{{x}},"y":{{y}}}`))
	if err != nil || string(raw) != `{"x":"123","y":9007199254740993}` {
		t.Fatalf("%s %v", raw, err)
	}
	// Ancestor and descendant conflicts with previously published captures keep both values.
	for _, name := range []string{"x.child"} {
		if err := session.Publish([]template.Capture{{Name: name, Value: json.RawMessage(`"replacement"`)}}); err == nil {
			t.Fatal("overwrite accepted")
		}
	}
}
