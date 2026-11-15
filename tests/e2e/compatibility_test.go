//go:build compatibility

package e2e

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const hurlVersion = "8.0.0"

var updateCompatibility = flag.Bool("update", false, "update compatibility observations; review contract changes")

// Multipart observations compare parts, never random boundary or raw framing.
type wirePart struct {
	Name     string `json:"name"`
	Filename string `json:"filename,omitempty"`
	MIME     string `json:"mime,omitempty"`
	Data     []byte `json:"data"`
}

type wireRequest struct {
	Multipart            []wirePart `json:"multipart,omitempty"`
	MultipartHeaderIssue string     `json:"multipart_header_issue,omitempty"`
	URI                  string     `json:"uri,omitempty"`
	Method               string     `json:"method"`
	Body                 string     `json:"body"`
	ContentType          string     `json:"content_type"`
	Probe                string     `json:"probe"`
	Authorization        string     `json:"authorization,omitempty"`
}

type observation struct {
	Exit     int           `json:"exit"`
	Paths    []string      `json:"paths"`
	Requests []wireRequest `json:"requests,omitempty"`
}

type wireHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type comparison struct {
	ResponseHeaders []wireHeader `json:"response_headers,omitempty"`
	ResponseJSON    string       `json:"response_json,omitempty"`
	Variables       []string     `json:"variables,omitempty"`
	Classification  string       `json:"classification"`
	HTTPGo          observation  `json:"httpgo"`
	Hurl            observation  `json:"hurl"`
}

// Each executable gets an independent scenario/payload tree. References stay
// relative, exactly as in the corpus, rather than becoming absolute test paths.
func compatibilityScenario(t *testing.T, name, text string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(filepath.Join(dir, "payloads"), os.DirFS("../../testdata/compatibility/payloads")); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return file
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
			var requests []wireRequest
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request := wireRequest{Method: r.Method, ContentType: r.Header.Get("Content-Type"), Probe: r.Header.Get("X-Probe"), Authorization: r.Header.Get("Authorization")}
				_, params, mediaErr := mime.ParseMediaType(request.ContentType)
				baseType, _, _ := strings.Cut(request.ContentType, ";")
				if strings.EqualFold(strings.TrimSpace(baseType), "multipart/form-data") {
					if mediaErr != nil {
						request.MultipartHeaderIssue = "parâmetros Content-Type inválidos ou repetidos"
						// Observe generated framing even when libcurl added a second
						// boundary. Never golden a random raw multipart request body.
						for _, parameter := range strings.Split(request.ContentType, ";")[1:] {
							_, candidate, err := mime.ParseMediaType("multipart/form-data;" + parameter)
							if err == nil && candidate["boundary"] != "" {
								params = candidate
							}
						}
					}
					reader := multipart.NewReader(r.Body, params["boundary"])
					request.ContentType = "multipart/form-data"
					for {
						part, err := reader.NextPart()
						if err == io.EOF {
							break
						}
						if err != nil {
							t.Error(err)
							return
						}
						data, err := io.ReadAll(part)
						if err != nil {
							t.Error(err)
							return
						}
						_, disposition, err := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
						if err != nil {
							t.Error(err)
							return
						}
						request.Multipart = append(request.Multipart, wirePart{Name: part.FormName(), Filename: disposition["filename"], MIME: part.Header.Get("Content-Type"), Data: data})
					}
				} else {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						return
					}
					request.Body = string(body)
				}
				mu.Lock()
				paths = append(paths, r.URL.Path)
				if expected.Variables != nil {
					request.URI = r.RequestURI
				}
				requests = append(requests, request)
				mu.Unlock()
				for _, header := range expected.ResponseHeaders {
					w.Header().Add(header.Name, header.Value)
				}
				switch r.URL.Path {
				case "/capture":
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, expected.ResponseJSON)
				case "/auth/login":
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `{"accessToken":"token-ficticio","items":["","array-token"]}`)
				case "/redirect301", "/redirect302", "/redirect303", "/redirect307", "/redirect308":
					status, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/redirect"))
					w.Header().Set("Location", "/health")
					w.WriteHeader(status)
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
			text := strings.ReplaceAll(string(input), "http://example.invalid", server.URL)
			generated := compatibilityScenario(t, filepath.Base(file), text)
			httpgoArgs := []string{"test", generated}
			var variableArgs []string
			for _, variable := range expected.Variables {
				assignment := strings.ReplaceAll(variable, "http://example.invalid", server.URL)
				httpgoArgs = append(httpgoArgs, "--var", assignment)
				variableArgs = append(variableArgs, "--variable", assignment)
			}
			code, out, stderr := run(t, httpgoArgs...)
			mu.Lock()
			httpgoPaths := slices.Clone(paths)
			httpgoRequests := slices.Clone(requests)
			paths, requests = nil, nil
			mu.Unlock()
			hurlContext, hurlCancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer hurlCancel()
			hurlArgs := []string{"--no-color", "--no-output", "--location", "--max-redirs", "10", "--max-time", "10", "--retry", "0", "--no-cookie-store", "--no-proxy", "*"}
			hurlArgs = append(hurlArgs, variableArgs...)
			hurlArgs = append(hurlArgs, compatibilityScenario(t, filepath.Base(file), text))
			command := exec.CommandContext(hurlContext, "hurl", hurlArgs...)
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
			hurlRequests := slices.Clone(requests)
			mu.Unlock()
			// Legacy pairs assert paths only; US-02 pairs also assert the wire
			// method, headers and exact JSON bytes for both executables.
			if expected.HTTPGo.Requests == nil {
				httpgoRequests = nil
			}
			if expected.Hurl.Requests == nil {
				hurlRequests = nil
			}
			got := comparison{ResponseHeaders: expected.ResponseHeaders, ResponseJSON: expected.ResponseJSON, Variables: expected.Variables, Classification: expected.Classification, HTTPGo: observation{Exit: code, Paths: httpgoPaths, Requests: httpgoRequests}, Hurl: observation{Exit: hurlCode, Paths: hurlPaths, Requests: hurlRequests}}
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
			if got.HTTPGo.Exit != expected.HTTPGo.Exit || !slices.Equal(got.HTTPGo.Paths, expected.HTTPGo.Paths) || got.Hurl.Exit != expected.Hurl.Exit || !slices.Equal(got.Hurl.Paths, expected.Hurl.Paths) || !reflect.DeepEqual(got.HTTPGo.Requests, expected.HTTPGo.Requests) || !reflect.DeepEqual(got.Hurl.Requests, expected.Hurl.Requests) {
				t.Fatalf("observed=%+v,expected=%+v\nhttpgo out=%q,err=%q\nhurl=%q", got, expected, out, stderr, hurlOutput)
			}
			t.Logf("%s: httpgo=%s %v; Hurl %s=%s %v", got.Classification, strconv.Itoa(code), httpgoPaths, hurlVersion, strconv.Itoa(hurlCode), hurlPaths)
		})
	}
}
