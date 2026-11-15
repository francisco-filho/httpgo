package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestProcessEnvironmentPublicStringTypes(t *testing.T) {
	for _, value := range []string{"123", "true", "null", "", "quote\"雪\n\\", "${OTHER}", "{{other}}"} {
		t.Run(value, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				var got struct {
					Token   string
					Sibling bool
					Items   []any
				}
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil || got.Token != value || !got.Sibling || len(got.Items) != 2 {
					t.Error("process content/type or typed siblings changed")
				}
				w.WriteHeader(201)
			}))
			defer server.Close()
			dir := t.TempDir()
			configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables: {token: '${OWNER_TOKEN}', sibling: true, items: [null, 123]}\ndefaults: {timeout: 2.5}")
			file := configFile(t, dir, "strings.hurl", "POST "+server.URL+"\n{\"token\":{{token}},\"sibling\":{{sibling}},\"items\":{{items}}}\nHTTP 201")
			// Unreferenced process options must not replace the configured URL/data.
			automatic := "http://invalid.test"
			code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"OWNER_TOKEN": &value, "OTHER": nil, "HTTPGO_BASE_URL": &automatic, "HTTPGO_TIMEOUT": &automatic}, "test", file)
			if code != 0 || stderr != "" || hits.Load() != 1 || !strings.Contains(out, "validadas=1") {
				t.Fatalf("code=%d hits=%d", code, hits.Load())
			}
		})
	}
}

func TestProcessEnvironmentPublicPrecedence(t *testing.T) {
	for _, mode := range []string{"environment literal", "environment reference", "CLI substitute"} {
		t.Run(mode, func(t *testing.T) {
			const value = "substitute-fiction"
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path != "/"+value || r.Header.Get("X-Probe") != "derived-"+value {
					t.Error("override value changed")
				}
				w.Header().Set("X-Echo", value)
				w.WriteHeader(200)
			}))
			defer server.Close()
			dir := t.TempDir()
			configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables: {token: '${DISCARDED_TOKEN}'}")
			source := "variables: {token: substitute-fiction}"
			if mode != "environment literal" {
				source = "variables: {token: '${OWNER_TOKEN}'}"
			}
			environmentFile(t, dir, "local", source)
			file := configFile(t, dir, "substitute-fiction.hurl", "GET "+server.URL+"/{{token}}\nX-Probe: derived-{{token}}\nHTTP 200\n[Asserts]\nheader \"X-Echo\" == \"different\"\nGET "+server.URL+"/never")
			args := []string{"test", file, "--env", "local"}
			processValue := value
			environment := map[string]*string{"DISCARDED_TOKEN": nil, "OWNER_TOKEN": &processValue}
			if mode == "CLI substitute" {
				args = append(args, "--var", "token="+value)
				environment["OWNER_TOKEN"] = nil
			}
			code, out, stderr := runProcessEnvironment(t, dir, environment, args...)
			if code != 1 || hits.Load() != 1 || strings.Contains(out+stderr, value) || !strings.Contains(stderr, "[oculto]") {
				t.Fatalf("mode=%s code=%d hits=%d safe=%v", mode, code, hits.Load(), !strings.Contains(out+stderr, value))
			}
		})
	}
}

func TestProcessEnvironmentPublicInvalidBeforeNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	for _, source := range []string{"'${OWNER_TOKEN}'", "'Bearer ${OWNER_TOKEN}'", "'${A}${B}'", "'${1BAD}'", "'${A B}'", "'${TOKEN'", "['${OWNER_TOKEN}']", "[{nested: ['${OWNER_TOKEN}']}]"} {
		for _, selected := range []bool{false, true} {
			dir := t.TempDir()
			text := "variables: {token: " + source + "}"
			args := []string{"test", configFile(t, dir, "invalid.hurl", "GET "+server.URL+"\nHTTP 200")}
			origin := ".httpgo.yaml"
			if selected {
				configFile(t, dir, ".httpgo.yaml", "version: 1")
				environmentFile(t, dir, "local", text)
				origin = filepath.Join("environments", "local.yaml")
				args = append(args, "--env", "local")
			} else {
				configFile(t, dir, ".httpgo.yaml", "version: 1\n"+text)
			}
			for _, override := range []bool{false, true} {
				if source == "'${OWNER_TOKEN}'" && override {
					continue
				}
				commandArgs := append([]string(nil), args...)
				if override {
					commandArgs = append(commandArgs, "--var", "token=fiction")
				}
				code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"OWNER_TOKEN": nil}, commandArgs...)
				if code != 2 || out != "" || !strings.Contains(stderr, origin) || hits.Load() != 0 {
					t.Fatalf("source=%s selected=%v override=%v code=%d hits=%d", source, selected, override, code, hits.Load())
				}
				if source == "'${OWNER_TOKEN}'" && (!strings.Contains(stderr, "OWNER_TOKEN") || !strings.Contains(stderr, "variables.token")) {
					t.Fatal("missing reference not identified")
				}
			}
		}
	}
}

