package e2e

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

var junitArtifactDir = flag.String("junit-artifact-dir", "", "retain real CLI JUnit artifacts for CI consumer validation")

type junitResult struct {
	Tests    int     `xml:"tests,attr"`
	Failures int     `xml:"failures,attr"`
	Errors   int     `xml:"errors,attr"`
	Skipped  int     `xml:"skipped,attr"`
	Time     float64 `xml:"time,attr"`
	Cases    []struct {
		Name    string  `xml:"name,attr"`
		Time    float64 `xml:"time,attr"`
		Failure *string `xml:"failure"`
		Error   *string `xml:"error"`
		Skipped *string `xml:"skipped"`
	} `xml:"testcase"`
}

func junitXML(t *testing.T, path string) (junitResult, string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result junitResult
	if err := xml.Unmarshal(data, &result); err != nil {
		t.Fatalf("XML: %v: %s", err, data)
	}
	return result, string(data)
}

func TestJUnitRealSuiteAndUnchangedRequests(t *testing.T) {
	for _, mode := range []string{"success", "mixed", "fail-fast", "transport-fast"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.URL.Path == "/assertion" && mode != "success" && mode != "transport-fast" {
					w.WriteHeader(503)
					return
				}
				if r.URL.Path == "/transport" && mode != "success" {
					w.Header().Set("Content-Length", "999") // A real incomplete HTTP response.
				}
				w.WriteHeader(200)
			}))
			defer server.Close()
			dir := t.TempDir()
			configFile(t, dir, "01-ação & \"aspas\" <ok>.hurl", "GET "+server.URL+"/ok\nHTTP 200")
			configFile(t, dir, "02-assertion.hurl", "GET "+server.URL+"/assertion\nHTTP 200")
			configFile(t, dir, "03-transport.hurl", "GET "+server.URL+"/transport\nHTTP 200")
			configFile(t, dir, "04-last.hurl", "GET "+server.URL+"/last\nHTTP 200")
			args := []string{"test", dir}
			if strings.HasSuffix(mode, "fast") {
				args = append(args, "--fail-fast")
			}
			plainCode, plainOut, plainErr := run(t, args...)
			plainHits := hits.Swap(0)
			path := filepath.Join(t.TempDir(), "target", "results.xml")
			code, out, stderr := run(t, append(args, "--output", "junit", "--output-file", path)...)
			doc, raw := junitXML(t, path)
			wantCode, failures, executionErrors, skipped, calls := 0, 0, 0, 0, 4
			switch mode {
			case "mixed":
				wantCode, failures, executionErrors = 2, 1, 1
			case "fail-fast":
				wantCode, failures, skipped, calls = 1, 1, 2, 2
			case "transport-fast":
				wantCode, executionErrors, skipped, calls = 2, 1, 1, 3
			}
			if code != wantCode || code != plainCode || hits.Load() != int32(calls) || plainHits != hits.Load() || out != "" || doc.Tests != 4 || len(doc.Cases) != 4 || doc.Failures != failures || doc.Errors != executionErrors || doc.Skipped != skipped || doc.Time < 0 {
				t.Fatalf("mode=%s code=%d plain=%d hits=%d plainHits=%d out=%q stderr=%q XML=%s", mode, code, plainCode, hits.Load(), plainHits, out, stderr, raw)
			}
			if !strings.Contains(plainOut, "Cenários executados:") || stderr != plainErr {
				t.Fatalf("plain mode differs: out=%q err=%q JUnit err=%q", plainOut, plainErr, stderr)
			}
			if doc.Cases[0].Name != suiteDisplayPath(t, filepath.Join(dir, "01-ação & \"aspas\" <ok>.hurl")) || doc.Cases[0].Failure != nil || doc.Cases[0].Error != nil || doc.Cases[0].Skipped != nil {
				t.Fatalf("identity/success: %+v", doc.Cases[0])
			}
			for _, tc := range doc.Cases {
				if tc.Time < 0 || (tc.Skipped != nil && tc.Time != 0) {
					t.Fatalf("invalid duration: %+v", tc)
				}
			}
			if *junitArtifactDir != "" {
				if err := os.MkdirAll(*junitArtifactDir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(*junitArtifactDir, mode+".xml"), []byte(raw), 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestJUnitRealSecretsAndEscaping(t *testing.T) {
	const token = "fiction-junit-env-<&\"secret"
	const captured = "fiction-junit-captured-secret"
	t.Setenv("HTTPGO_JUNIT_TOKEN", token)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("environment secret not sent intact")
		}
		if r.URL.Path == "/login" {
			_, _ = fmt.Fprintf(w, `{"token":%q}`, captured)
			return
		}
		if r.Header.Get("X-Token") != captured {
			t.Error("captured secret not sent intact")
		}
		w.Header().Set("X-Echo", token)
		_ = json.NewEncoder(w).Encode(map[string]string{"echo": captured})
	}))
	defer server.Close()
	root := t.TempDir()
	base := configFile(t, root, "base.yaml", "version: 1\nvariables:\n  token: ${HTTPGO_JUNIT_TOKEN}\nsecrets: [captured]")
	file := configFile(t, root, "ação & \"assertion\".hurl", "GET "+server.URL+"/login\nAuthorization: Bearer {{token}}\nHTTP 200\n[Captures]\ncaptured: jsonpath \"$.token\"\nGET "+server.URL+"/echo\nAuthorization: Bearer {{token}}\nX-Token: {{captured}}\nHTTP 200\n[Asserts]\njsonpath \"$.echo\" == \"ação & \\\"<different>\\\"\"")
	path := filepath.Join(root, "results.xml")
	code, out, stderr := run(t, "test", file, "--config", base, "--output=junit", "--output-file", path)
	doc, raw := junitXML(t, path)
	if code != 1 || out != "" || hits.Load() != 2 || doc.Failures != 1 || doc.Cases[0].Failure == nil || !strings.Contains(*doc.Cases[0].Failure, "assertion jsonpath") || !strings.Contains(*doc.Cases[0].Failure, "ação &") || !strings.Contains(raw, "[oculto]") {
		t.Fatalf("code=%d hits=%d out=%q stderr=%q XML=%s", code, hits.Load(), out, stderr, raw)
	}
	for _, secret := range []string{token, captured, "fiction-junit-env-", "fiction-junit-captured-"} {
		if strings.Contains(out+stderr+raw, secret) {
			t.Fatalf("secret appeared in output: %q", secret)
		}
	}
}

