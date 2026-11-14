//go:build ignore

// Run explicitly: go test -race ./tools/dummy/main.go ./tools/dummy/main_test.go
package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDummyEndpoints(t *testing.T) {
	server := httptest.NewServer(newHandler())
	defer server.Close()
	for _, tc := range []struct {
		method, path, body string
		status             int
		want, allow        string
	}{
		{"GET", "/health", "", 200, "{\"status\":\"ok\"}\n", ""},
		{"GET", "/error", "", 500, "{\"error\":\"dummy server error\"}\n", ""},
		{"GET", "/inexistente", "", 404, "{\"error\":\"not found\"}\n", ""},
		{"POST", "/health", "", 405, "{\"error\":\"method not allowed\"}\n", "GET"},
		{"GET", "/products", "", 405, "{\"error\":\"method not allowed\"}\n", "POST"},
		{"POST", "/products", `{"name":"Pizza","price":4500}`, 201, "{\"id\":1}\n", ""},
		{"POST", "/products", "", 201, "{\"id\":1}\n", ""},
		{"PUT", "/products/1", "{\n \"price\": 5000\n}", 200, "{\"id\":1}\n", ""},
		{"PATCH", "/products/1", `{"price":4900}`, 200, "{\"id\":1}\n", ""},
		{"PATCH", "/products/1", "", 200, "{\"id\":1}\n", ""},
		{"DELETE", "/products/1", "", 204, "", ""},
		{"DELETE", "/products/1", `null`, 204, "", ""},
		{"GET", "/products/1", "", 200, "{\"id\":1,\"name\":\"Pizza\",\"price\":4500}\n", ""},
		{"GET", "/products/2", "", 404, "{\"error\":\"not found\"}\n", ""},
		{"POST", "/products/1", "", 405, "{\"error\":\"method not allowed\"}\n", "GET, PUT, PATCH, DELETE"},
		{"OPTIONS", "/echo", "", 405, "{\"error\":\"method not allowed\"}\n", "GET, POST, PUT, PATCH, DELETE"},
	} {
		t.Run(tc.method+tc.path+strconv.Itoa(len(tc.body)), func(t *testing.T) {
			response, body := request(t, server, tc.method, tc.path, tc.body)
			if response.StatusCode != tc.status || body != tc.want || response.Header.Get("Allow") != tc.allow {
				t.Fatalf("status=%d,body=%q,allow=%q", response.StatusCode, body, response.Header.Get("Allow"))
			}
		})
	}
}

func TestEchoPreservesBytesAndSelectedMetadata(t *testing.T) {
	server := httptest.NewServer(newHandler())
	defer server.Close()
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		for index, body := range []string{"", `{"name":"Pizza"}`, "{\r\n \"values\": [true, null]\r\n}", `"HTTP 200 #"`, `42`, `false`, `null`} {
			if method == "GET" && body != "" {
				continue
			}
			t.Run(method+strconv.Itoa(index), func(t *testing.T) {
				response, got := request(t, server, method, "/echo?token=query-secret", body)
				if response.StatusCode != 200 || got != body || response.Header.Get("X-Dummy-Method") != method || response.Header.Get("X-Dummy-Body-Bytes") != strconv.Itoa(len(body)) || response.Header.Get("Content-Type") != "application/json" {
					t.Fatalf("status=%d,headers=%v,body=%q", response.StatusCode, response.Header, got)
				}
				for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie"} {
					if response.Header.Get(header) != "" {
						t.Errorf("echo leaked %s", header)
					}
				}
				if strings.Contains(got, "query-secret") || strings.Contains(got, "header-secret") {
					t.Fatalf("unexpected credentials in echo: %q", got)
				}
			})
		}
	}
}

func TestPayloadErrorsAreBoundedAndSafe(t *testing.T) {
	server := httptest.NewServer(newHandler())
	defer server.Close()
	for _, path := range []string{"/products", "/products/1", "/echo"} {
		method := "POST"
		if path == "/products/1" {
			method = "PATCH"
		}
		for _, tc := range []struct {
			body   string
			status int
		}{
			{`{"password":"body-secret",}`, 400},
			{strings.Repeat("x", (1<<20)+1), 413},
		} {
			response, body := request(t, server, method, path, tc.body)
			if response.StatusCode != tc.status || strings.Contains(body, "body-secret") || strings.Contains(body, "header-secret") {
				t.Fatalf("status=%d,body=%q", response.StatusCode, body)
			}
		}
	}
}

func request(t *testing.T, server *httptest.Server, method, path, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for _, header := range []string{"Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie"} {
		req.Header.Set(header, "header-secret")
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response, string(data)
}

func TestDocumentedScenariosWithBinary(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	binary := filepath.Join(t.TempDir(), "httpgo.exe")
	root := filepath.Join("..", "..")
	build := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/httpgo")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	server := httptest.NewServer(newHandler())
	defer server.Close()
	for _, name := range []string{"scenario_get_status.hurl", "scenario_unvalidated_status.hurl", "scenario_json_inline.hurl", "scenario_dummy_methods.hurl"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join(root, "testdata", "scenarios", name))
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(file, bytes.ReplaceAll(input, []byte("http://localhost:8080"), []byte(server.URL)), 0600); err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			cmd := exec.CommandContext(ctx, binary, "test", file)
			cmd.Stdout, cmd.Stderr = &out, &stderr
			if err := cmd.Run(); err != nil || stderr.Len() != 0 || !strings.Contains(out.String(), "não_iniciadas=0") {
				t.Fatalf("run=%v,out=%q,err=%q", err, out.String(), stderr.String())
			}
		})
	}
}
