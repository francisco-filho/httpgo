package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/runner"
)

func TestRegexCaptureCLI(t *testing.T) {
	for _, mode := range []string{"success", "missing", "cookie", "late invalid", "late unsupported"} {
		t.Run(mode, func(t *testing.T) {
			decl := `otp: regex "verificação é: ([0-9]{6})"`
			next := "POST http://example.test/verify\n{\"code\":{{otp}}}\nHTTP 200"
			if mode == "cookie" {
				decl = `otp: header "Set-Cookie" regex "refresh=([^;]+)"`
				next = "GET http://example.test/verify\nCookie: refresh={{otp}}\nHTTP 200"
			}
			source := "GET http://example.test/message\nHTTP 200\n[Captures]\n" + decl + "\n" + next
			if mode == "late invalid" {
				source += "\n[Captures]\nx: regex \"sensitive[\""
			}
			if mode == "late unsupported" {
				source += "\n[Captures]\nx: regex \"(?=sensitive)\""
			}
			calls := 0
			code, out, stderr := execute(t, context.Background(), executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
				calls++
				if calls == 1 {
					text := "Seu código de verificação é: 123456"
					if mode == "missing" {
						text = "no code"
					}
					return runner.Response{Status: 200, Body: []byte(text), Headers: []query.HeaderValue{{Name: "Set-Cookie", Value: "refresh=123456; Path=/; HttpOnly"}}}, nil
				}
				if mode == "cookie" {
					if req.Headers[0].Value != "refresh=123456" {
						t.Fatal("cookie attributes sent")
					}
				} else {
					var body map[string]string
					if json.Unmarshal(req.Body, &body) != nil || body["code"] != "123456" {
						t.Fatalf("body=%s", req.Body)
					}
				}
				return runner.Response{Status: 200}, nil
			}), "test", writeScenario(t, source), "--secret", "otp")
			wantCode, wantCalls := 0, 2
			if mode == "missing" {
				wantCode, wantCalls = 2, 1
			}
			if strings.HasPrefix(mode, "late") {
				wantCode, wantCalls = 2, 0
			}
			if code != wantCode || calls != wantCalls || strings.Contains(out+stderr, "123456") || strings.Contains(out+stderr, "sensitive") {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls, out, stderr)
			}
		})
	}
}
