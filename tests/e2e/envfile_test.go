package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestEnvFileDemonstration(t *testing.T) {
	const token = "fiction-env-file-token"
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/profile" || r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("incorrect path or credential")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	for _, selected := range []bool{false, true} {
		dir := t.TempDir()
		// Config and scenario are elsewhere; the env-file path is cwd-relative.
		other := t.TempDir()
		base := configFile(t, other, ".httpgo.yaml", "version: 1\nvariables: {base_url: '${API_URL}', auth: {token: '${OWNER_TOKEN}'}}")
		file := configFile(t, other, "profile.httpgo", "GET {{base_url}}/profile\nAuthorization: Bearer {{auth.token}}\nHTTP 200")
		configFile(t, dir, ".env.local", "API_URL="+server.URL+"\nOWNER_TOKEN="+token)
		args := []string{"test", file, "--config", base, "--env-file", ".env.local"}
		if selected {
			environmentFile(t, other, "local", "variables: {auth: {token: '${OWNER_TOKEN}'}}")
			args = append(args, "--env", "local")
		}
		code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"API_URL": nil, "OWNER_TOKEN": nil}, args...)
		if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") || strings.Contains(out+stderr, token) {
			t.Fatalf("code=%d stderr=%q", code, stderr)
		}
		// Process URL wins even when the fallback URL cannot be used.
		configFile(t, dir, ".env.local", "API_URL=http://invalid.test\nOWNER_TOKEN="+token)
		value := server.URL
		code, _, stderr = runProcessEnvironment(t, dir, map[string]*string{"API_URL": &value, "OWNER_TOKEN": nil}, args...)
		if code != 0 || stderr != "" {
			t.Fatalf("process precedence code=%d err=%q", code, stderr)
		}
		// No automatic loading, neither cwd nor ancestor.
		configFile(t, dir, ".env", "invalid content")
		code, out, stderr = runProcessEnvironment(t, dir, map[string]*string{"API_URL": nil, "OWNER_TOKEN": nil}, "test", file, "--config", base)
		if code != 2 || out != "" || !strings.Contains(stderr, "ausente") {
			t.Fatalf("unselected source used: code=%d out=%q err=%q", code, out, stderr)
		}
		// Absolute selection is also accepted.
		args[5] = filepath.Join(dir, ".env.local")
		code, _, _ = runProcessEnvironment(t, dir, map[string]*string{"API_URL": &value, "OWNER_TOKEN": nil}, args...)
		if code != 0 {
			t.Fatalf("absolute source code=%d", code)
		}
	}
	if hits.Load() != 6 {
		t.Fatalf("hits=%d want=6", hits.Load())
	}
}

