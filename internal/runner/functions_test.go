package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

const functionDate = "2025-11-16T09:34:56-03:00"

var functionUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func functionContext(t *testing.T, assignments, secrets []string) *template.Context {
	t.Helper()
	zone := time.FixedZone("-03", -3*60*60)
	c, err := template.NewWithClock(assignments, secrets, func() time.Time {
		return time.Date(2025, 11, 16, 9, 34, 56, 500, zone)
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// GO-590: every retry attempt resends the same rendered URL and body.
func TestFunctionsResolveOncePerEntryForRetries(t *testing.T) {
	scenario := pollingScenario(t, "POST http://example.test/jobs/{{newUuid}}\nContent-Type: application/json\n{\"id\":\"{{newUuid}}\",\"at\":\"{{newDate}}\"}\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n")
	var requests []Request
	r := NewWithRenderer(executorFunc(func(_ context.Context, request Request) (Response, error) {
		requests = append(requests, request)
		if len(requests) < 3 {
			return Response{Status: 503}, nil
		}
		return Response{Status: 200}, nil
	}), functionContext(t, nil, nil), nil)
	r.wait = func(context.Context, time.Duration) error { return nil }
	result := r.Run(context.Background(), scenario)
	if result.Err != nil || result.Diverged || len(requests) != 3 || result.Attempts.Started != 3 {
		t.Fatalf("result=%+v requests=%d", result, len(requests))
	}
	for _, attempt := range requests[1:] {
		if !reflect.DeepEqual(requests[0], attempt) {
			t.Fatalf("attempts differ: %+v %+v", requests[0], attempt)
		}
	}
	urlUUID := strings.TrimPrefix(requests[0].URL, "http://example.test/jobs/")
	if !functionUUID.MatchString(urlUUID) {
		t.Fatalf("url=%q", requests[0].URL)
	}
	var body struct {
		ID string `json:"id"`
		At string `json:"at"`
	}
	if err := json.Unmarshal(requests[0].Body, &body); err != nil {
		t.Fatalf("body=%s err=%v", requests[0].Body, err)
	}
	if !functionUUID.MatchString(body.ID) || body.ID == urlUUID || body.At != functionDate {
		t.Fatalf("body=%+v urlUUID=%q", body, urlUUID)
	}
}

// GO-590: attempts after a status divergence reuse the same rendered request.
func TestFunctionsResolveOncePerEntryAfterDivergence(t *testing.T) {
	scenario := pollingScenario(t, "POST http://example.test/jobs/{{newUuid}}\nContent-Type: application/json\n{\"id\":{{newUuid}}}\n[Options]\nretry: 2\nretry-interval: 0\nHTTP 200\n")
	var attempts []Request
	calls := 0
	r := NewWithRenderer(executorFunc(func(_ context.Context, request Request) (Response, error) {
		calls++
		attempts = append(attempts, request)
		if calls == 1 {
			return Response{Status: 503}, nil
		}
		return Response{Status: 200}, nil
	}), functionContext(t, nil, nil), nil)
	r.wait = func(context.Context, time.Duration) error { return nil }
	result := r.Run(context.Background(), scenario)
	if result.Err != nil || result.Diverged || len(attempts) != 2 || result.Attempts.Started != 2 {
		t.Fatalf("result=%+v attempts=%d", result, len(attempts))
	}
	if !reflect.DeepEqual(attempts[0], attempts[1]) {
		t.Fatalf("attempts differ: %+v %+v", attempts[0], attempts[1])
	}
}

// GO-590: textual multipart fields keep their prepared value across attempts.
func TestFunctionsResolveOncePerMultipartEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "upload"), []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	scenario, err := parser.Parse("multipart.hurl", strings.NewReader("POST http://example.test/jobs\n[Options]\nretry: 1\nretry-interval: 0\n[MultipartFormData]\nlabel: pedido-{{newUuid}}\nfile: file,upload;\nHTTP 200\n"))
	if err != nil {
		t.Fatal(err)
	}
	scenario.OriginDir = dir
	var parts [][]MultipartPart
	r := NewWithRenderer(executorFunc(func(_ context.Context, request Request) (Response, error) {
		parts = append(parts, request.Multipart.Parts)
		if len(parts) == 1 {
			return Response{Status: 503}, nil
		}
		return Response{Status: 200}, nil
	}), functionContext(t, nil, nil), nil)
	r.wait = func(context.Context, time.Duration) error { return nil }
	result := r.Run(context.Background(), scenario)
	if result.Err != nil || result.Diverged || len(parts) != 2 || result.Attempts.Started != 2 {
		t.Fatalf("result=%+v parts=%d", result, len(parts))
	}
	if !reflect.DeepEqual(parts[0], parts[1]) {
		t.Fatalf("multipart differ: %+v %+v", parts[0], parts[1])
	}
	label := parts[0][0]
	if !strings.HasPrefix(label.Value, "pedido-") || !functionUUID.MatchString(strings.TrimPrefix(label.Value, "pedido-")) {
		t.Fatalf("part=%+v", label)
	}
}

// GO-595: preparation failures never count an attempt or reach the executor.
func TestFunctionsPreparationFailureNeverStarts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "upload"), []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, input, wantDetail string
		renderFiles             bool
	}{
		{
			name:       "multipart text missing variable",
			input:      "POST http://example.test/jobs\n[Options]\nretry: 2\nretry-interval: 0\n[MultipartFormData]\nlabel: {{missing}}\nfile: file,upload;\nHTTP 200\n",
			wantDetail: "ausente",
		},
		{
			name:        "external rendered body missing variable",
			input:       "POST http://example.test/jobs\n[Options]\nretry: 2\nretry-interval: 0\nfile,payload.json;\nHTTP 200\n",
			wantDetail:  "ausente",
			renderFiles: true,
		},
		{
			name:       "inline body missing variable",
			input:      "POST http://example.test/jobs\nContent-Type: application/json\n[Options]\nretry: 2\nretry-interval: 0\n{\"at\":\"{{missing}}\"}\nHTTP 200\n",
			wantDetail: "ausente",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario, err := parser.Parse("fail.hurl", strings.NewReader("GET http://example.test/first\nHTTP 200\n"+tc.input))
			if err != nil {
				t.Fatal(err)
			}
			scenario.OriginDir = dir
			calls := 0
			r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200}, nil
			}), functionContext(t, nil, nil), nil)
			r.readFile = func(context.Context, string) ([]byte, error) { return []byte(`{"at":"{{missing}}"}`), nil }
			r.wait = func(context.Context, time.Duration) error { return nil }
			result := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: tc.renderFiles})
			if result.Err == nil || calls != 1 || result.Summary.Started != 1 || result.Summary.Completed != 1 || result.Attempts.Started != 1 {
				t.Fatalf("result=%+v calls=%d", result, calls)
			}
			if !strings.Contains(result.Err.Error(), tc.wantDetail) {
				t.Fatalf("err=%v", result.Err)
			}
			if len(result.Entries) != 2 || result.Entries[0].Outcome != Passed || result.Entries[1].Outcome != RenderFailed || result.Entries[1].Attempts != 0 {
				t.Fatalf("entries=%+v", result.Entries)
			}
		})
	}
}

