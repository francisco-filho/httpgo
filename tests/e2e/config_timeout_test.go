package e2e

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfigPublicFractionalTimeout(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/start" {
			w.Header().Set("Location", "/blocked")
			w.WriteHeader(302)
			return
		}
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	dir := t.TempDir()
	configFile(t, dir, ".httpgo.yaml", "version: 1\ndefaults: {timeout: 0.125}")
	file := configFile(t, dir, "timeout.hurl", "GET "+server.URL+"/start\nHTTP 200\nGET "+server.URL+"/never\nHTTP 200")
	code, out, stderr := runConfig(t, dir, "test", file)
	if code != 2 || hits.Load() != 2 || !strings.Contains(stderr, "limite total de 0.125 segundos") || !strings.Contains(out, "iniciadas=1 concluídas=0") || !strings.Contains(out, "não_iniciadas=1") {
		t.Fatalf("code=%d out=%q err=%q hits=%d", code, out, stderr, hits.Load())
	}
}

func TestConfigPublicTimeoutAboveTenSeconds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timer := time.NewTimer(10100 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-timer.C:
			w.WriteHeader(200)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	configFile(t, dir, ".httpgo.yaml", "version: 1\ndefaults: {timeout: 15}")
	file := configFile(t, dir, "slow.hurl", "GET "+server.URL+"\nHTTP 200")
	started := time.Now()
	code, out, stderr := runConfig(t, dir, "test", file)
	if code != 0 || stderr != "" || !strings.Contains(out, "validadas=1") || time.Since(started) < 10*time.Second {
		t.Fatalf("code=%d out=%q err=%q elapsed=%s", code, out, stderr, time.Since(started))
	}
}
