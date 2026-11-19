package release

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readmeInstallBlock(t *testing.T, marker string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(rootDir(t), "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, tail, ok := strings.Cut(string(data), marker)
	if !ok {
		t.Fatalf("README marker missing: %q", marker)
	}
	_, tail, ok = strings.Cut(tail, "```sh\n")
	if !ok {
		t.Fatalf("README shell block missing after %q", marker)
	}
	block, _, ok := strings.Cut(tail, "\n```")
	if !ok {
		t.Fatalf("README shell block not closed after %q", marker)
	}
	return block
}

func TestReadmeInstallContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(rootDir(t), "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, installation, ok := strings.Cut(string(data), "## Instalação\n")
	if !ok {
		t.Fatal("installation section missing")
	}
	installation, _, _ = strings.Cut(installation, "\n## Primeira execução")
	for _, want := range []string{
		"Linux e macOS (Darwin)", "amd64", "arm64", "x86_64", "aarch64", "Windows não faz parte",
		"Bash 3.2", "curl", "tar com suporte a gzip", "sha256sum", "shasum",
		"~/.local/bin", "--bin-dir", "--version v1.0.0", "pré-release", "sem fallback", "PATH",
		"não usa sudo nem", "altera configurações do shell", "executa código remoto", "revise seu conteúdo",
		"integridade", "não é uma assinatura de autenticidade", "da mesma release", "sem confirmação",
		"truncar a instalação anterior", "status diferente de zero", "https://github.com/francisco-filho/httpgo/releases",
		"httpgo_linux_amd64.tar.gz", "httpgo_linux_arm64.tar.gz", "httpgo_darwin_amd64.tar.gz", "httpgo_darwin_arm64.tar.gz",
		"checksums.txt", "ainda não\nforam homologadas", "docs/validation.md", "### Compilar a partir do código-fonte",
		"Go 1.26.10", "go build -trimpath -o bin/httpgo ./cmd/httpgo", "bin/httpgo.exe", "dev commit=unknown date=unknown",
	} {
		if !strings.Contains(installation, want) {
			t.Errorf("installation documentation missing %q", want)
		}
	}
}

// Execute the published blocks verbatim, but substitute HTTPS downloads only in
// the harness. The payload is a real native CLI binary; this is not public smoke.
func TestReadmeInstallCommands(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release installer targets Linux and macOS")
	}
	binary := filepath.Join(t.TempDir(), "httpgo")
	cmd := process(t, "go", "build", "-trimpath", "-ldflags", "-X main.version=v1.0.0", "-o", binary, "./cmd/httpgo")
	cmd.Dir = rootDir(t)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build native CLI: %v %s", err, out)
	}
	payload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(rootDir(t), "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, marker, bin string
		explicit, inspect bool
	}{
		{"default", "Para baixar e instalar a última release estável", ".local/bin", false, false},
		{"pinned", "Para fixar uma versão", ".local/bin", true, false},
		{"custom directory", "Para escolher outro diretório", "meus binarios", false, false},
		{"inspected script", "Se preferir baixar e\ninspecionar o script", ".local/bin", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newInstaller(t)
			h.archive("httpgo", tar.TypeReg, payload)
			put(t, filepath.Join(h.assets, "install.sh"), source, 0600)
			if err := os.Symlink("/bin/bash", filepath.Join(h.tools, "bash")); err != nil {
				t.Fatal(err)
			}
			systems := map[string]string{"linux": "Linux", "darwin": "Darwin"}
			h.env = append(h.env, "EXPECTED_TAG=v1.0.0", "LATEST_URL=https://github.com/francisco-filho/httpgo/releases/tag/v1.0.0",
				"FAKE_SYSTEM="+systems[runtime.GOOS], "FAKE_MACHINE="+runtime.GOARCH)
			block := readmeInstallBlock(t, tc.marker)
			cmd := process(t, "/bin/bash", "-e", "-o", "pipefail", "-c", block)
			cmd.Env, cmd.Dir = h.env, h.home
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			if err := cmd.Run(); err != nil || stderr.Len() != 0 {
				t.Fatalf("published command: %v\nstdout=%s\nstderr=%s", err, out.String(), stderr.String())
			}
			destination := filepath.Join(h.home, filepath.FromSlash(tc.bin), "httpgo")
			if !strings.Contains(out.String(), "Installed httpgo v1.0.0 at "+destination) {
				t.Errorf("install message = %q", out.String())
			}
			version, err := process(t, destination, "--version").CombinedOutput()
			if err != nil || !strings.HasPrefix(string(version), "httpgo v1.0.0 commit=") {
				t.Fatalf("installed CLI: %v %s", err, version)
			}
			log, err := os.ReadFile(h.log)
			if err != nil {
				t.Fatal(err)
			}
			wantLatest := 1
			if tc.explicit {
				wantLatest = 0
			}
			if strings.Count(string(log), "/latest\n") != wantLatest || !strings.Contains(string(log), "/download/v1.0.0/checksums.txt\n") {
				t.Fatalf("release selection = %s", log)
			}
			if tc.inspect {
				got, err := os.ReadFile(filepath.Join(h.home, "install.sh"))
				if err != nil || !bytes.Equal(got, source) {
					t.Fatalf("downloaded inspection copy differs: %v", err)
				}
			}
			staged, err := filepath.Glob(filepath.Join(filepath.Dir(destination), ".httpgo.*"))
			if err != nil || len(staged) != 0 {
				t.Fatalf("staging remains: %v %v", staged, err)
			}
			h.assertClean()
		})
	}
}

func TestInstallerLatestPinnedWhenReleaseChanges(t *testing.T) {
	h := newInstaller(t)
	const next = "https://github.com/francisco-filho/httpgo/releases/tag/v1.2.4"
	h.env = append(h.env, "LATEST_NEXT_URL="+next)
	out, stderr, err := h.run(true)
	if err != nil || stderr != "" || !strings.Contains(out, "Installed httpgo v1.2.3") {
		t.Fatalf("install pinned latest: %v %q %q", err, out, stderr)
	}
	log, err := os.ReadFile(h.log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(log), "/latest\n") != 1 || strings.Contains(string(log), "v1.2.4") ||
		!strings.Contains(string(log), "/download/v1.2.3/httpgo_linux_amd64.tar.gz\n") ||
		!strings.Contains(string(log), "/download/v1.2.3/checksums.txt\n") {
		t.Fatalf("release was not pinned: %s", log)
	}
	// Confirm that another resolution really would select the new release.
	cmd := process(t, filepath.Join(h.tools, "curl"), "-q", "--fail", "--silent", "--show-error", "--location",
		"--proto", "=https", "--proto-redir", "=https", "--output", "/dev/null", "--write-out", "%{url_effective}",
		"https://github.com/francisco-filho/httpgo/releases/latest")
	cmd.Env = h.env
	changed, err := cmd.CombinedOutput()
	if err != nil || string(changed) != next {
		t.Fatalf("simulated new publication: %v %q", err, changed)
	}
	h.assertClean()
}
