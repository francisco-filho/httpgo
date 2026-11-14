package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestTypedJSONArgumentsReachExecutor(t *testing.T) {
	file := writeScenario(t, "POST {{base}}/{{n}}/{{b}}/{{nil}}\nX-Probe: {{empty}}|{{code}}|{{nil}}\n{\"name\":\"pre-{{name}}\",\"price\":{{n}},\"active\":{{b}},\"code\":{{code}},\"nil\":{{nil}},\"nested\":{{obj}}}\nHTTP 201")
	const name = "quote\"\\\n雪 {{literal}}"
	encoded, _ := json.Marshal(name)
	fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
		if req.URL != "http://example.test/9007199254740993123456789/true/null" || req.Headers[0].Value != "|001|null" {
			t.Fatalf("request=%+v", req)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(req.Body, &got); err != nil {
			t.Fatal(err)
		}
		var gotName string
		if err := json.Unmarshal(got["name"], &gotName); err != nil {
			t.Fatal(err)
		}
		if gotName != "pre-"+name || string(got["price"]) != "9007199254740993123456789" || string(got["active"]) != "true" || string(got["code"]) != `"001"` || string(got["nil"]) != "null" || string(got["nested"]) != `{"a":[1,"{{missing}}"]}` {
			t.Fatalf("body=%s", req.Body)
		}
		return runner.Response{Status: 201}, nil
	})
	var out, stderr bytes.Buffer
	command := NewRootCommand(fake, BuildInfo{})
	command.SetOut(&out)
	command.SetErr(&stderr)
	command.SetArgs([]string{"test", file, "--var", "base=http://example.test", "--var", "n=9007199254740993123456789", "--var", "b=true", "--var", "nil=null", "--var", `code="001"`, "--var", "name=" + string(encoded), "--var", "empty=", "--var", `obj={"a":[1,"{{missing}}"]}`})
	code := command.ExecuteContext(context.Background())
	if code != 0 || stderr.Len() != 0 || !strings.Contains(out.String(), "validadas=1") || strings.Contains(out.String(), name) {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), stderr.String())
	}
}

func TestJSONTypesAndDecodedControlsStopSequence(t *testing.T) {
	for _, tc := range []struct{ second, value string }{
		{"GET http://example.test/{{x}}", `[]`},
		{"GET http://example.test/\nX-Probe: {{x}}", `{}`},
		{"POST http://example.test/\n\"{{x}}\"", `[]`},
		{"POST http://example.test/\n{{x.name}}", `{"name":"not-a-variable"}`},
		{"GET http://example.test/{{x}}", `"not encoded"`},
		{"GET http://example.test/\nX-Probe: {{x}}", `"fake\r\nInjected: yes"`},
		{"GET http://example.test/\nX-Probe: {{x}}", `"fake\u0000"`},
		{"GET http://example.test/\nHost: {{x}}", `"user:fake@example.test"`},
	} {
		fake := &fakeExecutor{statuses: []int{200}}
		code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "GET http://example.test/first\nHTTP 200\n"+tc.second+"\nGET http://example.test/last"), "--var", "x="+tc.value, "--secret", "x")
		if code != 2 || len(fake.calls) != 1 || !strings.Contains(out, "iniciadas=1 concluídas=1 validadas=1") || !strings.Contains(out, "não_iniciadas=2") || !strings.Contains(stderr, "erro de renderização") || strings.Contains(out+stderr, "fake") {
			t.Fatalf("case=%+v code=%d out=%q err=%q calls=%d", tc, code, out, stderr, len(fake.calls))
		}
	}
}

func TestDecodedSecretProtectedBeforeAndAfterContext(t *testing.T) {
	const secret = "fictitious\"\\\n雪"
	encoded, _ := json.Marshal(secret)
	quoted := strconv.Quote(secret)
	variants := []string{string(encoded), secret, string(encoded[1 : len(encoded)-1]), quoted[1 : len(quoted)-1], url.PathEscape(secret), url.QueryEscape(secret)}
	for _, path := range variants {
		for _, flags := range [][]string{
			{"--var", "x=" + string(encoded), "--secret", "x"},
			{"--secret", "x", "--var", "x=" + string(encoded), "--var", "x=duplicate"},
			{"--unknown", "--var", "x=" + string(encoded), "--secret", "x"},
		} {
			args := append([]string{"test", path}, flags...)
			code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, args...)
			if code != 2 {
				t.Fatalf("code=%d", code)
			}
			for _, variant := range variants {
				if strings.Contains(out+stderr, variant) {
					t.Fatalf("leaked variant %q: %q %q", variant, out, stderr)
				}
			}
		}
	}
	for _, mode := range []string{"success", "mismatch", "transport", "missing", "syntax", "write"} {
		t.Run(mode, func(t *testing.T) {
			text := "POST http://example.test/\n{\"s\":\"pre-{{x}}\",\"v\":{{x}}}\nHTTP 200\nGET http://example.test/last"
			fake := &fakeExecutor{statuses: []int{200, 200}}
			wantCode, wantCalls := 0, 2
			switch mode {
			case "mismatch":
				fake.statuses[0] = 503
				wantCode, wantCalls = 1, 1
			case "transport":
				fake.err = errors.New(secret)
				wantCode, wantCalls = 2, 1
			case "missing":
				text = "GET http://example.test/first\nHTTP 200\nPOST http://example.test/\n{\"a\":{{x}},\"b\":{{missing}}}"
				wantCode, wantCalls = 2, 1
			case "syntax":
				text += "\nPOST http://example.test/\n{\"{{x}}\":1}"
				wantCode, wantCalls = 2, 0
			case "write":
				wantCode, wantCalls = 2, 1
			}
			var out, stderr bytes.Buffer
			command := NewRootCommand(fake, BuildInfo{})
			command.SetOut(&out)
			command.SetErr(&stderr)
			if mode == "write" {
				command.SetOut(&failAfterWriter{successes: 0, cause: errors.New(secret)})
			}
			command.SetArgs([]string{"test", writeScenario(t, text), "--secret", "x", "--var", "x=" + string(encoded)})
			code := command.ExecuteContext(context.Background())
			if code != wantCode || len(fake.calls) != wantCalls {
				t.Fatalf("code=%d out=%q err=%q calls=%d", code, out.String(), stderr.String(), len(fake.calls))
			}
			for _, variant := range variants {
				if strings.Contains(out.String()+stderr.String(), variant) {
					t.Fatalf("leaked %q", variant)
				}
			}
		})
	}
}
