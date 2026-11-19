package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"capimgrosso.com/httpgo/internal/template"
)

func diagnosticResponse(request Request, status int, body string) Response {
	requestState := BodyJSON
	if request.Body == nil {
		requestState = BodyAbsent
	}
	return Response{Status: status, Body: []byte(body), Diagnostics: &HTTPDiagnostics{Request: HTTPBody{State: requestState, Bytes: request.Body}, Response: HTTPBody{State: BodyJSON, Bytes: []byte(body)}}}
}

func TestBodyPreviewValidationAndLimit(t *testing.T) {
	c, _ := template.New(nil, nil)
	session := c.NewSession()
	for _, tc := range []struct {
		name, body, media string
		state             BodyState
	}{
		{"json", ` {"n":1} `, "", BodyJSON},
		{"invalid", `{"private":`, "application/json", BodyOmitted},
		{"two JSONs", `{} {}`, "", BodyOmitted},
		{"binary", "\xff{}", "", BodyOmitted},
		{"octet stream", `{}`, "application/octet-stream", BodyOmitted},
		{"multipart", `{}`, "multipart/form-data; boundary=abc", BodyOmitted},
		{"charset", `{}`, "application/json; charset=iso-8859-1", BodyOmitted},
		{"structured suffix", `{}`, "application/problem+json; charset=UTF-8", BodyJSON},
		{"empty file", "", "", BodyOmitted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := prepareBodyPreview(HTTPBody{State: BodyJSON, Bytes: []byte(tc.body), ContentType: tc.media}, session)
			if got.State != tc.state || tc.state != BodyJSON && got.Text != "" {
				t.Fatalf("preview=%+v", got)
			}
		})
	}
	for _, length := range []int{bodyPreviewLimit - 1, bodyPreviewLimit, bodyPreviewLimit + 1} {
		body := `"` + strings.Repeat("x", length-2) + `"`
		got := prepareBodyPreview(HTTPBody{State: BodyJSON, Bytes: []byte(body)}, session)
		if len(got.Text) != min(length, bodyPreviewLimit) || got.Truncated != (length > bodyPreviewLimit) {
			t.Fatalf("length=%d preview=%+v", length, got)
		}
	}
	body := `"` + strings.Repeat("x", bodyPreviewLimit-2) + "€" + `"`
	got := prepareBodyPreview(HTTPBody{State: BodyJSON, Bytes: []byte(body)}, session)
	if !got.Truncated || !utf8.ValidString(got.Text) || len(got.Text) != bodyPreviewLimit-1 {
		t.Fatalf("utf8 length=%d preview=%+v", len(got.Text), got)
	}
	for _, state := range []BodyState{BodyAbsent, BodyUnavailable, BodyOmitted, BodyProtected, "invalid"} {
		got := prepareBodyPreview(HTTPBody{State: state, Bytes: []byte(`{"private":true}`)}, session)
		if got.Text != "" || got.State == BodyJSON {
			t.Fatalf("unexpected state preview=%+v", got)
		}
	}
}

func TestBodyPreviewProtectsBeforeCutAndNormalizesEscapes(t *testing.T) {
	c, _ := template.New(nil, nil)
	s := c.NewSession()
	const secret = "fiction-secret-\"<&€"
	s.ProtectValue(secret)
	raw, _ := json.Marshal(secret)
	for _, value := range []string{string(raw), `"\u0066iction-secret-\"\u003c\u0026€"`} {
		body := `{"padding":"` + strings.Repeat("x", bodyPreviewLimit-35) + `","secret":` + value + `,"tail":"` + strings.Repeat("z", 60) + `"}`
		preview := prepareBodyPreview(HTTPBody{State: BodyJSON, Bytes: []byte(body)}, s)
		if !preview.Truncated || len(preview.Text) > bodyPreviewLimit || strings.Contains(preview.Text, "fiction-secret") || strings.Contains(preview.Text, `\u0066iction`) || !strings.Contains(preview.Text, "[oculto]") {
			t.Fatalf("unsafe preview=%+v", preview)
		}
	}
}

