//go:build compatibility

package e2e

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const hurlVersion = "8.0.0"

var updateCompatibility = flag.Bool("update", false, "update compatibility observations; review contract changes")

type observation struct {
	Exit  int      `json:"exit"`
	Paths []string `json:"paths"`
}

type comparison struct {
	Classification string      `json:"classification"`
	HTTPGo         observation `json:"httpgo"`
	Hurl           observation `json:"hurl"`
}

func TestCompatibility(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	version, err := exec.CommandContext(ctx, "hurl", "--version").Output()
	if err != nil {
		t.Fatalf("Hurl %s is required, without silent skip: %v", hurlVersion, err)
	}
	fields := strings.Fields(string(version))
	if len(fields) < 2 || fields[0] != "hurl" || fields[1] != hurlVersion {
		t.Fatalf("require Hurl %s, got %q", hurlVersion, version)
	}
	files, err := filepath.Glob("../../testdata/compatibility/*.hurl")
	if err != nil || len(files) == 0 {
		t.Fatalf("corpus=%v,err=%v", files, err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			input, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			expectedPath := strings.TrimSuffix(file, ".hurl") + ".json"
			data, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatal(err)
			}
			var expected comparison
			if err := json.Unmarshal(data, &expected); err != nil {
				t.Fatal(err)
			}
			if expected.Classification == "" {
				t.Fatal("missing explicit difference classification")
			}
			var mu sync.Mutex
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				paths = append(paths, r.URL.Path)
				mu.Unlock()
				switch r.URL.Path {
				case "/health":
					w.WriteHeader(200)
				case "/missing":
					w.WriteHeader(404)
				case "/error":
					w.WriteHeader(500)
				case "/unavailable":
					w.WriteHeader(503)
				case "/boundary":
					w.WriteHeader(599)
				case "/redirect":
					w.Header().Set("Location", "/health")
					w.WriteHeader(302)
				case "/cross":
					w.Header().Set("Location", "http://localhost:"+strings.Split(r.Host, ":")[1]+"/health")
					w.WriteHeader(302)
				default:
					w.WriteHeader(200)
				}
			}))
			defer server.Close()
			generated := scenario(t, strings.ReplaceAll(string(input), "http://example.invalid", server.URL))
			code, out, stderr := run(t, "test", generated)
			mu.Lock()
			httpgoPaths := slices.Clone(paths)
			paths = nil
			mu.Unlock()
			hurlContext, hurlCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer hurlCancel()
			command := exec.CommandContext(hurlContext, "hurl", "--no-color", "--no-output", "--location", "--max-redirs", "10", "--max-time", "10", "--retry", "0", "--no-cookie-store", "--no-proxy", "*", generated)
			hurlOutput, hurlErr := command.CombinedOutput()
			hurlCode := 0
			if hurlContext.Err() != nil {
				t.Fatal(hurlContext.Err())
			}
			if hurlErr != nil {
				if command.ProcessState == nil {
					t.Fatal(hurlErr)
				}
				hurlCode = command.ProcessState.ExitCode()
			}
			mu.Lock()
			hurlPaths := slices.Clone(paths)
			mu.Unlock()
			got := comparison{Classification: expected.Classification, HTTPGo: observation{code, httpgoPaths}, Hurl: observation{hurlCode, hurlPaths}}
			if *updateCompatibility {
				encoded, err := json.MarshalIndent(got, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(expectedPath, append(encoded, '\n'), 0644); err != nil {
					t.Fatal(err)
				}
				t.Logf("updated %s; review compatibility contract change", expectedPath)
				return
			}
			if got.HTTPGo.Exit != expected.HTTPGo.Exit || !slices.Equal(got.HTTPGo.Paths, expected.HTTPGo.Paths) || got.Hurl.Exit != expected.Hurl.Exit || !slices.Equal(got.Hurl.Paths, expected.Hurl.Paths) {
				t.Fatalf("observed=%+v,expected=%+v\nhttpgo out=%q,err=%q\nhurl=%q", got, expected, out, stderr, hurlOutput)
			}
			t.Logf("%s: httpgo=%s %v; Hurl %s=%s %v", got.Classification, strconv.Itoa(code), httpgoPaths, hurlVersion, strconv.Itoa(hurlCode), hurlPaths)
		})
	}
}
