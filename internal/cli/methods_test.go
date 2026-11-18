package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNewMethodsPresentationAndSecretSafety(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		for _, mode := range []string{"success", "unvalidated", "mismatch", "transport", "bad JSON", "bad header", "late invalid"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				text := method + " http://example.test/products\nAuthorization: auth-secret\nProxy-Authorization: proxy-secret\nCookie: cookie-secret\nSet-Cookie: set-cookie-secret\nContent-Type: application/json\n{\"password\":\"body-secret\"}\nHTTP 201\nGET http://example.test/missing\nHTTP 404"
				fake := &fakeExecutor{statuses: []int{201, 404}}
				wantCode, wantCalls := 0, 2
				switch mode {
				case "unvalidated":
					text = strings.Replace(text, "HTTP 201\n", "", 1)
					fake.statuses[0] = 500
				case "mismatch":
					fake.statuses[0] = 503
					wantCode, wantCalls = 1, 1
				case "transport":
					fake.err = errors.New("auth-secret cookie-secret body-secret proxy-secret set-cookie-secret")
					wantCode, wantCalls = 2, 1
				case "bad JSON":
					text = strings.Replace(text, `"body-secret"}`, `"body-secret",}`, 1)
					wantCode, wantCalls = 2, 0
				case "bad header":
					text = strings.Replace(text, "Authorization: auth-secret", "Authorization: auth-secret\rInjected: cookie-secret", 1)
					wantCode, wantCalls = 2, 0
				case "late invalid":
					text += "\nPATCH http://example.test/last\n{\"password\":\"body-secret\",}"
					wantCode, wantCalls = 2, 0
				}
				code, out, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, text))
				if code != wantCode || len(fake.calls) != wantCalls {
					t.Fatalf("code=%d,out=%q,err=%q,calls=%v", code, out, stderr, fake.calls)
				}
				for _, secret := range []string{"auth-secret", "proxy-secret", "cookie-secret", "set-cookie-secret", "body-secret"} {
					if strings.Contains(out+stderr, secret) {
						t.Fatalf("leaked %s in %q %q", secret, out, stderr)
					}
				}
				if wantCalls > 0 && (!strings.Contains(out, method+" http://example.test/products:") || !strings.Contains(out, "resumo")) {
					t.Fatalf("unhelpful output=%q", out)
				}
				if wantCalls == 0 && (!prevalidationSummary(out) || !strings.Contains(stderr, ":") || !strings.Contains(stderr, "sintaxe inválida")) {
					t.Fatalf("out=%q,err=%q", out, stderr)
				}
				if mode == "unvalidated" && !strings.Contains(out, "sem validação de status (recebido 500)") {
					t.Fatalf("out=%q", out)
				}
				if code == 0 && stderr != "" {
					t.Fatalf("unexpected stderr=%q", stderr)
				}
			})
		}
	}
	code, out, stderr := execute(t, context.Background(), &fakeExecutor{}, "test", "--help")
	if code != 0 || stderr != "" || !strings.Contains(out, "httpgo test products.httpgo") || !strings.Contains(out, `{"name":"Pizza","price":4500}`) {
		t.Fatalf("help=%q,err=%q", out, stderr)
	}
}