// GO-596 / RN-07: reserved names are rejected before any request is sent.
func TestReservedFunctionNamesAreRejectedBeforeSend(t *testing.T) {
	for _, tc := range []struct{ name, input string }{
		{"capture", "GET http://example.test/jobs\nHTTP 200\n[Captures]\nnewDate: jsonpath \"$.at\"\n"},
		{"option variable", "GET http://example.test/jobs\n[Options]\nvariable: newUuid=literal\nHTTP 200\n"},
		{"blocked after valid entry", "GET http://example.test/first\nHTTP 200\nGET http://example.test/second\nHTTP 200\n[Captures]\nnewUuid: jsonpath \"$.id\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario, err := parser.Parse("reserved.hurl", strings.NewReader(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			r := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) {
				calls++
				return Response{Status: 200, Body: []byte(`{"id":"x","at":"y"}`)}, nil
			}), functionContext(t, nil, nil), nil)
			result := r.Run(context.Background(), scenario)
			if calls != 0 || result.Summary.Started != 0 || result.Summary.NotStarted != len(scenario.Entries) || result.Attempts != (AttemptSummary{}) {
				t.Fatalf("calls=%d result=%+v", calls, result)
			}
			var failure *RenderError
			if !errors.As(result.Err, &failure) || !strings.Contains(failure.Detail, template.ReservedDetail) || result.ErrorPosition != failure.Position {
				t.Fatalf("err=%v failure=%+v", result.Err, failure)
			}
		})
	}
	for _, name := range []string{"newUuid.value", "newDate_x"} {
		t.Run("allowed "+name, func(t *testing.T) {
			scenario, err := parser.Parse("descendant.hurl", strings.NewReader("GET http://example.test/jobs\nHTTP 200\n[Captures]\n"+name+": jsonpath \"$.id\"\n[Asserts]\njsonpath \"$.id\" == \"x\"\nGET http://example.test/{{ "+name+" }}\nHTTP 200\n"))
			if err != nil {
				t.Fatal(err)
			}
			var urls []string
			r := NewWithRenderer(executorFunc(func(_ context.Context, request Request) (Response, error) {
				urls = append(urls, request.URL)
				return Response{Status: 200, Body: []byte(`{"id":"x"}`)}, nil
			}), functionContext(t, nil, nil), nil)
			result := r.Run(context.Background(), scenario)
			if len(urls) != 2 || result.Err != nil || result.Diverged || result.Entries[0].Outcome != Passed {
				t.Fatalf("urls=%v result=%+v", urls, result)
			}
			if urls[1] != "http://example.test/x" {
				t.Fatalf("descendant capture not resolvable: %v", urls)
			}
		})
	}
}

