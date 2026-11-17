package runner

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/template"
)

// GO-439,442,443: partial responses never publish; reporting runs on safe metadata.
func TestCountCaptureBoundaryFailures(t *testing.T) {
	for _, mode := range []string{"transport", "cancel", "batch", "collision", "pending assertion", "report success", "report mismatch", "report capture"} {
		t.Run(mode, func(t *testing.T) {
			source := "GET http://example.test/first\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\n"
			if mode == "batch" || mode == "report capture" {
				source += "other: jsonpath \"$.missing\"\n"
			}
			if mode == "collision" {
				source += "initial.child: jsonpath \"$\" count\n"
			}
			if mode == "pending assertion" {
				source += "[Asserts]\njsonpath \"$\" count == 98\n"
			}
			source += "GET http://example.test/next"
			s := parseCaptureScenario(t, source)
			c, _ := template.New([]string{"initial=7"}, []string{"baseline"})
			session := c.NewSession()
			cause := errors.New("private writer cause")
			calls, reports := 0, 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				response := Response{Status: 200, Body: []byte(`[null,null,null]`)}
				switch mode {
				case "transport":
					return response, cause
				case "cancel":
					return response, context.Canceled
				case "report mismatch":
					response.Status = 503
				}
				return response, nil
			}), observedHeaderSession{session}, func(e EntryResult) error {
				reports++
				if e.JSONFailure != nil && (e.JSONFailure.Expected != "[oculto]" || e.JSONFailure.Actual != "[oculto]") {
					t.Fatalf("unsafe pending failure: %+v", e.JSONFailure)
				}
				assertCountMetadata(t, e)
				if strings.HasPrefix(mode, "report") {
					return cause
				}
				return nil
			}).Run(context.Background(), s)
			if calls != 1 || reports != 1 || got.Summary.NotStarted != 1 {
				t.Fatalf("calls=%d reports=%d result=%+v", calls, reports, got)
			}
			if strings.HasPrefix(mode, "report") && !errors.Is(got.ReportErr, cause) {
				t.Fatalf("report cause=%v", got.ReportErr)
			}
			if mode == "transport" && (!errors.Is(got.Err, cause) || got.Summary.Completed != 0) {
				t.Fatalf("partial evaluated: %+v", got)
			}
			if mode == "cancel" && !errors.Is(got.Err, context.Canceled) {
				t.Fatalf("cancel cause=%v", got.Err)
			}
			if _, err := session.Render("{{baseline}}"); (err == nil) != (mode == "report success") {
				t.Fatalf("publication error=%v", err)
			}
			if mode == "batch" || mode == "collision" || mode == "report capture" {
				if session.Sanitize("3") != "[oculto]" {
					t.Fatal("rejected batch lost protection")
				}
			}
			if mode == "pending assertion" && session.Sanitize("3") != "[oculto]" {
				t.Fatal("discarded secret lost protection")
			}
			if text, err := session.Render("{{initial}}"); err != nil || text.Value != "7" {
				t.Fatalf("initial changed: %+v %v", text, err)
			}
			for _, e := range got.Entries {
				assertCountMetadata(t, e)
			}
		})
	}
}

func assertCountMetadata(t *testing.T, e EntryResult) {
	t.Helper()
	if len(e.Entry.Captures) != 0 || len(e.Entry.Body) != 0 || len(e.Entry.Headers) != 0 || len(e.Entry.BodyPlan.References) != 0 {
		t.Fatalf("operational entry retained: %+v", e.Entry)
	}
	for _, a := range e.Entry.Assertions {
		if a.JSON != nil && (a.JSON.Expected != nil || a.JSON.ExpectedTemplate != nil || a.JSON.Compiled != nil) {
			t.Fatalf("operational expected retained: %+v", a.JSON)
		}
	}
	data, _ := json.Marshal(e)
	if strings.Contains(string(data), "[null") {
		t.Fatalf("operational data: %s", data)
	}
}

// GO-439: different actual counts are concealed by provenance, not replacement.
func TestCountCaptureSecretComparisonCallback(t *testing.T) {
	for _, path := range []string{"$", "$[*]"} {
		source := "GET http://example.test/before\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\nGET http://example.test/after\nHTTP 200\n[Asserts]\njsonpath \"" + path + "\" count == {{baseline}}\nGET http://example.test/end"
		s := dynamicScenario(t, source)
		c, _ := template.New(nil, []string{"baseline"})
		calls, reports := 0, 0
		got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
			calls++
			body := `[1,2,3]`
			if calls == 2 {
				body = `[1,2,3,4]`
			}
			return Response{Status: 200, Body: []byte(body)}, nil
		}), c, func(e EntryResult) error {
			reports++
			assertCountMetadata(t, e)
			if e.JSONFailure != nil && (e.JSONFailure.Expected != "[oculto]" || e.JSONFailure.Actual != "[oculto]" || e.JSONFailure.Detail != "") {
				t.Fatalf("unsafe: %+v", e.JSONFailure)
			}
			return nil
		}).Run(context.Background(), s)
		if got.Err != nil || !got.Diverged || calls != 2 || reports != 2 {
			t.Fatalf("result=%+v calls=%d reports=%d", got, calls, reports)
		}
	}
}

// GO-444: repeated runs recreate the baseline; another file cannot resolve it.
func TestCountCaptureRunnerIsolation(t *testing.T) {
	source := "GET http://example.test/before\nHTTP 200\n[Captures]\nbaseline: jsonpath \"$\" count\nPOST http://example.test/after\n{\"count\":{{baseline}},\"initial\":{{initial}}}\nHTTP 200"
	s := parseCaptureScenario(t, source)
	before := parseCaptureScenario(t, source)
	c, _ := template.New([]string{"initial=7"}, []string{"baseline"})
	calls := 0
	r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
		calls++
		if calls%2 == 0 && string(req.Body) != `{"count":3,"initial":7}` {
			t.Fatalf("body=%s", req.Body)
		}
		return Response{Status: 200, Body: []byte(`[null,null,null]`)}, nil
	}), c, func(e EntryResult) error { assertCountMetadata(t, e); return nil })
	for range 2 {
		got := r.Run(context.Background(), s)
		if got.Err != nil || got.Diverged || got.Summary.Completed != 2 || !reflect.DeepEqual(s, before) {
			t.Fatalf("result=%+v mutated=%v", got, !reflect.DeepEqual(s, before))
		}
	}
	got := r.Run(context.Background(), parseCaptureScenario(t, "GET http://example.test/{{baseline}}"))
	if got.Err == nil || got.Summary.Started != 0 || calls != 4 || c.Sanitize("3") != "3" {
		t.Fatalf("state escaped: %+v calls=%d", got, calls)
	}
}
