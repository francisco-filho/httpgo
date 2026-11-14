package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestJSONDemoFile(t *testing.T) {
	input, err := os.ReadFile("../../testdata/scenarios/scenario_json_inline.hurl")
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || r.URL.Path != "/products" || r.Header.Get("Content-Type") != "application/json" || string(data) != `{"name":"Pizza","price":4500}` {
			t.Errorf("method=%s,path=%s,body=%q", r.Method, r.URL.Path, data)
		}
		hits.Add(1)
		w.WriteHeader(201)
	}))
	defer server.Close()
	code, out, stderr := run(t, "test", scenario(t, strings.ReplaceAll(string(input), "http://localhost:8080", server.URL)))
	if code != 0 || stderr != "" || hits.Load() != 1 || !strings.Contains(out, "esperado 201, recebido 201") {
		t.Fatalf("code=%d,out=%q,err=%q,hits=%d", code, out, stderr, hits.Load())
	}
}

func TestOperationsByBinary(t *testing.T) {
	for _, mode := range []string{"success", "mismatch"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				mu.Lock()
				methods = append(methods, r.Method)
				mu.Unlock()
				switch r.Method {
				case "POST":
					if string(body) != `{"name":"Pizza","price":4500}` || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("POST body=%q,headers=%v", body, r.Header)
					}
					if r.Header.Get("Authorization") != "auth-secret" || r.Header.Get("Proxy-Authorization") != "proxy-secret" || r.Header.Get("Cookie") != "cookie-secret" || r.Header.Get("Set-Cookie") != "set-cookie-secret" {
						t.Error("missing explicit headers")
					}
					if mode == "mismatch" {
						w.WriteHeader(503)
					} else {
						w.WriteHeader(201)
					}
				case "PUT":
					if string(body) != "[\n  true, null, {\"value\": \"body-secret\"}\n]" {
						t.Errorf("body=%q", body)
					}
					w.WriteHeader(200)
				case "PATCH":
					if string(body) != `"HTTP 200 GET # [Captures] body-secret"` {
						t.Errorf("body=%q", body)
					}
					w.WriteHeader(500)
				case "DELETE":
					if len(body) != 0 {
						t.Errorf("DELETE body=%q", body)
					}
					w.WriteHeader(204)
				case "GET":
					if len(body) != 0 {
						t.Errorf("GET body=%q", body)
					}
					w.WriteHeader(404)
				default:
					t.Errorf("unexpected method=%s", r.Method)
				}
			}))
			defer server.Close()
			text := fmt.Sprintf("POST %s/products\nContent-Type: application/json\nAuthorization: auth-secret\nProxy-Authorization: proxy-secret\nCookie: cookie-secret\nSet-Cookie: set-cookie-secret\n{\"name\":\"Pizza\",\"price\":4500}\nHTTP 201\nPUT %s/products/1\n[\n  true, null, {\"value\": \"body-secret\"}\n]\nHTTP 200\nPATCH %s/products/1\n\"HTTP 200 GET # [Captures] body-secret\"\nDELETE %s/products/1\nHTTP 204\nGET %s/missing\nHTTP 404", server.URL, server.URL, server.URL, server.URL, server.URL)
			code, out, stderr := run(t, "test", scenario(t, text))
			wantCode := 0
			wantMethods := []string{"POST", "PUT", "PATCH", "DELETE", "GET"}
			if mode == "mismatch" {
				wantCode = 1
				wantMethods = wantMethods[:1]
			}
			mu.Lock()
			defer mu.Unlock()
			if code != wantCode || !slices.Equal(methods, wantMethods) || !strings.Contains(out, "POST "+server.URL+"/products:") || !strings.Contains(out, "resumo") {
				t.Fatalf("code=%d,out=%q,err=%q,methods=%v", code, out, stderr, methods)
			}
			if mode == "success" && (stderr != "" || !strings.Contains(out, "sem validação de status (recebido 500)") || !strings.Contains(out, "validadas=4")) {
				t.Fatalf("out=%q,err=%q", out, stderr)
			}
			if mode == "mismatch" && (!strings.Contains(stderr, "esperado 201, recebido 503") || !strings.Contains(out, "não_iniciadas=4")) {
				t.Fatalf("out=%q,err=%q", out, stderr)
			}
			assertNoSecrets(t, out+stderr)
		})
	}
}

func TestJSONScalarBytesByBinary(t *testing.T) {
	for index, body := range []string{`42`, `-1.20e+3`, `true`, `false`, `null`, `"body-secret"`, `{"a":{"b":{}}}`} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			got := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				got <- string(data)
				w.WriteHeader(201)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "POST "+server.URL+"\n  "+body+" \t\nHTTP 201"))
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d,out=%q,err=%q", code, out, stderr)
			}
			if received := <-got; received != body {
				t.Fatalf("received=%q,want=%q", received, body)
			}
			assertNoSecrets(t, out+stderr)
		})
	}
}

func TestInvalidLastOperationPreventsEverySend(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	for _, suffix := range []string{
		`{"password":"body-secret",}`, `{"password":"body-secret"`, "Authorization: auth-secret\rInjected: cookie-secret", "Bad Header: auth-secret", "Cookie: cookie-secret\n continuation-secret", "{}\nAuthorization: auth-secret", "{}\n{}", "[Asserts]", "Content-Length: 100", "X-Test: {{auth-secret}}", "{\"a\":\"{{body-secret}}\"}",
	} {
		file := scenario(t, "GET "+server.URL+"/first\nHTTP 200\nPOST "+server.URL+"/last\n"+suffix)
		code, out, stderr := run(t, "test", file)
		if code != 2 || out != "" || !strings.HasPrefix(stderr, fmt.Sprintf("%q:", file)) || hits.Load() != 0 {
			t.Fatalf("code=%d,out=%q,err=%q,hits=%d", code, out, stderr, hits.Load())
		}
		assertNoSecrets(t, out+stderr)
	}
	code, out, stderr := run(t, "test", scenario(t, "POST "+server.URL+"/first\nGET "+server.URL+"/last\n{\"password\":\"body-secret\"}"))
	if code != 2 || out != "" || !strings.Contains(stderr, "body em GET") || hits.Load() != 0 {
		t.Fatalf("code=%d,out=%q,err=%q,hits=%d", code, out, stderr, hits.Load())
	}
	assertNoSecrets(t, out+stderr)
}

func TestRedirectBytesByBinary(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var hits atomic.Int32
			const body = "{\n \"password\": \"body-secret\"\n}"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				wantMethod, wantBody := "POST", body
				if r.URL.Path == "/final" && status <= 303 {
					wantMethod, wantBody = "GET", ""
				}
				if r.Method != wantMethod || string(data) != wantBody || r.Header.Get("Authorization") != "auth-secret" {
					t.Errorf("method=%s,body=%q,header=%q", r.Method, data, r.Header.Get("Authorization"))
				}
				if r.URL.Path == "/start" {
					w.Header().Set("Location", "/final")
					w.WriteHeader(status)
					return
				}
				w.WriteHeader(201)
			}))
			defer server.Close()
			code, out, stderr := run(t, "test", scenario(t, "POST {{base}}/start\nAuthorization: {{auth}}\n"+body+"\nHTTP 201"), "--var", "base="+server.URL, "--var", "auth=auth-secret", "--secret", "auth")
			if code != 0 || stderr != "" || hits.Load() != 2 || !strings.Contains(out, "esperado 201, recebido 201") {
				t.Fatalf("code=%d,out=%q,err=%q,hits=%d", code, out, stderr, hits.Load())
			}
			assertNoSecrets(t, out+stderr)
		})
	}
}