func TestEnvFileStringsAndLiteralContract(t *testing.T) {
	values := map[string]string{"NUMBER": "123", "BOOL": "true", "NIL": "null", "EMPTY": "", "LITERAL": `${OTHER}-$()-{{other}}-\n-#-=`, "QUOTED": "  inner spaces  ", "PROCESS": "", "UNQUOTED": "#literal=value"}
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		var got map[string]string
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		for key, want := range values {
			if value, ok := got[key]; !ok || value != want {
				t.Errorf("key=%s got=%q want=%q present=%t", key, value, want, ok)
			}
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	dir := t.TempDir()
	source := "version: 1\nvariables:\n"
	payload := make(map[string]string)
	environment := map[string]*string{}
	for name := range values {
		source += "  " + name + ": ${HTTPGO_43_" + name + "}\n"
		payload[name] = "{{" + name + "}}"
		environment["HTTPGO_43_"+name] = nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	empty := ""
	environment["HTTPGO_43_PROCESS"] = &empty
	configFile(t, dir, ".httpgo.yaml", source)
	file := configFile(t, dir, "strings.httpgo", "POST "+server.URL+"\n"+string(body)+"\nHTTP 200")
	configFile(t, dir, "values.env", " # comment\r\n HTTPGO_43_NUMBER = 123 \r\nHTTPGO_43_BOOL=true\nHTTPGO_43_NIL=null\nHTTPGO_43_EMPTY=\nHTTPGO_43_PROCESS=must-not-fallback\nHTTPGO_43_LITERAL='${OTHER}-$()-{{other}}-\\n-#-='\nHTTPGO_43_QUOTED=\"  inner spaces  \"\nHTTPGO_43_UNQUOTED=#literal=value\nHTTPGO_TIMEOUT=0.001\nNO_COLOR=1")
	code, _, stderr := runProcessEnvironment(t, dir, environment, "test", file, "--env-file=values.env")
	if code != 0 || stderr != "" || hits.Load() != 1 {
		t.Fatalf("code=%d hits=%d err=%q", code, hits.Load(), stderr)
	}
}

func TestEnvFileInvalidProcessContract(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	dir := t.TempDir()
	file := configFile(t, dir, "run.httpgo", "GET "+server.URL+"\nHTTP 200")
	configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables: {token: '${HTTPGO_43_PRESENT}'}")
	present := "fiction-from-process"
	environment := map[string]*string{"HTTPGO_43_PRESENT": &present, "HTTPGO_43_MISSING": nil}
	for _, source := range []string{"A=fiction-env-sentinel\nA=fiction-env-sentinel", "A=fiction-env-sentinel\nbroken", "export A=fiction-env-sentinel", "1A=fiction-env-sentinel", "A='fiction-env-sentinel", "A=\"fiction-env-sentinel\" # tail", "A='fiction-env-sentinel\nsecond'"} {
		configFile(t, dir, "invalid.env", source)
		code, out, stderr := runProcessEnvironment(t, dir, environment, "test", file, "--env-file", "invalid.env", "--var", "token=substitute-fiction", "--output", "junit", "--output-file", "report.xml")
		if code != 2 || out != "" || !strings.Contains(stderr, "invalid.env") || strings.Contains(stderr, "fiction-env-sentinel") || hits.Load() != 0 {
			t.Fatalf("invalid source code=%d hits=%d err=%q", code, hits.Load(), stderr)
		}
	}
	for _, flags := range [][]string{{"--env-file"}, {"--env-file="}, {"--env-file", ""}, {"--env-file=invalid.env", "--env-file", "invalid.env"}, {"--env-file", "missing.env"}, {"--env-file", dir}} {
		args := []string{"test", file, "--output", "junit", "--output-file", "report.xml"}
		code, out, stderr := runProcessEnvironment(t, dir, environment, append(args, flags...)...)
		if code != 2 || out != "" || stderr == "" || hits.Load() != 0 {
			t.Fatalf("flags=%v code=%d hits=%d err=%q", flags, code, hits.Load(), stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "report.xml")); !os.IsNotExist(err) {
		t.Fatal("preparation created report")
	}
	configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables: {unused: '${HTTPGO_43_MISSING}'}")
	configFile(t, dir, "empty.env", "# comments only\n")
	code, out, stderr := runProcessEnvironment(t, dir, environment, "test", file, "--env-file", "empty.env")
	if code != 2 || out != "" || !strings.Contains(stderr, "HTTPGO_43_MISSING") || !strings.Contains(stderr, "--env-file") || hits.Load() != 0 {
		t.Fatalf("missing reference code=%d err=%q", code, stderr)
	}
}

func TestEnvFilePrecedenceExtrasAndFreshProcess(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1); w.WriteHeader(200) }))
	defer server.Close()
	dir := t.TempDir()
	file := configFile(t, dir, "run.httpgo", "GET {{url}}\nHTTP 200")
	configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables: {url: '${HTTPGO_43_DISCARDED}'}")
	environmentFile(t, dir, "local", "variables: {url: "+server.URL+"}")
	configFile(t, dir, "values.env", "HTTPGO_TIMEOUT=0.001\nNO_COLOR=1\nEXTRA=unreferenced\n")
	for _, flags := range [][]string{{"--env", "local"}, {"--var", "url=" + server.URL}} {
		args := append([]string{"test", file, "--env-file", "values.env"}, flags...)
		code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"HTTPGO_43_DISCARDED": nil}, args...)
		if code != 0 || stderr != "" || strings.Contains(out, server.URL) {
			t.Fatalf("override code=%d err=%q", code, stderr)
		}
	}
	file = configFile(t, dir, "extra.httpgo", "GET "+server.URL+"/{{EXTRA}}\nHTTP 200")
	code, _, _ := runProcessEnvironment(t, dir, map[string]*string{"HTTPGO_43_DISCARDED": nil}, "test", file, "--env", "local", "--env-file", "values.env")
	if code != 2 || hits.Load() != 2 {
		t.Fatal("extra key imported")
	}
	// Valid empty file, and no discovery of invalid .env in cwd or ancestor.
	configFile(t, dir, "empty.env", "")
	configFile(t, dir, ".env", "invalid")
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	configFile(t, child, ".env", "invalid")
	file = configFile(t, child, "plain.httpgo", "GET "+server.URL+"\nHTTP 200")
	for _, flags := range [][]string{nil, {"--env-file", filepath.Join(dir, "empty.env")}} {
		code, _, stderr := runProcessEnvironment(t, child, nil, append([]string{"test", file}, flags...)...)
		if code != 0 || stderr != "" {
			t.Fatalf("unselected file/empty source code=%d err=%q", code, stderr)
		}
	}
	if hits.Load() != 4 {
		t.Fatalf("hits=%d want=4", hits.Load())
	}
}

