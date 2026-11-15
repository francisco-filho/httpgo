package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func TestProcessEnvironmentProtectionAcrossBodies(t *testing.T) {
	for _, override := range []bool{false, true} {
		t.Run(map[bool]string{false: "resolved", true: "CLI substitute"}[override], func(t *testing.T) {
			t.Setenv("HTTPGO_US13_TOKEN", "process-fiction")
			base := writeConfig(t, "version: 1\nvariables: {token: '${HTTPGO_US13_TOKEN}', marked: explicit-fiction}\nsecrets: [marked, future]")
			value := "process-fiction"
			args := []string{"--config", base, "--secret", "cliMarked", "--var", "cliMarked=cli-marked-fiction", "--render-files"}
			if override {
				value = "substitute-fiction"
				args = append(args, "--var", "token="+value)
			}
			dir := t.TempDir()
			const upload = "\x00\xff{{token}}\r\n"
			for name, data := range map[string]string{"upload.bin": upload, "payload.json": `{"token":"prefix-{{token}}","captured":{{future}}}`} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			header := "\nX-Token: prefix-{{token}}\n"
			text := "GET http://example.test/{{token}}" + header + "HTTP 200\n[Captures]\nfuture: jsonpath \"$.value\"\nPOST http://example.test/inline" + header + `{"token":"prefix-{{token}}","marked":{{marked}},"cli":{{cliMarked}},"capture":{{future}}}` + "\nHTTP 200\nPOST http://example.test/external" + header + "file,payload.json;\nHTTP 200\nPOST http://example.test/multipart" + header + "[MultipartFormData]\ndescription: prefix-{{token}}-{{future}}\nupload: file,upload.bin;\nHTTP 200"
			file := filepath.Join(dir, "scenario.hurl")
			if err := os.WriteFile(file, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			calls := 0
			fake := executorFunc(func(_ context.Context, request runner.Request) (runner.Response, error) {
				calls++
				if request.Headers[0].Value != "prefix-"+value {
					t.Error("header value changed")
				}
				switch calls {
				case 1:
					if request.URL != "http://example.test/"+value {
						t.Error("URL value changed")
					}
				case 2, 3:
					if !strings.Contains(string(request.Body), value) || !strings.Contains(string(request.Body), "captured-fiction") {
						t.Error("body value changed")
					}
				case 4:
					if request.Multipart == nil || request.Multipart.Parts[0].Value != "prefix-"+value+"-captured-fiction" {
						t.Fatal("multipart field changed")
					}
					data, err := io.ReadAll(request.Multipart.Source)
					if err != nil || string(data) != upload {
						t.Fatal("multipart file bytes changed")
					}
				}
				return runner.Response{Status: 200, Body: []byte(`{"value":"captured-fiction"}`)}, nil
			})
			code, out, stderr := execute(t, context.Background(), fake, append([]string{"test", file}, args...)...)
			if code != 0 || calls != 4 || stderr != "" {
				t.Fatalf("code=%d calls=%d", code, calls)
			}
			for _, secret := range []string{value, "explicit-fiction", "cli-marked-fiction", "captured-fiction"} {
				if strings.Contains(out+stderr, secret) {
					t.Fatal("known value leaked")
				}
			}
		})
	}
}
