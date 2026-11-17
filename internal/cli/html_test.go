package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capimgrosso.com/httpgo/internal/runner"
)

func readHTML(t *testing.T, base string) (string, string) {
	t.Helper()
	ext := filepath.Ext(base)
	paths, err := filepath.Glob(strings.TrimSuffix(base, ext) + "-*" + ext)
	if err != nil || len(paths) != 1 {
		t.Fatalf("HTML paths=%v err=%v", paths, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	return paths[0], string(data)
}

func TestHTMLArguments(t *testing.T) {
	for _, args := range [][]string{
		{"test", "input", "--output", "html", "--output-file", "out.html"},
		{"test", "--output-file=out.html", "input", "--output=html"},
	} {
		got, err := parseArguments(args)
		if err != nil || got.output != "html" || got.outputFile != "out.html" {
			t.Fatalf("args=%v got=%+v err=%v", args, got, err)
		}
	}
	for _, flags := range [][]string{
		{"--output=html"}, {"--output=html", "--output-file="},
		{"--output=html", "--output=html", "--output-file=out.html"},
		{"--output=html", "--output=junit", "--output-file=out.html"},
		{"--output=junit", "--output=html", "--output-file=out.html"},
		{"--output=html", "--output-file=out.html", "--output-file=out.html"},
	} {
		fake := &fakeExecutor{}
		code, out, stderr := execute(t, context.Background(), fake, append([]string{"test", "input"}, flags...)...)
		if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 {
			t.Fatalf("flags=%v code=%d out=%q err=%q calls=%v", flags, code, out, stderr, fake.calls)
		}
	}
}

func TestHTMLResultsAndKnownCounts(t *testing.T) {
	for _, mode := range []string{"success", "assertion", "parse", "render", "single", "unvalidated"} {
		for _, fast := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", mode, fast), func(t *testing.T) {
				dir := t.TempDir()
				first := suiteFile(t, dir, "01-ação & \"<script>\".hurl", "GET http://example.test/ok\nHTTP 200")
				middle := "GET http://example.test/middle\nHTTP 200\nGET http://example.test/after\nHTTP 200"
				switch mode {
				case "parse":
					middle += "\nHEAD http://example.test/no"
				case "render":
					middle = "GET http://example.test/{{missing}}\nHTTP 200"
				case "unvalidated":
					middle = "GET http://example.test/middle"
				}
				suiteFile(t, dir, "02.hurl", middle)
				suiteFile(t, dir, "03.hurl", "GET http://example.test/last\nHTTP 200")
				calls := 0
				fake := executorFunc(func(_ context.Context, req runner.Request) (runner.Response, error) {
					calls++
					if mode == "assertion" && strings.HasSuffix(req.URL, "/middle") {
						return runner.Response{Status: 503}, nil
					}
					return runner.Response{Status: 200}, nil
				})
				input := dir
				if mode == "single" {
					input = first
				}
				base := filepath.Join(t.TempDir(), "target", "report.html")
				args := []string{"test", input, "--output=html", "--output-file", base}
				if fast {
					args = append(args, "--fail-fast")
				}
				code, out, stderr := execute(t, context.Background(), fake, args...)
				path, html := readHTML(t, base)
				wantCode, wantCalls := 0, 4
				switch mode {
				case "assertion":
					wantCode, wantCalls = 1, 3
				case "parse", "render":
					wantCode, wantCalls = 2, 2
				case "single":
					wantCalls = 1
				case "unvalidated":
					wantCalls = 3
				}
				if fast && wantCode != 0 {
					wantCalls--
				}
				if code != wantCode || calls != wantCalls || out != "" || !strings.Contains(stderr, fmt.Sprintf("relatório HTML gerado em %q", path)) || strings.Contains(html, "<script>") {
					t.Fatalf("code=%d calls=%d out=%q err=%q HTML=%s", code, calls, out, stderr, html)
				}
				for _, text := range []string{"Resumo agregado", "Resultados por arquivo", "Duração lógica", "GET", "http://example.test/ok", "&lt;script&gt;"} {
					if !strings.Contains(html, text) {
						t.Fatalf("HTML missing %q: %s", text, html)
					}
				}
				if mode == "assertion" && (!strings.Contains(html, "esperado 200, recebido 503") || !strings.Contains(html, "não iniciadas=1")) {
					t.Fatal(html)
				}
				if fast && wantCode != 0 && (!strings.Contains(html, "<strong>não executado</strong>") || !strings.Contains(html, "total=desconhecido")) {
					t.Fatal(html)
				}
				if mode == "parse" && !strings.Contains(html, "total=desconhecido; iniciadas=0") {
					t.Fatal(html)
				}
				if mode == "unvalidated" && !strings.Contains(html, "sem validação de status (recebido 200)") {
					t.Fatal(html)
				}
			})
		}
	}
}

func TestHTMLPreexecutionErrorsDoNotOpenReportOrSend(t *testing.T) {
	for _, mode := range []string{"destination", "config", "discovery", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			file := suiteFile(t, root, "input.hurl", "GET http://example.test\nHTTP 200")
			base := filepath.Join(root, "report.html")
			args := []string{"test", file, "--output=html", "--output-file", base}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch mode {
			case "destination":
				args = args[:3]
			case "config":
				args = append(args, "--config", suiteFile(t, root, "bad.yaml", "version: 999"))
			case "discovery":
				args = append(args, filepath.Join(root, "missing"))
			case "cancelled":
				cancel()
			}
			fake := &fakeExecutor{}
			code, out, stderr := execute(t, ctx, fake, args...)
			paths, err := filepath.Glob(filepath.Join(root, "*.html"))
			if code != 2 || out != "" || stderr == "" || len(fake.calls) != 0 || len(paths) != 0 || err != nil || strings.Contains(stderr, "gerado em") {
				t.Fatalf("code=%d calls=%v out=%q err=%q paths=%v glob=%v", code, fake.calls, out, stderr, paths, err)
			}
		})
	}
}
