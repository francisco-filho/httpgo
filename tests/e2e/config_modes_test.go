package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestConfigExternalOriginAndMixedModes(t *testing.T) {
	var hits atomic.Int32
	const payload = `{"price":4500,"code":"001","items":[true,null,{"id":9007199254740993}]}`
	const upload = "\x00\xff{{undefined}}\r\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("X-Token") != "prefix-override-fake-token" {
			t.Errorf("header=%q", r.Header.Get("X-Token"))
		}
		switch r.URL.Path {
		case "/capture":
			_, _ = io.WriteString(w, `{"value":"captured-fake-token"}`)
		case "/inline/captured-fake-token", "/external":
			body, err := io.ReadAll(r.Body)
			if err != nil || string(body) != payload {
				t.Errorf("path=%s body=%q err=%v", r.URL.Path, body, err)
			}
			w.WriteHeader(201)
		case "/multipart":
			reader, err := r.MultipartReader()
			if err != nil {
				t.Error(err)
				return
			}
			field, err := reader.NextPart()
			if err != nil {
				t.Error(err)
				return
			}
			text, err := io.ReadAll(field)
			if err != nil || string(text) != "price-4500" {
				t.Errorf("text=%q err=%v", text, err)
			}
			part, err := reader.NextPart()
			if err != nil {
				t.Error(err)
				return
			}
			data, err := io.ReadAll(part)
			if err != nil || string(data) != upload || part.FileName() != "upload.bin" {
				t.Errorf("data=%q err=%v", data, err)
			}
			if _, err := reader.NextPart(); err != io.EOF {
				t.Error(err)
			}
			w.WriteHeader(201)
		default:
			t.Errorf("unexpected path=%s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	selected := configFile(t, t.TempDir(), "external.yaml", fmt.Sprintf(`version: 1
variables:
  api: {baseUrl: %s}
  product: {price: 4500, code: "001", items: [true, null, {id: 9007199254740993}]}
  auth: {token: base-fake-token}
secrets: [auth.token, future]
`, server.URL))
	dir := t.TempDir()
	configFile(t, dir, "payload.json", `{"price":{{product.price}},"code":{{product.code}},"items":{{product.items}}}`)
	configFile(t, dir, "upload.bin", upload)
	header := "\nX-Token: prefix-{{auth.token}}\n"
	file := configFile(t, dir, "source.hurl", "GET {{api.baseUrl}}/capture"+header+"HTTP 200\n[Captures]\nfuture: jsonpath \"$.value\"\nPOST {{api.baseUrl}}/inline/{{future}}"+header+"{\"price\":{{product.price}},\"code\":{{product.code}},\"items\":{{product.items}}}\nHTTP 201\nPOST {{api.baseUrl}}/external"+header+"file,payload.json;\nHTTP 201\nPOST {{api.baseUrl}}/multipart"+header+"[MultipartFormData]\ndescription: price-{{product.price}}\ndocument: file,upload.bin;\nHTTP 201")
	// Neither cwd nor selected config directory contains the payloads.
	code, out, stderr := runConfig(t, t.TempDir(), "test", file, "--config", selected, "--var", "auth.token=override-fake-token", "--render-files")
	if code != 0 || stderr != "" || hits.Load() != 4 || !strings.Contains(out, "validadas=4") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
	for _, secret := range []string{"base-fake-token", "override-fake-token", "captured-fake-token", upload, payload} {
		if strings.Contains(out+stderr, secret) {
			t.Fatalf("leak %q: %q %q", secret, out, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(selected), "payload.json")); !os.IsNotExist(err) {
		t.Fatalf("fixture unexpectedly placed in config directory: %v", err)
	}
}

func TestConfigSecretProtectionByBinary(t *testing.T) {
	for _, mode := range []string{"success", "status", "header", "json", "render", "missing future"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path != "/override-fake-token" || r.Header.Get("X-Token") != "prefix-override-fake-token" {
					t.Errorf("path=%s header=%q", r.URL.Path, r.Header.Get("X-Token"))
				}
				w.Header().Set("X-Value", "prefix-override-fake-token")
				if mode == "status" {
					w.WriteHeader(503)
				}
				_, _ = io.WriteString(w, `{"value":"prefix-override-fake-token"}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			selected := configFile(t, dir, "project.yaml", "version: 1\nvariables: {token: base-fake-token}\nsecrets: [token, future]")
			text := "GET " + server.URL + "/{{token}}\nX-Token: prefix-{{token}}\nHTTP 200\n"
			if mode == "header" {
				text += "[Asserts]\nheader \"X-Value\" == \"different-base-fake-token\"\n"
			}
			if mode == "json" {
				text += "[Asserts]\njsonpath \"$.value\" == \"different-base-fake-token\"\n"
			}
			if mode == "render" {
				text = "GET " + server.URL + "/{{token}}\nX-Bad: {{missing}}\nHTTP 200\n"
			}
			if mode == "missing future" {
				text = "GET " + server.URL + "/{{future}}\nHTTP 200\n"
			}
			text += "GET " + server.URL + "/never\nHTTP 200"
			// Success uses only one request; failures must never reach the second.
			if mode == "success" {
				text = strings.Split(text, "GET "+server.URL+"/never")[0]
			}
			file := configFile(t, dir, "base-fake-token.hurl", text)
			code, out, stderr := runConfig(t, dir, "test", file, "--config", selected, "--var", "token=override-fake-token")
			want, wantHits := 0, int32(1)
			switch mode {
			case "status", "header", "json":
				want = 1
			case "render", "missing future":
				want, wantHits = 2, 0
			}
			if code != want || hits.Load() != wantHits || mode == "success" && stderr != "" {
				t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
			}
			for _, secret := range []string{"base-fake-token", "override-fake-token"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("leak %q: %q %q", secret, out, stderr)
				}
			}
		})
	}
}
