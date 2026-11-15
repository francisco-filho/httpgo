package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func writeConfig(t *testing.T, text string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "project.yaml")
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestConfigTypedRequest(t *testing.T) {
	path := writeConfig(t, `version: 1
variables:
  api: {baseUrl: http://example.test}
  product: {price: 4500, code: "001", items: [9007199254740993, null, true]}
`)
	var request runner.Request
	calls := 0
	fake := executorFunc(func(_ context.Context, r runner.Request) (runner.Response, error) {
		request = r
		calls++
		return runner.Response{Status: 201}, nil
	})
	file := writeScenario(t, "POST {{api.baseUrl}}/products\n{\"price\":{{product.price}},\"code\":{{product.code}},\"items\":{{product.items}}}\nHTTP 201")
	code, out, stderr := execute(t, context.Background(), fake, "test", file, "--config", path)
	if code != 0 || stderr != "" || calls != 1 {
		t.Fatalf("code=%d out=%q err=%q calls=%d", code, out, stderr, calls)
	}
	if request.URL != "http://example.test/products" || string(request.Body) != `{"price":4500,"code":"001","items":[9007199254740993,null,true]}` {
		t.Fatalf("request=%+v", request)
	}
}

func TestConfigInvalidBeforeHTTP(t *testing.T) {
	file := writeScenario(t, "GET http://example.test/first\nGET http://example.test/last")
	for _, source := range []string{"version: 1\nvariables: {a: '${TOKEN}'}", "version: 1\n---\nversion: 1", "version: 2", "version: 1\nvariables: {a: &x hidden-fixture}", "version: 1\nunknown: hidden-fixture", "version: ["} {
		fake := &fakeExecutor{}
		path := writeConfig(t, source)
		code, out, stderr := execute(t, context.Background(), fake, "test", file, "--config="+path)
		if code != 2 || out != "" || len(fake.calls) != 0 || !strings.Contains(stderr, path) || strings.Contains(stderr, "hidden-fixture") {
			t.Fatalf("source=%q code=%d out=%q err=%q", source, code, out, stderr)
		}
	}
	for _, flags := range [][]string{{"--config"}, {"--config="}, {"--config", filepath.Join(t.TempDir(), "missing")}, {"--config", t.TempDir()}, {"--config", writeConfig(t, ""), "--config", writeConfig(t, "")}} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, append([]string{"test", file}, flags...)...)
		if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 {
			t.Fatalf("flags=%v code=%d out=%q err=%q", flags, code, out, stderr)
		}
	}
}

func TestConfigHierarchyNotAnObject(t *testing.T) {
	fake := &fakeExecutor{}
	code, _, stderr := execute(t, context.Background(), fake, "test", writeScenario(t, "POST http://example.test/\n{{api}}"), "--config", writeConfig(t, "version: 1\nvariables: {api: {baseUrl: http://example.test}}"))
	if code != 2 || len(fake.calls) != 0 || !strings.Contains(stderr, "variável ausente") {
		t.Fatalf("code=%d err=%q", code, stderr)
	}
}