func TestProcessEnvironmentPublicEchoAssertionsAndEarlyFailures(t *testing.T) {
	const value = "echo-fiction-token"
	for _, mode := range []string{"header actual", "header expected", "json actual", "json expected", "syntax", "missing reference first", "missing reference last", "arguments", "CLI duplicate", "transport"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			expected, actual := "different", "prefix-"+value
			if strings.Contains(mode, "expected") {
				expected, actual = actual, expected
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Header.Get("X-Probe") != value {
					t.Error("credential did not reach server")
				}
				if mode == "transport" {
					connection, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = connection.Close()
					return
				}
				w.Header().Set("X-Echo", actual)
				_, _ = fmt.Fprintf(w, `{"value":%q}`, actual)
			}))
			defer server.Close()
			dir := t.TempDir()
			baseText := "version: 1\nvariables: {token: '${OWNER_TOKEN}'}"
			if strings.HasPrefix(mode, "missing reference") {
				key := "aMissing"
				if strings.HasSuffix(mode, "last") {
					key = "zMissing"
				}
				baseText = "version: 1\nvariables: {token: '${OWNER_TOKEN}', " + key + ": '${ABSENT_TOKEN}'}"
			}
			base := configFile(t, dir, value+".yaml", baseText)
			text := "GET " + server.URL + "/{{token}}\nX-Probe: {{token}}\nHTTP 200\n"
			want, wantHits := 2, int32(0)
			switch {
			case strings.HasPrefix(mode, "header"):
				text += fmt.Sprintf("[Asserts]\nheader \"X-Echo\" == %q\n", expected)
				want, wantHits = 1, 1
			case strings.HasPrefix(mode, "json"):
				text += fmt.Sprintf("[Asserts]\njsonpath \"$.value\" == %q\n", expected)
				want, wantHits = 1, 1
			case mode == "syntax":
				text += "HEAD " + server.URL + "\n"
			case mode == "transport":
				wantHits = 1
			}
			text += "GET " + server.URL + "/never\nHTTP 200"
			file := configFile(t, dir, value+".hurl", text)
			args := []string{"test", file, "--config", base}
			if mode == "arguments" {
				args = append(args, "--unsupported")
			}
			if mode == "CLI duplicate" {
				args = append(args, "--var", "token=duplicate-fiction", "--var", "token=duplicate-fiction")
			}
			processValue := value
			code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"OWNER_TOKEN": &processValue, "ABSENT_TOKEN": nil}, args...)
			// CLI override discards the reference, so only the substitute is known.
			if mode != "CLI duplicate" && strings.Contains(out+stderr, value) {
				t.Fatal("known process value leaked")
			}
			if code != want || hits.Load() != wantHits || stderr == "" || strings.Contains(out+stderr, "duplicate-fiction") {
				t.Fatalf("mode=%s code=%d want=%d hits=%d wantHits=%d", mode, code, want, hits.Load(), wantHits)
			}
		})
	}
}

func TestProcessEnvironmentPublicMultipartAndExternal(t *testing.T) {
	const value = "multipart-fiction-token"
	const literal = "\x00\xff{{token}}\r\n"
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("X-Probe") != "prefix-"+value {
			t.Error("credential changed")
		}
		if r.URL.Path == "/multipart" {
			reader, err := r.MultipartReader()
			if err != nil {
				t.Error(err)
				return
			}
			for _, want := range []string{"prefix-" + value, literal} {
				part, err := reader.NextPart()
				if err != nil {
					t.Error(err)
					return
				}
				data, err := io.ReadAll(part)
				if err != nil || string(data) != want {
					t.Error("multipart bytes changed")
				}
			}
			if _, err := reader.NextPart(); err != io.EOF {
				t.Error("unexpected part")
			}
		} else {
			var got struct{ Token string }
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil || got.Token != "prefix-"+value {
				t.Error("body changed")
			}
		}
		w.WriteHeader(201)
	}))
	defer server.Close()
	dir := t.TempDir()
	base := configFile(t, t.TempDir(), "base.yaml", "version: 1\nvariables: {token: '${OWNER_TOKEN}'}")
	body := `{"token":"prefix-{{token}}"}`
	configFile(t, dir, "payload.json", body)
	configFile(t, dir, "upload.bin", literal)
	header := "\nX-Probe: prefix-{{token}}\n"
	file := configFile(t, dir, "modes.hurl", "POST "+server.URL+"/inline"+header+body+"\nHTTP 201\nPOST "+server.URL+"/external"+header+"file,payload.json;\nHTTP 201\nPOST "+server.URL+"/multipart"+header+"[MultipartFormData]\ndescription: prefix-{{token}}\nupload: file,upload.bin;\nHTTP 201")
	processValue := value
	code, out, stderr := runProcessEnvironment(t, t.TempDir(), map[string]*string{"OWNER_TOKEN": &processValue}, "test", file, "--config", base, "--render-files")
	if code != 0 || stderr != "" || hits.Load() != 3 || strings.Contains(out+stderr, value) {
		t.Fatalf("code=%d hits=%d safe=%v", code, hits.Load(), !strings.Contains(out+stderr, value))
	}
}
