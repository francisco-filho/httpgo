package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/template"
)

func TestExternalCapturedRootTypes(t *testing.T) {
	for _, value := range []string{`"quotes\"\\\n雪{{literal}}"`, `9007199254740993123456789`, `true`, `null`, `[9007199254740993,"{{literal}}"]`, `{"nested":[null,false]}`} {
		t.Run(value, func(t *testing.T) {
			scenario, err := parser.Parse("safe.hurl", strings.NewReader("GET http://example.test/capture\nHTTP 200\n[Captures]\nproduct.value: jsonpath \"$\"\nPOST http://example.test/echo\nfile,payload;\nHTTP 201"))
			if err != nil {
				t.Fatal(err)
			}
			calls, reports := 0, 0
			r := New(executorFunc(func(_ context.Context, req Request) (Response, error) {
				if calls != reports {
					t.Fatal("send preceded report")
				}
				calls++
				if calls == 1 {
					return Response{Status: 200, Body: []byte(value)}, nil
				}
				if string(req.Body) != value {
					t.Fatalf("body=%s want=%s", req.Body, value)
				}
				return Response{Status: 201}, nil
			}), func(entry EntryResult) error {
				reports++
				if entry.Entry.Body != nil || entry.Entry.ExternalBody != nil || entry.Entry.Captures != nil {
					t.Fatalf("unsafe entry=%+v", entry)
				}
				return nil
			})
			r.readFile = func(context.Context, string) ([]byte, error) { return []byte("{{product.value}}"), nil }
			got := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: true})
			if got.Err != nil || got.Diverged || calls != 2 || reports != 2 || got.Summary.Validated != 2 {
				t.Fatalf("got=%+v calls=%d reports=%d", got, calls, reports)
			}
		})
	}
}

func TestExternalSecretsAndOperationalMetadataNeverReachReports(t *testing.T) {
	secret := `"credential-fictitious\"\\\n雪"`
	for _, captured := range []bool{false, true} {
		for _, mode := range []string{"success", "mismatch", "render", "transport", "writer"} {
			t.Run(fmt.Sprintf("captured=%t/%s", captured, mode), func(t *testing.T) {
				prefix := "GET http://example.test/first\nHTTP 200\n"
				var assignments []string
				if captured {
					prefix += "[Captures]\ncredential: jsonpath \"$\"\n"
				} else {
					assignments = []string{"credential=" + secret}
				}
				scenario, err := parser.Parse("credential-fictitious.hurl", strings.NewReader(prefix+"POST http://example.test/affected\nfile,credential-fictitious.json;\nHTTP 201\nGET http://example.test/last"))
				if err != nil {
					t.Fatal(err)
				}
				renderer, err := template.New(assignments, []string{"credential"})
				if err != nil {
					t.Fatal(err)
				}
				reports, calls := 0, 0
				writerErr := errors.New("private writer cause")
				r := NewWithRenderer(executorFunc(func(_ context.Context, req Request) (Response, error) {
					calls++
					if calls == 1 {
						return Response{Status: 200, Body: []byte(secret)}, nil
					}
					if req.URL == "http://example.test/last" {
						return Response{Status: 200}, nil
					}
					var decoded map[string]string
					if err := json.Unmarshal(req.Body, &decoded); err != nil {
						t.Fatal(err)
					}
					var raw string
					if err := json.Unmarshal([]byte(secret), &raw); err != nil {
						t.Fatal(err)
					}
					if decoded["token"] != "prefix-"+raw {
						t.Fatalf("body=%q", req.Body)
					}
					switch mode {
					case "mismatch":
						return Response{Status: 503}, nil
					case "transport":
						return Response{}, errors.New("private transport cause")
					}
					return Response{Status: 201}, nil
				}), renderer, func(entry EntryResult) error {
					reports++
					assertExternalSafeReport(t, entry, secret)
					if mode == "writer" && reports == 2 {
						return writerErr
					}
					return nil
				})
				r.readFile = func(context.Context, string) ([]byte, error) {
					body := `{"token":"prefix-{{credential}}"}`
					if mode == "render" {
						body = `{"token":"prefix-{{credential}}","missing":{{missing}}}`
					}
					return []byte(body), nil
				}
				got := r.RunWithOptions(context.Background(), scenario, Options{RenderFiles: true})
				for _, entry := range got.Entries {
					assertExternalSafeReport(t, entry, secret)
				}
				if mode == "success" && (got.Err != nil || calls != 3) || mode != "success" && mode != "render" && calls != 2 || mode == "render" && (calls != 1 || got.Entries[1].Outcome != RenderFailed) || mode == "writer" && !errors.Is(got.ReportErr, writerErr) {
					t.Fatalf("got=%+v calls=%d", got, calls)
				}
			})
		}
	}
}

func assertExternalSafeReport(t *testing.T, entry EntryResult, secret string) {
	t.Helper()
	if entry.Entry.Body != nil || entry.Entry.ExternalBody != nil || entry.Entry.Multipart != nil || entry.Entry.Headers != nil || len(entry.Entry.BodyPlan.References) != 0 {
		t.Fatalf("operational metadata retained: %+v", entry)
	}
	shown := fmt.Sprintf("%+v", entry)
	for _, variant := range template.SecretVariants(secret) {
		if variant != "" && strings.Contains(shown, variant) {
			t.Fatalf("secret in report: %q", shown)
		}
	}
}

func TestExternalRenderOriginIsSanitized(t *testing.T) {
	renderer, err := template.New([]string{"credential=credential-fictitious"}, []string{"credential"})
	if err != nil {
		t.Fatal(err)
	}
	session := renderer.NewSession()
	_, failure := renderExternal(session, []byte("{{missing}}"), parser.FileBody{Path: "credential-fictitious.json", Position: parser.Position{File: "credential-fictitious.hurl", Line: 4, Column: 1}})
	if failure == nil || strings.Contains(fmt.Sprintf("%+v %v %v", failure, failure.Reference, failure.Position), "credential-fictitious") {
		t.Fatalf("unsafe failure=%+v", failure)
	}
}
