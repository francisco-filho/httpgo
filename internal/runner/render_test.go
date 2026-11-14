package runner

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func TestRenderSequenceStopsBeforeSend(t *testing.T) {
	for _, tc := range []struct {
		name, url, header, host string
		assignments             []string
		line                    int
	}{
		{name: "missing", url: "{{missing}}", line: 3},
		{name: "relative", url: "{{url}}", assignments: []string{"url=/relative"}, line: 3},
		{name: "scheme", url: "{{url}}", assignments: []string{"url=ftp://example.test"}, line: 3},
		{name: "host", url: "{{url}}", assignments: []string{"url=http://:80"}, line: 3},
		{name: "port", url: "{{url}}", assignments: []string{"url=http://example.test:65536"}, line: 3},
		{name: "encoding", url: "{{url}}", assignments: []string{"url=http://example.test/%zz"}, line: 3},
		{name: "no implicit encoding", url: "http://example.test/{{value}}", assignments: []string{"value=not encoded"}, line: 3},
		{name: "CRLF", url: "http://example.test/", header: "{{value}}", assignments: []string{"value=fake\r\nInjected: yes"}, line: 4},
		{name: "control", url: "http://example.test/", header: "{{value}}", assignments: []string{"value=fake\x7f"}, line: 4},
		{name: "Host", url: "http://example.test/", host: "{{value}}", assignments: []string{"value=user:fake@example.test"}, line: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "GET http://example.test/first\nHTTP 200\nGET " + tc.url + "\n"
			if tc.header != "" {
				input += "X-Probe: " + tc.header + "\n"
			}
			if tc.host != "" {
				input += "Host: " + tc.host + "\n"
			}
			input += "GET http://example.test/last"
			scenario, err := parser.Parse("safe.hurl", strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			renderer, err := template.New(tc.assignments, []string{"value", "url"})
			if err != nil {
				t.Fatal(err)
			}
			calls, reports := 0, 0
			got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { calls++; return Response{200}, nil }), renderer, func(EntryResult) error { reports++; return nil }).Run(context.Background(), scenario)
			var located *RenderError
			if !errors.As(got.Err, &located) || located.Position.Line != tc.line || calls != 1 || reports != 2 || got.Summary != (Summary{3, 1, 1, 1, 0, 2}) || len(got.Entries) != 2 || got.Entries[1].Outcome != RenderFailed {
				t.Fatalf("result=%+v, calls=%d reports=%d", got, calls, reports)
			}
			if strings.Contains(got.Err.Error(), "fake") {
				t.Fatalf("unsafe error: %v", got.Err)
			}
		})
	}
}

func TestRenderPreservesScenarioAndText(t *testing.T) {
	input := "POST {{base}}/{{ path }}?q=a/b\nX-Probe: pre-{{value}}-{{empty}}\nHost: {{host}}\n{ \"unchanged\": true }\nHTTP 201"
	scenario, err := parser.Parse("input.hurl", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	before, err := parser.Parse("input.hurl", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := template.New([]string{"base=http://example.test", "path=a%2Fb", "value={{literal}}=true", "empty=", "host=example.test:8080"}, []string{"path"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	executor := executorFunc(func(_ context.Context, req Request) (Response, error) {
		calls++
		if req.URL != "http://example.test/a%2Fb?q=a/b" || req.Headers[0].Value != "pre-{{literal}}=true-" || req.Headers[1].Value != "example.test:8080" || string(req.Body) != `{ "unchanged": true }` {
			t.Fatalf("request=%+v", req)
		}
		req.Headers[0].Value = "mutated"
		req.Body[0] = 'X'
		return Response{201}, nil
	})
	got := NewWithRenderer(executor, renderer, nil).Run(context.Background(), scenario)
	if got.Err != nil || calls != 1 || strings.Contains(got.Entries[0].DisplayURL, "a%2Fb") || !reflect.DeepEqual(before, scenario) {
		t.Fatalf("result=%+v scenario=%+v", got, scenario)
	}
}

func TestRenderReportFailure(t *testing.T) {
	renderer, _ := template.New(nil, nil)
	reportErr := errors.New("write failed")
	got := NewWithRenderer(executorFunc(func(context.Context, Request) (Response, error) { t.Fatal("unexpected send"); return Response{}, nil }), renderer, func(EntryResult) error { return reportErr }).Run(context.Background(), parser.Scenario{Entries: []parser.Entry{{URL: "{{missing}}"}, {URL: "http://example.test"}}})
	if got.ReportErr != reportErr || got.Summary.Started != 0 || got.Summary.NotStarted != 2 {
		t.Fatalf("result=%+v", got)
	}
}