func TestDiagnosticSecretCapturePendingTracksExtractionNotCommit(t *testing.T) {
	for _, mode := range []string{"status", "partial", "duplicate secret pending", "extracted then partial public", "assertion", "stage conflict"} {
		t.Run(mode, func(t *testing.T) {
			captures := "token: jsonpath \"$.token\""
			secrets := []string{"token"}
			status := 200
			switch mode {
			case "status":
				status = 503
			case "partial":
				captures += "\nother: jsonpath \"$.missing\""
				secrets = append(secrets, "other")
			case "duplicate secret pending":
				captures += "\ntoken: jsonpath \"$.missing\""
			case "extracted then partial public":
				captures += "\npublic: jsonpath \"$.missing\""
			case "stage conflict":
				captures += "\ntoken.child: jsonpath \"$.token\""
			}
			scenario := pollingScenario(t, "POST http://example.test\n{}\nHTTP 200\n[Captures]\n"+captures+"\n[Asserts]\njsonpath \"$.ok\" == true\nGET http://example.test/never")
			c, _ := template.New(nil, secrets)
			session := c.NewSession()
			reports := 0
			r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
				if !req.DiagnosticBodies {
					t.Fatal("missing opt-in")
				}
				return diagnosticResponse(req, status, `{"token":"fiction-private-token","ok":false}`), nil
			}), observedHeaderSession{session}, func(entry EntryResult) error {
				reports++
				if entry.Entry.Body != nil || entry.Entry.Headers != nil {
					t.Fatal("operational data in callback")
				}
				if entry.Bodies == nil {
					t.Fatal("missing previews")
				}
				preview := entry.Bodies.Response
				pending := mode == "status" || mode == "partial" || mode == "duplicate secret pending"
				if pending && (preview.State != BodyProtected || preview.Text != "") || !pending && (preview.State != BodyJSON || strings.Contains(preview.Text, "fiction-private-token") || !strings.Contains(preview.Text, "[oculto]")) {
					t.Fatalf("mode=%s preview=%+v", mode, preview)
				}
				return nil
			})
			got := r.RunWithOptions(context.Background(), scenario, Options{DiagnosticBodies: true})
			if reports != 1 || len(got.Entries) != 1 || got.Err == nil && !got.Diverged {
				t.Fatalf("result=%+v reports=%d", got, reports)
			}
			if _, err := session.Render("{{token}}"); err == nil {
				t.Fatal("rejected capture published")
			}
		})
	}
}

func TestDiagnosticPollingKeepsOnlyLastSentAttempt(t *testing.T) {
	for _, mode := range []string{"exhausted", "later preparation", "wait cancellation", "success", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			scenario := pollingScenario(t, "POST http://example.test\nfile,payload.json;\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200")
			calls, reads, reports := 0, 0, 0
			r := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				calls++
				status := 503
				if mode == "success" {
					status = 200
				}
				return diagnosticResponse(req, status, fmt.Sprintf(`{"attempt":%d}`, calls)), nil
			}), func(entry EntryResult) error { reports++; return nil })
			r.readFile = func(context.Context, string) ([]byte, error) {
				reads++
				if mode == "later preparation" && reads == 2 {
					return nil, errors.New("raw-private-error")
				}
				return []byte(fmt.Sprintf(`{"payload":%d}`, reads)), nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r.wait = func(context.Context, time.Duration) error {
				if mode == "wait cancellation" {
					cancel()
					return ctx.Err()
				}
				return nil
			}
			got := r.RunWithOptions(ctx, scenario, Options{DiagnosticBodies: mode != "disabled"})
			if reports != 1 || len(got.Entries) != 1 {
				t.Fatalf("reports=%d result=%+v", reports, got)
			}
			preview := got.Entries[0].Bodies
			if mode == "success" || mode == "disabled" {
				if preview != nil {
					t.Fatalf("unexpected preview %+v", preview)
				}
				return
			}
			wantAttempt := 1
			if mode == "exhausted" {
				wantAttempt = 2
			}
			if calls != wantAttempt || preview == nil || preview.Request.Text != fmt.Sprintf(`{"payload":%d}`, wantAttempt) || preview.Response.Text != fmt.Sprintf(`{"attempt":%d}`, wantAttempt) {
				t.Fatalf("calls=%d result=%+v preview=%+v", calls, got, preview)
			}
		})
	}
}