func TestJUnitRealPreexecutionAndDestinationErrors(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(503)
	}))
	defer server.Close()
	file := scenario(t, "GET "+server.URL+"\nHTTP 200")
	for _, mode := range []string{"config", "format", "missing destination", "directory", "parent file"} {
		t.Run(mode, func(t *testing.T) {
			hits.Store(0)
			path := filepath.Join(t.TempDir(), "results.xml")
			args := []string{"test", file, "--output=junit", "--output-file", path}
			wantHits := int32(0)
			switch mode {
			case "config":
				args = append(args, "--config", configFile(t, t.TempDir(), "invalid.yaml", "version: 999"))
			case "format":
				args[2] = "--output=json"
			case "missing destination":
				args = args[:3]
			case "directory":
				args[4], wantHits = t.TempDir(), 1
			case "parent file":
				args[4], wantHits = filepath.Join(file, "results.xml"), 1
			}
			code, out, stderr := run(t, args...)
			_, err := os.Stat(path)
			if code != 2 || out != "" || stderr == "" || strings.Contains(stderr, "publicado") || hits.Load() != wantHits || !os.IsNotExist(err) {
				t.Fatalf("code=%d hits=%d out=%q stderr=%q artifact err=%v", code, hits.Load(), out, stderr, err)
			}
		})
	}
}
