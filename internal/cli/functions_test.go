package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/runner"
)

const cliFunctionDate = "2025-11-16T09:34:56-03:00"

var cliFunctionUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func cliClock() func() time.Time {
	zone := time.FixedZone("-03", -3*60*60)
	return func() time.Time { return time.Date(2025, 11, 16, 9, 34, 56, 700, zone) }
}

// recordingExecutor keeps the requests it receives for wire assertions.
type recordingExecutor struct {
	requests []runner.Request
	statuses []int
}

func (r *recordingExecutor) Execute(_ context.Context, request runner.Request) (runner.Response, error) {
	r.requests = append(r.requests, request)
	return runner.Response{Status: r.statuses[min(len(r.requests), len(r.statuses))-1]}, nil
}

func executeClocked(t *testing.T, fake runner.HTTPExecutor, args ...string) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	command := NewRootCommand(fake, BuildInfo{"1.0-test", "abc123", "2026-11-14"})
	command.SetIn(strings.NewReader(""))
	command.SetOut(&out)
	command.SetErr(&stderr)
	command.SetArgs(args)
	command.now = cliClock()
	return command.ExecuteContext(context.Background()), out.String(), stderr.String()
}

// GO-586: the documented demonstration generates values, validates 201 and exits 0.
func TestFunctionDemonstrationEndToEnd(t *testing.T) {
	file := writeScenario(t, "POST {{api.baseUrl}}/products\nContent-Type: application/json\n{\"requestId\":\"{{newUuid}}\",\"createdAt\":\"{{newDate}}\",\"label\":\"pedido-{{newUuid}}\"}\nHTTP 201")
	recorder := &recordingExecutor{statuses: []int{201}}
	code, out, stderr := executeClocked(t, recorder, "test", file, "--var", "api.baseUrl=http://example.test")
	if code != 0 || len(recorder.requests) != 1 || stderr != "" {
		t.Fatalf("code=%d requests=%d stderr=%q", code, len(recorder.requests), stderr)
	}
	request := recorder.requests[0]
	if request.URL != "http://example.test/products" || len(request.Headers) != 1 || request.Headers[0].Value != "application/json" {
		t.Fatalf("request=%+v", request)
	}
	var body struct {
		RequestID string `json:"requestId"`
		CreatedAt string `json:"createdAt"`
		Label     string `json:"label"`
	}
	if err := json.Unmarshal(request.Body, &body); err != nil {
		t.Fatalf("body=%s err=%v", request.Body, err)
	}
	if !cliFunctionUUID.MatchString(body.RequestID) || body.CreatedAt != cliFunctionDate || !strings.HasPrefix(body.Label, "pedido-") {
		t.Fatalf("body=%+v", body)
	}
	label := strings.TrimPrefix(body.Label, "pedido-")
	if !cliFunctionUUID.MatchString(label) || label == body.RequestID {
		t.Fatalf("occurrences reused a value: %+v", body)
	}
	if !strings.Contains(out, "status validado") || strings.Contains(out, body.RequestID) || strings.Contains(stderr, body.RequestID) {
		t.Fatalf("out=%q err=%q", out, stderr)
	}
}

// GO-587: occurrences and runs always produce new values, composed with text and variables.
func TestFunctionValuesAreFreshPerOccurrenceAndRun(t *testing.T) {
	file := writeScenario(t, "GET {{api.baseUrl}}/items/{{newUuid}}?first={{newUuid}}&at={{newDate}}&tag={{tag}}-{{newUuid}}\nHTTP 200")
	var collected []string
	for run := 0; run < 2; run++ {
		recorder := &recordingExecutor{statuses: []int{200}}
		code, _, stderr := executeClocked(t, recorder, "test", file, "--var", "api.baseUrl=http://example.test", "--var", "tag=suite")
		if code != 0 || len(recorder.requests) != 1 || stderr != "" {
			t.Fatalf("run=%d code=%d requests=%d stderr=%q", run, code, len(recorder.requests), stderr)
		}
		url := recorder.requests[0].URL
		path := strings.TrimPrefix(strings.SplitN(url, "?", 2)[0], "http://example.test/items/")
		query := strings.SplitN(url, "?", 2)[1]
		values := strings.Split(query, "&")
		first := strings.TrimPrefix(values[0], "first=")
		at := strings.TrimPrefix(values[1], "at=")
		tag := strings.TrimPrefix(values[2], "tag=")
		if !cliFunctionUUID.MatchString(path) || !cliFunctionUUID.MatchString(first) || path == first {
			t.Fatalf("run=%d url=%q", run, url)
		}
		if at != cliFunctionDate {
			t.Fatalf("run=%d date=%q", run, at)
		}
		if strings.Count(tag, "suite-") != 1 || !cliFunctionUUID.MatchString(strings.TrimPrefix(tag, "suite-")) {
			t.Fatalf("run=%d tag=%q", run, tag)
		}
		collected = append(collected, path, first)
	}
	if collected[0] == collected[2] || collected[1] == collected[3] {
		t.Fatalf("runs reused values: %v", collected)
	}
}

