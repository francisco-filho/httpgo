package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestVerboseRealFlagFormsAndPreflight(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		_, _ = io.WriteString(w, `{"final":true}`)
	}))
	defer server.Close()
	file := scenario(t, "POST "+server.URL+"\n{}\nHTTP 200")
	for _, flags := range [][]string{{"-v"}, {"--verbose"}, {"-vv"}, {"-v", "-v"}, {"--verbose", "--verbose"}, {"-v", "--verbose"}, {"--verbose", "-v"}} {
		code, out, stderr := run(t, append([]string{"test", file}, flags...)...)
		level2 := len(flags) == 2 || flags[0] == "-vv"
		if code != 1 || strings.Contains(out, "POST") || !strings.Contains(out, "Testes executados: 1") || strings.Count(stderr, "[ERRO]") != 1 || strings.Contains(stderr, "[request body]") != level2 || strings.Contains(stderr, "\r") || strings.Contains(stderr, "\x1b") {
			t.Fatalf("flags=%v code=%d out=%q err=%q", flags, code, out, stderr)
		}
	}
	before := calls.Load()
	for _, flags := range [][]string{{"-v", "-vv"}, {"-vv", "--verbose"}, {"--verbose", "--verbose", "--verbose"}, {"-vvv"}, {"--verbose=2"}} {
		if code, out, stderr := run(t, append([]string{"test", file}, flags...)...); code != 2 || out != "" || stderr == "" || calls.Load() != before {
			t.Fatalf("flags=%v code=%d out=%q err=%q calls=%d", flags, code, out, stderr, calls.Load())
		}
	}
	for _, format := range []string{"junit", "html"} {
		for _, verboseFirst := range []bool{false, true} {
			dir := t.TempDir()
			dest := filepath.Join(dir, "report."+format)
			if err := os.WriteFile(dest, []byte("preserve"), 0600); err != nil {
				t.Fatal(err)
			}
			flags := []string{"--output", format, "--output-file", dest}
			if verboseFirst {
				flags = append([]string{"-vv"}, flags...)
			} else {
				flags = append(flags, "-v")
			}
			code, out, stderr := run(t, append([]string{"test", file}, flags...)...)
			data, err := os.ReadFile(dest)
			entries, listErr := os.ReadDir(dir)
			if code != 2 || out != "" || stderr == "" || calls.Load() != before || err != nil || string(data) != "preserve" || listErr != nil || len(entries) != 1 {
				t.Fatalf("format=%s code=%d calls=%d data=%q entries=%v err=%v/%v", format, code, calls.Load(), data, entries, err, listErr)
			}
		}
	}
}

func TestVerboseRealFailureCategoriesAndInterruption(t *testing.T) {
	for _, level := range []string{"-v", "-vv"} {
		for _, mode := range []string{"status", "assertion", "capture", "render", "parse", "transport", "incomplete", "polling pass", "polling exhausted"} {
			t.Run(level+"/"+mode, func(t *testing.T) {
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					n := calls.Add(1)
					w.Header().Set("Content-Type", "application/json")
					if mode == "incomplete" {
						w.Header().Set("Content-Length", "100")
					}
					status := 200
					if mode == "status" || mode == "polling exhausted" || mode == "polling pass" && n == 1 {
						status = 503
					}
					w.WriteHeader(status)
					_, _ = io.WriteString(w, `{"items":["MILK"]}`)
				}))
				defer server.Close()
				url := server.URL
				if mode == "transport" {
					server.Close()
				}
				source := "GET " + url + "\nHTTP 200"
				phase := "status"
				wantCode, wantCalls := 1, int32(1)
				switch mode {
				case "assertion":
					source += "\n[Asserts]\njsonpath \"$.items\" contains \"SOY\""
					phase = "assertion"
				case "capture":
					source += "\n[Captures]\nx: jsonpath \"$.absent\""
					phase = "captures"
					wantCode = 2
				case "render":
					source = "GET " + url + "/{{missing}}"
					phase = "renderização"
					wantCode, wantCalls = 2, 0
				case "parse":
					source += "\n[Unsupported]"
					phase = "parsing"
					wantCode, wantCalls = 2, 0
				case "transport":
					phase = "transporte"
					wantCode, wantCalls = 2, 0
				case "incomplete":
					phase = "transporte"
					wantCode = 2
				case "polling pass", "polling exhausted":
					source = "GET " + url + "\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200"
					wantCalls = 2
					if mode == "polling pass" {
						wantCode = 0
					}
				}
				dir := t.TempDir()
				for _, subdir := range []string{"a", "b"} {
					if err := os.MkdirAll(filepath.Join(dir, subdir), 0700); err != nil {
						t.Fatal(err)
					}
				}
				configFile(t, dir, "a/same.httpgo", source)
				// Never parse this after fail-fast. It has no fabricated successful steps.
				configFile(t, dir, "b/same.hurl", "invalid syntax that must stay unread")
				code, out, stderr := run(t, "test", dir, level, "--fail-fast")
				if mode == "polling pass" {
					wantCode = 2
					phase = "parsing"
				}
				if code != wantCode || calls.Load() != wantCalls || strings.Contains(out, "GET") || strings.Count(stderr, "["+phase+"]") != 1 {
					t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls.Load(), out, stderr)
				}
				if mode != "polling pass" && !strings.Contains(stderr, "same.hurl NÃO EXECUTADO") {
					t.Fatalf("not skipped: %q", stderr)
				}
				if mode == "parse" && strings.Contains(stderr, "step ") {
					t.Fatalf("invented step: %q", stderr)
				}
				if strings.HasPrefix(mode, "polling") && strings.Count(stderr, "GET ") != 1 {
					t.Fatalf("transient lines: %q", stderr)
				}
				if level == "-v" && strings.Contains(stderr, "[response body]") {
					t.Fatalf("level1 body: %q", stderr)
				}
				if mode == "incomplete" && level == "-vv" && (!strings.Contains(stderr, "indisponível/incompleto") || strings.Contains(stderr, "MILK")) {
					t.Fatalf("partial exposed: %q", stderr)
				}
			})
		}
	}
}