func TestEnvFileSuiteReload(t *testing.T) {
	var hits atomic.Int32
	dir := t.TempDir()
	path := configFile(t, dir, "values.env", "OWNER_TOKEN=first-fiction")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		want := "first-fiction"
		if n > 2 {
			want = "second-fiction"
		}
		if r.Header.Get("X-Token") != want {
			t.Error("source changed mid-suite or was not reloaded")
		}
		if err := os.WriteFile(path, []byte("OWNER_TOKEN=second-fiction"), 0600); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"captured"}`)
	}))
	defer server.Close()
	configFile(t, dir, ".httpgo.yaml", "version: 1\nvariables: {token: '${OWNER_TOKEN}'}")
	first := configFile(t, dir, "first.httpgo", "GET "+server.URL+"\nX-Token: {{token}}\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"")
	second := configFile(t, dir, "second.httpgo", "GET "+server.URL+"\nX-Token: {{token}}\nHTTP 200\n[Captures]\nid: jsonpath \"$.id\"")
	for range 2 {
		code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"OWNER_TOKEN": nil}, "test", first, second, "--env-file", "values.env")
		if code != 0 || stderr != "" || strings.Contains(out, "first-fiction") || strings.Contains(out, "second-fiction") {
			t.Fatalf("suite code=%d err=%q", code, stderr)
		}
	}
	code, out, stderr := runProcessEnvironment(t, dir, map[string]*string{"OWNER_TOKEN": nil}, "test", first, second)
	if code != 2 || out != "" || !strings.Contains(stderr, "ausente") || hits.Load() != 4 {
		t.Fatal("file source leaked into next invocation without flag")
	}
}

func TestEnvFileGuideFixturesAndReports(t *testing.T) {
	const token = "token-de-demonstracao"
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("credential changed")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"echo":"prefix-`+token+`"}`)
	}))
	defer server.Close()
	dir := t.TempDir()
	configFile(t, dir, ".env.local", strings.ReplaceAll(guideBlock(t, "docs/configuration.md", "Salve `.env.local`", "dotenv"), "http://localhost:8080", server.URL))
	configFile(t, dir, "local.yaml", guideBlock(t, "docs/configuration.md", "Salve `local.yaml`", "yaml"))
	text := guideBlock(t, "docs/configuration.md", "Salve `profile.httpgo`", "hurl")
	file := configFile(t, dir, "profile.httpgo", text)
	environment := map[string]*string{"API_URL": nil, "OWNER_TOKEN": nil}
	code, _, stderr := runProcessEnvironment(t, dir, environment, "test", file, "--config", "local.yaml", "--env-file", ".env.local")
	if code != 0 || stderr != "" {
		t.Fatalf("guide code=%d err=%q", code, stderr)
	}
	configFile(t, dir, "profile.httpgo", text+"\n[Asserts]\njsonpath \"$.echo\" == \"different\"\n")
	for _, mode := range []string{"text", "verbose", "junit", "html"} {
		args := []string{"test", file, "--config", "local.yaml", "--env-file", ".env.local"}
		switch mode {
		case "verbose":
			args = append(args, "-vv")
		case "junit", "html":
			args = append(args, "--output", mode, "--output-file", "report."+mode)
		}
		code, out, stderr := runProcessEnvironment(t, dir, environment, args...)
		shown := out + stderr
		if mode == "junit" || mode == "html" {
			paths, err := filepath.Glob(filepath.Join(dir, "report*"))
			if err != nil || len(paths) == 0 {
				t.Fatal("missing report")
			}
			for _, path := range paths {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				shown += string(data)
			}
		}
		if code != 1 || strings.Contains(shown, token) || !strings.Contains(shown, "[oculto]") {
			t.Fatalf("mode=%s code=%d safe=%t", mode, code, !strings.Contains(shown, token))
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixtureArgs := []string{"test", filepath.Join(root, "testdata/scenarios/scenario_envfile_profile.httpgo"), "--config", filepath.Join(root, "testdata/config/envfile_base.yaml"), "--env-file", filepath.Join(root, "testdata/config/envfile.local")}
	url := server.URL
	code, _, stderr = runProcessEnvironment(t, dir, map[string]*string{"API_URL": &url, "OWNER_TOKEN": nil}, fixtureArgs...)
	if code != 0 || stderr != "" {
		t.Fatalf("fixture code=%d err=%q", code, stderr)
	}
	fixtureArgs[5] = filepath.Join(root, "testdata/config/envfile_invalid.local")
	code, out, stderr := runProcessEnvironment(t, dir, environment, fixtureArgs...)
	if code != 2 || out != "" || strings.Contains(stderr, token) || hits.Load() != 6 {
		t.Fatalf("invalid fixture code=%d hits=%d", code, hits.Load())
	}
}
