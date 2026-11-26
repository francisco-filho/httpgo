package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var e2eFunctionUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// GO-586: the documented demonstration runs against a real HTTP server.
func TestFunctionDemonstrationByBinary(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || r.Method != "POST" || r.URL.Path != "/products" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("request=%v err=%v", r, err)
		}
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.WriteHeader(201)
	}))
	defer server.Close()
	file := scenario(t, "POST {{api.baseUrl}}/products\nContent-Type: application/json\n{\"requestId\":\"{{newUuid}}\",\"createdAt\":\"{{newDate}}\",\"label\":\"pedido-{{newUuid}}\"}\nHTTP 201")
	for execution := 0; execution < 2; execution++ {
		code, out, stderr := run(t, "test", file, "--var", "api.baseUrl="+server.URL)
		if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 2 {
		t.Fatalf("bodies=%q", bodies)
	}
	var seen []string
	for _, body := range bodies {
		var decoded struct {
			RequestID string `json:"requestId"`
			CreatedAt string `json:"createdAt"`
			Label     string `json:"label"`
		}
		if err := json.Unmarshal([]byte(body), &decoded); err != nil {
			t.Fatalf("body=%s err=%v", body, err)
		}
		if !e2eFunctionUUID.MatchString(decoded.RequestID) {
			t.Fatalf("requestId=%q", decoded.RequestID)
		}
		if _, err := time.Parse(time.RFC3339, decoded.CreatedAt); err != nil {
			t.Fatalf("createdAt=%q err=%v", decoded.CreatedAt, err)
		}
		label := strings.TrimPrefix(decoded.Label, "pedido-")
		if !e2eFunctionUUID.MatchString(label) || label == decoded.RequestID {
			t.Fatalf("label=%q requestId=%q", decoded.Label, decoded.RequestID)
		}
		seen = append(seen, decoded.RequestID, label)
	}
	for i := range seen {
		for j := i + 1; j < len(seen); j++ {
			if seen[i] == seen[j] {
				t.Fatalf("repeated generated value: %v", seen)
			}
		}
	}
}

// GO-587: a repeated execution produces new values in URL and header.
func TestFunctionValuesChangeBetweenExecutionsByBinary(t *testing.T) {
	var mu sync.Mutex
	var urls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		urls = append(urls, r.URL.String())
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()
	file := scenario(t, "GET "+server.URL+"/items/{{newUuid}}\nX-When: {{newDate}}\nHTTP 200")
	for execution := 0; execution < 2; execution++ {
		if code, out, stderr := run(t, "test", file); code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
			t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(urls) != 2 || urls[0] == urls[1] {
		t.Fatalf("urls=%v", urls)
	}
	for _, raw := range urls {
		path := strings.TrimPrefix(raw, "/items/")
		if !e2eFunctionUUID.MatchString(path) {
			t.Fatalf("url=%q", raw)
		}
	}
}

// GO-592 / GO-594: reserved names and invalid final URLs fail with code 2 and no request.
func TestFunctionsRejectionsByBinary(t *testing.T) {
	var hits int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(200)
	}))
	defer server.Close()
	for _, tc := range []struct {
		name, text, wantDetail string
		wantHits               int
	}{
		{"capture", "GET " + server.URL + "/first\nHTTP 200\nGET " + server.URL + "/second\nHTTP 200\n[Captures]\nnewUuid: jsonpath \"$.id\"\n", "nome reservado para função embutida (newDate, newUuid)", 0},
		{"option variable", "GET " + server.URL + "/first\n[Options]\nvariable: newDate=literal\nHTTP 200\n", "nome reservado para função embutida (newDate, newUuid)", 0},
		{"relative url", "GET " + server.URL + "/first\nHTTP 200\nGET {{newUuid}}\nHTTP 200\nGET " + server.URL + "/last\nHTTP 200\n", "erro de renderização", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hits = 0
			file := scenario(t, tc.text)
			code, _, stderr := run(t, "test", file)
			if code != 2 || hits != tc.wantHits || !strings.Contains(stderr, tc.wantDetail) {
				t.Fatalf("code=%d hits=%d stderr=%q", code, hits, stderr)
			}
			if strings.Contains(stderr, "00000000-0000") {
				t.Fatalf("unsafe diagnostic: %q", stderr)
			}
		})
	}
	hits = 0
	file := scenario(t, "GET "+server.URL+"/health\nHTTP 200")
	if code, _, stderr := run(t, "test", file, "--var", "newDate=2025-01-01T00:00:00Z"); code != 2 || hits != 0 || !strings.Contains(stderr, "nome reservado para função embutida (newDate, newUuid)") {
		t.Fatalf("code=%d hits=%d stderr=%q", code, hits, stderr)
	}
}

// GO-596: scenarios without functions keep their previous behavior.
func TestFunctionsStaticScenarioUnchangedByBinary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	defer server.Close()
	file := scenario(t, "GET "+server.URL+"/health\nHTTP 200")
	code, out, stderr := run(t, "test", file)
	if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") {
		t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
	}
}
