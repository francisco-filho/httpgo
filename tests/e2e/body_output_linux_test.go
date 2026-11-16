package e2e

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBodyRealOutputFailure(t *testing.T) {
	testAssertionRealOutputFailure(t, `body contains "expected"`, []string{"expected", "different", "\xff"})
}

func TestMembershipRealOutputFailure(t *testing.T) {
	testAssertionRealOutputFailure(t, `jsonpath "$" not contains "forbidden"`, []string{`[]`, `["forbidden"]`, `invalid`})
}

func TestHeaderMatchesRealOutputFailure(t *testing.T) {
	testAssertionRealOutputFailure(t, `header "Set-Cookie" matches "^session=private-response$"`, []string{"expected", "different", "incomplete"})
}

func testAssertionRealOutputFailure(t *testing.T, predicate string, bodies []string) {
	t.Helper()
	for _, body := range bodies {
		var hits atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			hits.Add(1)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Set-Cookie", "session=private-response")
			if body == "different" {
				w.Header().Set("Set-Cookie", "session=private-other")
			}
			if body == "incomplete" {
				w.Header().Set("Content-Length", "100")
			}
			_, _ = io.WriteString(w, body)
		}))
		file := scenario(t, "GET "+server.URL+"/first\nHTTP *\n[Asserts]\n"+predicate+"\nGET "+server.URL+"/never")
		full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		command := exec.CommandContext(ctx, binary(t), "test", file)
		var stderr bytes.Buffer
		command.Stdout, command.Stderr = full, &stderr
		err = command.Run()
		cancel()
		_ = full.Close()
		server.Close()
		if err == nil || command.ProcessState == nil || command.ProcessState.ExitCode() != 2 || hits.Load() != 1 || !strings.Contains(stderr.String(), "escrever o resultado") || strings.Contains(stderr.String(), "private") {
			t.Fatalf("state=%v err=%v hits=%d stderr=%s", command.ProcessState, err, hits.Load(), stderr.String())
		}
	}
}
