package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEnvironmentBodiesAndCaptures(t *testing.T) {
	var hits atomic.Int32
	const wantJSON = `{"price":9007199254740993,"active":true,"code":"001","items":[null,{"name":"Suco"}]}`
	const literal = "\x00\xff{{product.price}}\r\n"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("X-Probe") != "derived-cli-fake-token" {
			t.Errorf("header=%q", r.Header.Get("X-Probe"))
		}
		switch r.URL.Path {
		case "/capture":
			_, _ = io.WriteString(w, `{"value":"captured-env-fake"}`)
		case "/inline/captured-env-fake", "/external":
			data, err := io.ReadAll(r.Body)
			if err != nil || string(data) != wantJSON {
				t.Errorf("body=%q want=%q err=%v", data, wantJSON, err)
			}
			w.WriteHeader(201)
		case "/multipart":
			reader, err := r.MultipartReader()
			if err != nil {
				t.Error(err)
				return
			}
			for _, want := range []string{"price-9007199254740993", literal} {
				part, err := reader.NextPart()
				if err != nil {
					t.Error(err)
					return
				}
				data, err := io.ReadAll(part)
				if err != nil || string(data) != want {
					t.Errorf("part=%q want=%q err=%v", data, want, err)
				}
			}
			if _, err := reader.NextPart(); err != io.EOF {
				t.Errorf("extra part: %v", err)
			}
			w.WriteHeader(201)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	base := configFile(t, root, "base.yaml", "version: 1\nvariables: {product: {price: 1, active: true, code: \"001\"}, token: base-fake-token}\nsecrets: [token]")
	environmentFile(t, root, "local", "variables: {base: "+server.URL+", product: {price: 9007199254740993, items: [null, {name: Suco}]}, token: env-fake-token}\nsecrets: [future]")
	dir := t.TempDir()
	template := `{"price":{{product.price}},"active":{{product.active}},"code":{{product.code}},"items":{{product.items}}}`
	configFile(t, dir, "body.json", template)
	configFile(t, dir, "literal.bin", literal)
	header := "\nX-Probe: derived-{{token}}\n"
	file := configFile(t, dir, "mixed.hurl", "GET {{base}}/capture"+header+"HTTP 200\n[Captures]\nfuture: jsonpath \"$.value\"\nPOST {{base}}/inline/{{future}}"+header+template+"\nHTTP 201\nPOST {{base}}/external"+header+"file,body.json;\nHTTP 201\nPOST {{base}}/multipart"+header+"[MultipartFormData]\nprice: price-{{product.price}}\nfile: file,literal.bin;\nHTTP 201")
	code, out, stderr := runConfig(t, t.TempDir(), "test", file, "--config", base, "--env", "local", "--render-files", "--var", "token=cli-fake-token")
	if code != 0 || stderr != "" || hits.Load() != 4 || !strings.Contains(out, "validadas=4") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
	for _, secret := range []string{"base-fake-token", "env-fake-token", "cli-fake-token", "captured-env-fake", wantJSON, literal} {
		if strings.Contains(out+stderr, secret) {
			t.Fatalf("leaked %q: %q %q", secret, out, stderr)
		}
	}
}

func TestEnvironmentSecretAssertions(t *testing.T) {
	for _, mode := range []string{"success", "status", "header", "json", "render", "missing future"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path != "/cli-fake-token" {
					t.Errorf("path=%s", r.URL.Path)
				}
				w.Header().Set("X-Value", "derived-cli-fake-token")
				if mode == "status" {
					w.WriteHeader(503)
				}
				_, _ = io.WriteString(w, `{"value":"derived-cli-fake-token"}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			base := configFile(t, dir, "base.yaml", "version: 1\nvariables: {token: base-fake-token}")
			environmentFile(t, dir, "local", "variables: {token: env-fake-token}\nsecrets: [token, future]")
			text := "GET " + server.URL + "/{{token}}\nHTTP 200\n"
			want, wantHits := 0, int32(1)
			switch mode {
			case "status":
				want = 1
			case "header":
				want = 1
				text += "[Asserts]\nheader \"X-Value\" == \"base-fake-token-env-fake-token\"\n"
			case "json":
				want = 1
				text += "[Asserts]\njsonpath \"$.value\" == \"base-fake-token-env-fake-token\"\n"
			case "render":
				want, wantHits = 2, 0
				text = "GET " + server.URL + "/{{token}}\nX-Probe: {{missing}}\nHTTP 200\n"
			case "missing future":
				want, wantHits = 2, 0
				text = "GET " + server.URL + "/{{future}}\nHTTP 200\n"
			}
			if mode != "success" {
				text += "GET " + server.URL + "/never\nHTTP 200"
			}
			file := configFile(t, dir, "base-fake-token-env-fake-token.hurl", text)
			code, out, stderr := runConfig(t, dir, "test", file, "--config", base, "--env", "local", "--var", "token=cli-fake-token")
			if code != want || hits.Load() != wantHits || mode == "success" && stderr != "" {
				t.Fatalf("code=%d want=%d out=%q err=%q hits=%d", code, want, out, stderr, hits.Load())
			}
			for _, secret := range []string{"base-fake-token", "env-fake-token", "cli-fake-token"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatalf("leaked %s: %q %q", secret, out, stderr)
				}
			}
		})
	}
}
