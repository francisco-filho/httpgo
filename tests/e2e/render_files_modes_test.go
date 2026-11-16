package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestExternalRenderedMixedModesByBinary(t *testing.T) {
	const secret = `"credential-fictitious\"\\\n雪"`
	for _, mode := range []string{"success", "mismatch", "assertion", "incomplete", "render"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch r.URL.Path {
				case "/capture":
					_, _ = io.WriteString(w, `{"token":`+secret+`,"product":{"id":9007199254740993,"active":false,"empty":null,"array":[true,"{{literal}}"]}}`)
					return
				case "/inline":
					body, err := io.ReadAll(r.Body)
					if err != nil || string(body) != `{"inline":42,"literal":"{{literal}}"}` {
						t.Errorf("inline body=%q err=%v", body, err)
					}
				case "/external":
					body, err := io.ReadAll(r.Body)
					var got struct {
						Data  json.RawMessage `json:"data"`
						Token string          `json:"token"`
					}
					if err != nil || json.Unmarshal(body, &got) != nil || string(got.Data) != `{"active":false,"array":[true,"{{literal}}"],"empty":null,"id":9007199254740993}` {
						t.Errorf("external body=%q err=%v", body, err)
					}
					var token string
					if err := json.Unmarshal([]byte(secret), &token); err != nil {
						t.Error(err)
					}
					if got.Token != "prefix-"+token {
						t.Errorf("token=%q", got.Token)
					}
					if mode == "mismatch" {
						w.WriteHeader(503)
						return
					}
					if mode == "incomplete" {
						w.Header().Set("Content-Length", "100")
						w.WriteHeader(201)
						_, _ = io.WriteString(w, "short")
						return
					}
					if mode == "assertion" {
						w.Header().Set("X-Result", "unexpected")
					}
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
					if err != nil || field.FormName() != "description" || string(text) != "typed-42" {
						t.Errorf("field=%v data=%q err=%v", field.Header, text, err)
					}
					part, err := reader.NextPart()
					if err != nil {
						t.Error(err)
						return
					}
					data, err := io.ReadAll(part)
					if err != nil || part.FormName() != "document" || string(data) != "\x00\xff{{missing}}" {
						t.Errorf("file=%v data=%q err=%v", part.Header, data, err)
					}
					if _, err := reader.NextPart(); err != io.EOF {
						t.Errorf("extra part err=%v", err)
					}
				case "/last":
					w.WriteHeader(200)
					return
				default:
					t.Errorf("unexpected path=%s", r.URL.Path)
				}
				w.WriteHeader(201)
			}))
			defer server.Close()
			text := "GET " + server.URL + "/capture\nHTTP 200\n[Captures]\ncredential: jsonpath \"$.token\"\nproduct: jsonpath \"$.product\"\nPOST " + server.URL + "/inline\n{\"inline\":{{value}},\"literal\":{{literal}}}\nHTTP 201\nPOST " + server.URL + "/external\nfile,payload.data;\nHTTP 201\n"
			if mode == "assertion" {
				text += "[Asserts]\nheader \"X-Result\" == \"expected\"\n"
			}
			text += "POST " + server.URL + "/multipart\n[MultipartFormData]\ndescription: typed-{{value}}\ndocument: file,upload.bin;\nHTTP 201\nGET " + server.URL + "/last\nHTTP 200"
			file := scenario(t, text)
			payload := `{"data":{{product}},"token":"prefix-{{credential}}"}`
			if mode == "render" {
				payload = `{"data":{{product}},"token":"prefix-{{credential}}","missing":{{missing}}}`
			}
			for name, body := range map[string]string{"payload.data": payload, "upload.bin": "\x00\xff{{missing}}"} {
				if err := os.WriteFile(filepath.Join(filepath.Dir(file), name), []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			code, out, stderr := run(t, "test", file, "--render-files", "--var", "value=42", "--var", `literal="{{literal}}"`, "--secret", "credential")
			wantCode, wantHits := 0, int32(5)
			switch mode {
			case "mismatch", "assertion":
				wantCode, wantHits = 1, 3
			case "incomplete":
				wantCode, wantHits = 2, 3
			case "render":
				wantCode, wantHits = 2, 2
			}
			if code != wantCode || hits.Load() != wantHits || !strings.Contains(out, "iniciadas="+fmt.Sprint(wantHits)) || mode == "success" && stderr != "" {
				t.Fatalf("code=%d hits=%d out=%q err=%q", code, hits.Load(), out, stderr)
			}
			for _, s := range []string{"credential-fictitious", secret, payload, "{{missing}}", "9007199254740993"} {
				if strings.Contains(out+stderr, s) {
					t.Fatalf("unsafe output=%q", out+stderr)
				}
			}
		})
	}
}

func TestExternalRenderedOriginAcrossProcessDirectories(t *testing.T) {
	root := t.TempDir()
	scenarioDir := filepath.Join(root, "project", "tests")
	if err := os.MkdirAll(scenarioDir, 0700); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(root, " {{literal}} payload.data ")
	source := " \r\n{\r\n \"value\":true\r\n}\r\n"
	if err := os.WriteFile(payload, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != source || r.Header.Get("Content-Type") != "text/plain" {
			t.Errorf("body=%q headers=%v err=%v", body, r.Header, err)
		}
		hits.Add(1)
		w.WriteHeader(201)
	}))
	defer server.Close()
	file := filepath.Join(scenarioDir, "source.hurl")
	for _, ref := range []string{filepath.Join("..", "..", filepath.Base(payload)), payload} {
		if err := os.WriteFile(file, []byte("POST "+server.URL+"\nContent-Type: text/plain\nfile,"+ref+";\nHTTP 201"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, cwd := range []string{root, t.TempDir()} {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			command := exec.CommandContext(ctx, binary(t), "test", file, "--render-files")
			command.Dir = cwd
			var out, stderr bytes.Buffer
			command.Stdout, command.Stderr = &out, &stderr
			err := command.Run()
			cancel()
			if err != nil || stderr.Len() != 0 || !strings.Contains(out.String(), "validadas=1") {
				t.Fatalf("cwd=%q ref=%q err=%v out=%q stderr=%q", cwd, ref, err, out.String(), stderr.String())
			}
		}
	}
	if hits.Load() != 4 {
		t.Fatalf("hits=%d", hits.Load())
	}
}

func TestExternalRenderedStructuralErrorPreventsAllNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	for _, suffix := range []string{"file,;", "file,payload;\n{}", "file,payload;\nX-Test: after body"} {
		file := scenario(t, "GET "+server.URL+"/first\nHTTP 200\nPOST "+server.URL+"\n"+suffix)
		code, out, stderr := run(t, "test", file, "--render-files")
		if code != 2 || !prevalidationSummary(out) || stderr == "" || hits.Load() != 0 {
			t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
		}
	}
}
