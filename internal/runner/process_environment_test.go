package runner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/config"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestProcessSecretIsSanitizedBeforeCallback(t *testing.T) {
	const token = "fiction-process-callback"
	t.Setenv("HTTPGO_US13_TOKEN", token)
	for _, mode := range []string{"header actual", "header expected", "json actual", "json expected"} {
		t.Run(mode, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "base.yaml")
			if err := os.WriteFile(file, []byte("version: 1\nvariables: {token: '${HTTPGO_US13_TOKEN}'}"), 0600); err != nil {
				t.Fatal(err)
			}
			settings, err := config.Load(context.Background(), file)
			if err != nil {
				t.Fatal(err)
			}
			renderer, err := settings.Compose(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			expected, actual := "different", "prefix-"+token
			if strings.Contains(mode, "expected") {
				expected, actual = actual, expected
			}
			assertion := fmt.Sprintf("header \"X-Echo\" == %q", expected)
			if strings.HasPrefix(mode, "json") {
				assertion = fmt.Sprintf("jsonpath \"$.value\" == %q", expected)
			}
			scenario, err := parser.Parse(token+".hurl", strings.NewReader("GET http://example.test/{{token}}\nHTTP 200\n[Asserts]\n"+assertion+"\nGET http://example.test/never"))
			if err != nil {
				t.Fatal(err)
			}
			inspect := func(entry runner.EntryResult) {
				t.Helper()
				if strings.Contains(fmt.Sprintf("%+v %+v %+v %+v", entry, entry.Entry, entry.HeaderFailure, entry.JSONFailure), token) {
					t.Fatal("structured result leaked secret")
				}
				if entry.Entry.Headers != nil || entry.Entry.Body != nil {
					t.Fatal("result retained operational data")
				}
				if entry.HeaderFailure != nil && !strings.Contains(entry.HeaderFailure.Expected+entry.HeaderFailure.Actual, "[oculto]") {
					t.Fatal("header operands not protected")
				}
				if entry.JSONFailure != nil && !strings.Contains(entry.JSONFailure.Expected+entry.JSONFailure.Actual, "[oculto]") {
					t.Fatal("JSON operands not protected")
				}
			}
			calls, callbacks := 0, 0
			r := runner.NewWithRenderer(configExecutor(func(_ context.Context, request runner.Request) (runner.Response, error) {
				calls++
				if request.URL != "http://example.test/"+token {
					t.Error("operational value redacted")
				}
				return runner.Response{Status: 200, Headers: []query.HeaderValue{{Name: "X-Echo", Value: actual}}, Body: []byte(fmt.Sprintf(`{"value":%q}`, actual))}, nil
			}), renderer, func(entry runner.EntryResult) error { callbacks++; inspect(entry); return nil })
			result := r.Run(context.Background(), scenario)
			if !result.Diverged || result.Err != nil || result.ReportErr != nil || calls != 1 || callbacks != 1 {
				t.Fatalf("calls=%d callbacks=%d diverged=%v", calls, callbacks, result.Diverged)
			}
			for _, entry := range result.Entries {
				inspect(entry)
			}
		})
	}
}