// GO-591: functions resolve in query and assertion operands like variables.
func TestFunctionsResolveInQueryOperands(t *testing.T) {
	scenario, err := parser.ParseWithOptions("operands.hurl", strings.NewReader("GET http://example.test/jobs\nHTTP 200\n[Captures]\nwhen: jsonpath \"$.when\"\n[Asserts]\njsonpath \"$.when\" == \"{{newDate}}\"\njsonpath \"$.id\" == \"{{newDate}}\"\n"), parser.ParseOptions{RenderAsserts: true})
	if err != nil {
		t.Fatal(err)
	}
	var requests []Request
	r := NewWithRenderer(executorFunc(func(_ context.Context, request Request) (Response, error) {
		requests = append(requests, request)
		return Response{Status: 200, Body: []byte(`{"id":"` + functionDate + `","when":"` + functionDate + `"}`)}, nil
	}), functionContext(t, nil, nil), nil)
	result := r.Run(context.Background(), scenario)
	if result.Err != nil || result.Diverged || len(requests) != 1 {
		t.Fatalf("result=%+v requests=%d", result, len(requests))
	}
	if len(result.Entries) != 1 || result.Entries[0].Outcome != Passed {
		t.Fatalf("entries=%+v", result.Entries)
	}
	if _, err := functionContext(t, nil, nil).Render(""); err != nil {
		t.Fatal(err)
	}
}

// GO-596: generated values never become secrets and never mask each other.
func TestFunctionsAreNotSecret(t *testing.T) {
	scenario, err := parser.Parse("secret.hurl", strings.NewReader("GET http://example.test/{{newUuid}}?token={{token}}\nHTTP 200\n"))
	if err != nil {
		t.Fatal(err)
	}
	var requests []Request
	r := NewWithRenderer(executorFunc(func(_ context.Context, request Request) (Response, error) {
		requests = append(requests, request)
		return Response{Status: 200}, nil
	}), functionContext(t, []string{"token=fictitious-token"}, []string{"token"}), nil)
	result := r.Run(context.Background(), scenario)
	if result.Err != nil || len(requests) != 1 || len(result.Entries) != 1 {
		t.Fatalf("result=%+v requests=%d", result, len(requests))
	}
	display := result.Entries[0].DisplayURL
	generated := strings.TrimPrefix(strings.SplitN(display, "?", 2)[0], "http://example.test/")
	if !functionUUID.MatchString(generated) || strings.Contains(display, "fictitious-token") {
		t.Fatalf("display=%q", display)
	}
}