func TestVerboseRealBodiesFilesRedirectsAndOmissions(t *testing.T) {
	for _, mode := range []string{"inline", "file", "rendered file", "empty file", "non JSON", "invalid JSON", "binary", "multipart", "301", "302", "303", "307", "308", "absent"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			var sent []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				mu.Lock()
				sent = append(sent, r.Method+" "+string(data))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/redirect" {
					w.Header().Set("Location", "/final")
					var code int
					if _, err := fmt.Sscan(mode, &code); err != nil {
						t.Error(err)
					}
					w.WriteHeader(code)
					_, _ = io.WriteString(w, `{"intermediate":"never-show"}`)
					return
				}
				if mode == "non JSON" || mode == "multipart" {
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(503)
					_, _ = io.WriteString(w, "private-plain-text")
					return
				}
				if mode == "invalid JSON" {
					w.WriteHeader(503)
					_, _ = io.WriteString(w, `{"private-incomplete":`)
					return
				}
				if mode == "binary" {
					w.Header().Set("Content-Type", "application/octet-stream")
					w.WriteHeader(503)
					_, _ = w.Write([]byte{0xff, 0, 1})
					return
				}
				w.WriteHeader(503)
				_, _ = io.WriteString(w, `{"final":true}`)
			}))
			defer server.Close()
			dir := t.TempDir()
			payload := `{"sent":"rendered"}`
			source := "POST " + server.URL + "\nContent-Type: application/json\n" + payload + "\nHTTP 200"
			flags := []string{"-vv"}
			switch mode {
			case "file", "rendered file", "empty file", "non JSON", "binary", "invalid JSON":
				if mode == "rendered file" {
					payload = `{"sent":"{{value}}"}`
					flags = append(flags, "--render-files", "--var", "value=rendered")
				}
				if mode == "empty file" {
					payload = ""
				}
				if mode == "non JSON" {
					payload = "private-plain-text"
				}
				if mode == "binary" {
					payload = "private\x00binary"
				}
				if mode == "invalid JSON" {
					payload = `{"private-incomplete":`
				}
				configFile(t, dir, "body.json", payload)
				source = "POST " + server.URL + "\nfile,body.json;\nHTTP 200"
			case "multipart":
				configFile(t, dir, "body.json", payload)
				source = "POST " + server.URL + "\n[MultipartFormData]\nfile: file,body.json;\nHTTP 200"
			case "absent":
				source = "POST " + server.URL + "\nHTTP 200"
			case "301", "302", "303", "307", "308":
				source = strings.Replace(source, server.URL, server.URL+"/redirect", 1)
			}
			file := configFile(t, dir, "one.httpgo", source)
			code, out, stderr := run(t, append([]string{"test", file}, flags...)...)
			if code != 1 || strings.Contains(out, "body") || !strings.Contains(stderr, "[request body]") || !strings.Contains(stderr, "[response body]") || strings.Contains(stderr, "never-show") {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
			mu.Lock()
			defer mu.Unlock()
			switch mode {
			case "301", "302", "303", "absent":
				if !strings.Contains(stderr, "[request body]\n  ausente") || strings.Contains(stderr, `{"sent":`) {
					t.Fatalf("not absent: %q sent=%v", stderr, sent)
				}
			case "empty file", "non JSON", "binary", "multipart", "invalid JSON":
				if !strings.Contains(stderr, "omitido (conteúdo não JSON)") || strings.Contains(stderr, "private") {
					t.Fatalf("not omitted: %q", stderr)
				}
			default:
				if !strings.Contains(stderr, `{"sent":"rendered"}`) || strings.Contains(stderr, "{{value}}") || !strings.Contains(stderr, `{"final":true}`) {
					t.Fatalf("bad bodies: %q sent=%v", stderr, sent)
				}
			}
		})
	}
}

