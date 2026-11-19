package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These checks guard the publication boundary, not GitHub service behavior.
// Native and public HTTPS evidence is produced only by the release workflow.
func TestReleaseWorkflowGuards(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(rootDir(t), ".github/workflows/release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, required := range []string{
		"tags: ['v*']", "permissions:\n  contents: read", "go-version-file: go.mod",
		"actions/setup-go@v6", "scripts/package-release.sh --validate-tag", "make fmt lint test", "go vet ./...", "make build e2e",
		"runner: ubuntu-24.04\n", "runner: ubuntu-24.04-arm\n", "runner: macos-15-intel\n", "runner: macos-15\n",
		"test \"$(uname -s)\" = \"$EXPECTED_SYSTEM\"", "test \"$(uname -m)\" = \"$EXPECTED_MACHINE\"",
		"go test ./tests/release -count=1 -v", "HTTPGO_RELEASE_ASSETS:", "HTTPGO_RELEASE_TAG:",
		"github.event_name == 'push' && github.ref_type == 'tag' && github.repository == 'francisco-filho/httpgo'",
		"base=https://github.com/francisco-filho/httpgo/releases",
		"needs: [package, native]", "sha256sum --check --strict checksums.txt",
		"softprops/action-gh-release@v2", "draft: true", "fail_on_unmatched_files: true", "make_latest: 'false'",
		"prerelease: ${{ needs.package.outputs.kind == 'prerelease' }}", "-F draft=false -F prerelease=\"$prerelease\" -f make_latest=\"$latest\"",
		"needs: [package, publish]", "releases", "/latest/download/install.sh\" | /bin/bash", "--version \"$TAG\" --bin-dir",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("workflow missing guard %q", required)
		}
	}
	publish := strings.Index(workflow, "  publish:\n")
	draft := strings.Index(workflow, "      - name: Upload all assets to a draft")
	finalize := strings.Index(workflow, "      - name: Publish only after successful upload")
	smoke := strings.Index(workflow, "  public-smoke:\n")
	if publish < 0 || draft < publish || finalize < draft || smoke < finalize {
		t.Fatalf("unsafe publication ordering")
	}
	if strings.Count(workflow, "contents: write") != 1 || !strings.Contains(workflow[publish:draft], "contents: write") {
		t.Fatal("write permission must be exclusive to publisher")
	}
	if strings.Contains(workflow, "continue-on-error:") || strings.Contains(workflow, "always()") {
		t.Fatal("release must not bypass failed gates")
	}
	// Syntax-check all embedded shell programs too, without executing a release.
	lines := strings.Split(workflow, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "run: |" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		var shell strings.Builder
		for _, body := range lines[i+1:] {
			if strings.TrimSpace(body) != "" && len(body)-len(strings.TrimLeft(body, " ")) <= indent {
				break
			}
			shell.WriteString(strings.TrimPrefix(body, strings.Repeat(" ", indent+2)))
			shell.WriteByte('\n')
		}
		cmd := process(t, "/bin/bash", "-n")
		cmd.Stdin = strings.NewReader(shell.String())
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("shell at workflow line %d: %v %s", i+1, err, out)
		}
	}
}

func TestPackagePreparationFailures(t *testing.T) {
	for _, tool := range []string{"go", "tar", "sha256sum"} {
		t.Run(tool, func(t *testing.T) {
			tools := t.TempDir()
			put(t, filepath.Join(tools, tool), []byte("#!/bin/bash\nprintf 'injected preparation failure\\n' >&2\nexit 1\n"), 0700)
			output := filepath.Join(t.TempDir(), "assets")
			cmd := process(t, "/bin/bash", filepath.Join(rootDir(t), "scripts/package-release.sh"), "v1.2.3", output)
			cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "injected preparation failure") || strings.Contains(string(out), "prepared ") {
				t.Fatalf("preparation: %v %s", err, out)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("partial assets exposed: %v", err)
			}
		})
	}
}