// GO-592: reserved names are rejected before any request, with a safe position.
func TestReservedFunctionNamesFailWithCodeTwo(t *testing.T) {
	capture := writeScenario(t, "GET http://example.test/first\nHTTP 200\nGET http://example.test/second\nHTTP 200\n[Captures]\nnewUuid: jsonpath \"$.id\"")
	optionVariable := writeScenario(t, "GET http://example.test\n[Options]\nvariable: newDate=literal\nHTTP 200")
	config := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(config, []byte("version: 1\nvariables:\n  newDate: literal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		args    []string
		wantPos string
	}{
		{"cli variable date", []string{"test", writeScenario(t, "GET http://example.test\nHTTP 200"), "--var", "newDate=2025-01-01T00:00:00Z"}, ""},
		{"cli variable uuid", []string{"test", writeScenario(t, "GET http://example.test\nHTTP 200"), "--var", "newUuid=00000000-0000-4000-8000-000000000000"}, ""},
		{"secret uuid", []string{"test", writeScenario(t, "GET http://example.test\nHTTP 200"), "--secret", "newUuid"}, ""},
		{"configuration variable", []string{"test", "--config", config, writeScenario(t, "GET http://example.test\nHTTP 200")}, ""},
		{"capture", []string{"test", capture}, ":6:1"},
		{"options variable", []string{"test", optionVariable}, ":3:1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeExecutor{}
			code, out, stderr := executeClocked(t, fake, tc.args...)
			if code != 2 || len(fake.calls) != 0 || !strings.Contains(stderr, "nome reservado para função embutida (newDate, newUuid)") {
				t.Fatalf("code=%d calls=%v out=%q err=%q", code, len(fake.calls), out, stderr)
			}
			if tc.wantPos != "" && !strings.Contains(stderr, tc.wantPos) {
				t.Fatalf("stderr lacks position %q: %q", tc.wantPos, stderr)
			}
			if strings.Contains(stderr, "00000000-0000-4000") || strings.Contains(stderr, "2025-01-01T00:00:00Z") {
				t.Fatalf("diagnostic echoed the value: %q", stderr)
			}
		})
	}
}

// GO-594: generated text keeps final URL validation, without implicit encoding
// or sending later entries. The functions themselves only produce safe text.
func TestFunctionGeneratedValuesKeepFinalValidation(t *testing.T) {
	for _, tc := range []struct{ name, text string }{
		{"relative URL", "GET {{newUuid}}\nHTTP 200"},
		{"invalid port", "GET http://example.test:{{newUuid}}/health\nHTTP 200"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := writeScenario(t, "GET http://example.test/first\nHTTP 200\n"+tc.text+"\nGET http://example.test/last\nHTTP 200")
			recorder := &recordingExecutor{statuses: []int{200}}
			code, _, stderr := executeClocked(t, recorder, "test", file)
			if code != 2 || len(recorder.requests) != 1 || stderr == "" {
				t.Fatalf("code=%d requests=%d stderr=%q", code, len(recorder.requests), stderr)
			}
			if strings.Contains(stderr, "%2F") || strings.Contains(stderr, "%20") || strings.Contains(stderr, "=") && strings.Contains(stderr, "newUuid=") {
				t.Fatalf("implicit encoding or unlocalized diagnostic: %q", stderr)
			}
			for _, request := range recorder.requests {
				if strings.Contains(request.URL, "%") || strings.ContainsAny(request.URL, "\r\n") {
					t.Fatalf("implicit encoding or injected bytes: %q", request.URL)
				}
			}
		})
	}
	// Generated values stay valid in headers and are inserted as plain text.
	file := writeScenario(t, "GET http://example.test/health\nX-Probe: {{newUuid}}-{{newDate}}\nHTTP 200")
	recorder := &recordingExecutor{statuses: []int{200}}
	if code, _, stderr := executeClocked(t, recorder, "test", file); code != 0 || len(recorder.requests) != 1 || stderr != "" {
		t.Fatalf("code=%d requests=%d stderr=%q", code, len(recorder.requests), stderr)
	}
	probe := recorder.requests[0].Headers[0].Value
	if !strings.HasPrefix(probe, "") || !strings.HasSuffix(probe, "-"+cliFunctionDate) {
		t.Fatalf("probe=%q", probe)
	}
}

// GO-596: static scenarios are unchanged and generated values never become secrets.
func TestFunctionSecurityRegressions(t *testing.T) {
	static := writeScenario(t, "GET http://example.test/health\nHTTP 200")
	fake := &fakeExecutor{statuses: []int{200}}
	code, out, stderr := executeClocked(t, fake, "test", static)
	if code != 0 || len(fake.calls) != 1 || stderr != "" || !strings.Contains(out, "status validado") {
		t.Fatalf("code=%d calls=%v out=%q err=%q", code, len(fake.calls), out, stderr)
	}
	const secret = "fictitious-secret-value"
	withSecret := writeScenario(t, "GET http://example.test/{{newUuid}}?token={{token}}\nHTTP 200")
	fake = &fakeExecutor{statuses: []int{200}}
	code, out, stderr = executeClocked(t, fake, "test", withSecret, "--var", "token="+secret, "--secret", "token")
	if code != 0 || len(fake.calls) != 1 {
		t.Fatalf("code=%d calls=%v out=%q err=%q", code, len(fake.calls), out, stderr)
	}
	combined := out + stderr
	if strings.Contains(combined, secret) || strings.Contains(combined, "[oculto]") {
		t.Fatalf("generated value changed protection: %q", combined)
	}
	if strings.Contains(combined, fake.calls[0]) {
		t.Fatalf("URL echoed: %q", combined)
	}
}