func TestVerboseRealPollingUsesLastSentFile(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(fmt.Sprint(remove), func(t *testing.T) {
			dir := t.TempDir()
			payload := configFile(t, dir, "payload.json", `{"attempt":1}`)
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				data, err := io.ReadAll(r.Body)
				if err != nil || string(data) != fmt.Sprintf(`{"attempt":%d}`, n) {
					t.Errorf("sent=%q err=%v attempt=%d", data, err, n)
				}
				if n == 1 {
					if remove {
						err = os.Remove(payload)
					} else {
						err = os.WriteFile(payload, []byte(`{"attempt":2}`), 0600)
					}
					if err != nil {
						t.Error(err)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(503)
				_, _ = fmt.Fprintf(w, `{"finalAttempt":%d}`, n)
			}))
			defer server.Close()
			file := configFile(t, dir, "one.httpgo", "POST "+server.URL+"\nfile,payload.json;\n[Options]\nretry: 1\nretry-interval: 0\nHTTP 200")
			code, out, stderr := run(t, "test", file, "-vv")
			wantCode, wantCalls := 1, int32(2)
			if remove {
				wantCode, wantCalls = 2, 1
			}
			if code != wantCode || calls.Load() != wantCalls || !strings.Contains(stderr, fmt.Sprintf(`{"attempt":%d}`, wantCalls)) || !strings.Contains(stderr, fmt.Sprintf(`{"finalAttempt":%d}`, wantCalls)) || strings.Count(stderr, "[request body]") != 1 || strings.Contains(out, "body") {
				t.Fatalf("code=%d calls=%d out=%q err=%q", code, calls.Load(), out, stderr)
			}
		})
	}
}

func TestVerboseRealSecretCapturePreviews(t *testing.T) {
	const secret = "fiction-captured-private"
	for _, mode := range []string{"status pending", "partial pending", "duplicate secret pending", "assertion after extraction"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if mode == "status pending" {
					w.WriteHeader(503)
				}
				_, _ = fmt.Fprintf(w, `{"token":%q,"ok":false}`, secret)
			}))
			defer server.Close()
			source := "POST " + server.URL + "?key=private-query\n{}\nHTTP 200\n[Captures]\ntoken: jsonpath \"$.token\""
			flags := []string{"--secret", "token", "-vv"}
			if mode == "partial pending" {
				source += "\nother: jsonpath \"$.absent\""
				flags = append(flags, "--secret", "other")
			}
			if mode == "duplicate secret pending" {
				source += "\ntoken: jsonpath \"$.absent\""
			}
			source += "\n[Asserts]\njsonpath \"$.ok\" == true"
			code, out, stderr := run(t, append([]string{"test", scenario(t, source)}, flags...)...)
			wantCode := 1
			if mode == "partial pending" || mode == "duplicate secret pending" {
				wantCode = 2
			}
			if code != wantCode || strings.Contains(out+stderr, secret) || strings.Contains(out+stderr, "private-query") {
				t.Fatalf("code=%d out=%q err=%q", code, out, stderr)
			}
			if mode == "assertion after extraction" {
				if !strings.Contains(stderr, `"token":[oculto]`) {
					t.Fatalf("safe preview missing: %q", stderr)
				}
			} else if !strings.Contains(stderr, "[response body]\n  omitido por proteção") {
				t.Fatalf("pending exposed: %q", stderr)
			}
		})
	}
}
