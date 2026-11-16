package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRegexCaptureOTPAndCookieByBinary(t *testing.T) {
	for _, mode := range []string{"otp", "missing", "cookie", "no implicit cookie"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := hits.Add(1)
				if n == 1 {
					w.Header().Add("Set-Cookie", "refresh=fictional-token; Path=/; HttpOnly; Secure")
					w.Header().Add("Set-Cookie", "refresh=ignored; Path=/")
					if mode != "missing" {
						_, _ = io.WriteString(w, "Seu código de verificação é: 123456")
					}
					return
				}
				if mode == "cookie" {
					if r.Header.Get("Cookie") != "refresh=fictional-token" {
						t.Error("wrong cookie or attributes sent")
					}
				} else if mode == "otp" {
					body, _ := io.ReadAll(r.Body)
					var value map[string]string
					if json.Unmarshal(body, &value) != nil || value["code"] != "123456" {
						t.Errorf("body=%s", body)
					}
				} else if r.Header.Get("Cookie") != "" {
					t.Error("implicit cookie store")
				}
			}))
			defer server.Close()
			source := "GET {{base}}/message/123\nHTTP 200\n[Captures]\notp_code: regex \"verificação é: ([0-9]{6})\"\nPOST {{base}}/verify\nContent-Type: application/json\n{\"code\":\"{{otp_code}}\"}\nHTTP 200"
			if strings.Contains(mode, "cookie") {
				source = "GET {{base}}/session\nHTTP 200\n[Captures]\nrefresh_token: header \"Set-Cookie\" regex \"refresh=([^;]+)\"\nGET {{base}}/next\n"
				if mode == "cookie" {
					source += "Cookie: refresh={{refresh_token}}\n"
				}
				source += "HTTP 200"
			}
			code, out, stderr := run(t, "test", scenario(t, source), "--var", "base="+server.URL, "--secret", "otp_code")
			wantCode, wantHits := 0, int32(2)
			if mode == "missing" {
				wantCode, wantHits = 2, 1
			}
			if code != wantCode || hits.Load() != wantHits || strings.Contains(out+stderr, "123456") || strings.Contains(out+stderr, "fictional-token") {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
		})
	}
}
