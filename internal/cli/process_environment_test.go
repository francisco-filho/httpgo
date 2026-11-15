package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestProcessEnvironmentRequest(t *testing.T) {
	for _, environment := range []bool{false, true} {
		t.Run(map[bool]string{false: "base", true: "selected environment"}[environment], func(t *testing.T) {
			t.Setenv("HTTPGO_OWNER_TOKEN", "fiction-process-token")
			t.Setenv("HTTPGO_TIMEOUT", "0.001")
			t.Setenv("HTTPGO_BASE_URL", "http://not-used.test")
			source := "variables: {auth: {token: '${HTTPGO_OWNER_TOKEN}'}, api: {url: http://example.test}, price: 123, items: [true, null]}\ndefaults: {timeout: 2.5}"
			base := writeConfig(t, "version: 1\n"+source)
			args := []string{"--config", base}
			if environment {
				base = writeConfig(t, "version: 1\nvariables: {auth: {token: '${DISCARDED_PROCESS_TOKEN}'}, sibling: true}")
				writeEnvironment(t, base, "local", source)
				args = []string{"--config", base, "--env", "local"}
			}
			calls := 0
			fake := executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
				calls++
				if request.URL != "http://example.test/profile" || len(request.Headers) != 1 || request.Headers[0].Value != "Bearer fiction-process-token" || request.Timeout != 2500*time.Millisecond {
					t.Error("incorrect operational URL/header/timeout")
				}
				return runner.Response{Status: 200}, nil
			})
			file := writeScenario(t, "GET {{api.url}}/profile\nAuthorization: Bearer {{auth.token}}\nHTTP 200")
			code, out, stderr := execute(t, context.Background(), fake, append([]string{"test", file}, args...)...)
			if code != 0 || stderr != "" || calls != 1 || strings.Contains(out, "fiction-process-token") {
				t.Fatalf("code=%d calls=%d protected=%v", code, calls, !strings.Contains(out+stderr, "fiction-process-token"))
			}
		})
	}
}

func TestProcessEnvironmentJSONString(t *testing.T) {
	for _, value := range []string{"123", "true", "null", "", "quotes\"雪\n", "${ANOTHER}", "{{another}}"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("HTTPGO_OWNER_TOKEN", value)
			calls := 0
			fake := executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
				calls++
				var got struct{ Token string }
				if err := json.Unmarshal(request.Body, &got); err != nil || got.Token != value {
					t.Error("environment content/type changed")
				}
				return runner.Response{Status: 200}, nil
			})
			code, _, _ := execute(t, context.Background(), fake, "test", writeScenario(t, "POST http://example.test/\n{\"token\":{{token}}}\nHTTP 200"), "--config", writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_OWNER_TOKEN}'}"))
			if code != 0 || calls != 1 {
				t.Fatalf("code=%d calls=%d", code, calls)
			}
		})
	}
}

func TestProcessEnvironmentInvalidBeforeNetwork(t *testing.T) {
	for _, source := range []string{
		"token: '${HTTPGO_US13_ABSENT}'", "token: 'Bearer ${TOKEN}'", "token: '${1BAD}'", "token: '${TOKEN'", "token: ['${TOKEN}']", "token: [{nested: ['${TOKEN}']}]",
	} {
		fake := &fakeExecutor{}
		base := writeConfig(t, "version: 1\nvariables: {"+source+"}")
		// Missing reference is mandatory even if the scenario does not use it.
		file := writeScenario(t, "GET http://example.test/\nHTTP 200")
		code, out, stderr := execute(t, context.Background(), fake, "test", file, "--config", base)
		if code != 2 || out != "" || len(fake.calls) != 0 || !strings.Contains(stderr, base) {
			t.Fatalf("code=%d calls=%d err=%q", code, len(fake.calls), stderr)
		}
		if strings.Contains(source, "ABSENT") {
			continue
		}
		code, _, _ = execute(t, context.Background(), fake, "test", file, "--config", base, "--var", "token=fiction")
		if code != 2 || len(fake.calls) != 0 {
			t.Fatal("override hid invalid source")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeExecutor{}
	code, _, _ := execute(t, ctx, fake, "test", writeScenario(t, "GET http://example.test/"), "--config", writeConfig(t, "version: 1\nvariables: {token: '${TOKEN}'}"))
	if code != 2 || len(fake.calls) != 0 {
		t.Fatal("canceled preparation sent HTTP")
	}
}
