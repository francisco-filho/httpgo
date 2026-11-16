package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSuiteRealIsolationProtectionAndPayloadOrigins(t *testing.T) {
	const token = "fiction-suite-token"
	const cookie = "fiction-suite-cookie"
	const initial = "fiction-process-initial"
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if r.Header.Get("Cookie") != "" {
			t.Error("implicit cookie crossed requests/files")
		}
		switch r.URL.Path {
		case "/login":
			w.Header().Set("Set-Cookie", cookie)
			_, _ = io.WriteString(w, `{"token":"`+token+`"}`)
		case "/json/a", "/json/b":
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			var body map[string]string
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Error(err)
				return
			}
			name := strings.TrimPrefix(r.URL.Path, "/json/")
			if body["from"] != name || body["initial"] != initial {
				t.Errorf("body=%s", raw)
			}
			if name == "a" && (body["token"] != token || r.Header.Get("Authorization") != "Bearer "+token) {
				t.Error("operational capture changed")
			}
		case "/multipart":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
				return
			}
			defer func() { _ = r.MultipartForm.RemoveAll() }()
			if r.FormValue("text") != "prefix-"+token {
				t.Error("operational derived field changed")
			}
			file, header, err := r.FormFile("document")
			if err != nil {
				t.Error(err)
				return
			}
			raw, err := io.ReadAll(file)
			if err != nil {
				t.Error(err)
			}
			if err := file.Close(); err != nil {
				t.Error(err)
			}
			if string(raw) != "literal {{token}}\x00" || header.Filename != "document.bin" {
				t.Error("multipart bytes/name changed")
			}
		case "/echo":
			w.Header().Set("X-Echo", "prefix-"+token+" "+cookie)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
	defer server.Close()
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	for _, dir := range []string{a, b} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	configFile(t, a, "01.hurl", "POST {{base}}/login\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\"\nPOST {{base}}/json/a\nAuthorization: Bearer {{token}}\nfile,payload.json;\nHTTP 200\nPOST {{base}}/multipart\n[MultipartFormData]\ntext: prefix-{{token}}\ndocument: file,document.bin;\nHTTP 200")
	configFile(t, a, "payload.json", `{"from":"a","initial":"{{initial}}","token":"{{token}}"}`)
	configFile(t, a, "document.bin", "literal {{token}}\x00")
	configFile(t, b, "02.hurl", "POST {{base}}/json/b\nfile,payload.json;\nHTTP 200")
	configFile(t, b, "payload.json", `{"from":"b","initial":"{{initial}}"}`)
	configFile(t, b, "03.hurl", "GET {{base}}/{{token}}")
	configFile(t, b, "04-"+token+".hurl", "GET {{base}}/echo\nHTTP 200\n[Asserts]\nheader \"X-Echo\" == \"different\"")
	configFile(t, b, "05-"+cookie+".hurl", "GET {{base}}/never\nHEAD {{base}}/invalid")
	configDir := t.TempDir()
	config := configFile(t, configDir, "base.yaml", "version: 1\nvariables:\n  base: "+server.URL+"\n  initial: ${HTTPGO_SUITE_INITIAL}\nsecrets: [token]")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary(t), "test", root, "--config", config, "--render-files")
	secret := initial
	command.Env = childEnvironment(map[string]*string{"HTTPGO_SUITE_INITIAL": &secret})
	var out, stderr bytes.Buffer
	command.Stdout = &out
	command.Stderr = &stderr
	err := command.Run()
	mu.Lock()
	got := strings.Join(paths, " ")
	mu.Unlock()
	if err == nil || command.ProcessState == nil || command.ProcessState.ExitCode() != 2 || got != "/login /json/a /multipart /json/b /echo" || !strings.Contains(stderr.String(), "variável ausente") || !strings.Contains(stderr.String(), "HEAD") || !strings.Contains(out.String(), "sucessos=2 falhas_assertion=1 erros=2") {
		t.Fatalf("state=%v err=%v paths=%v out=%q stderr=%q", command.ProcessState, err, got, out.String(), stderr.String())
	}
	for _, secret := range []string{token, cookie, initial, "prefix-"} {
		if strings.Contains(out.String()+stderr.String(), secret) {
			t.Fatalf("secret/derived %q leaked", secret)
		}
	}
}
