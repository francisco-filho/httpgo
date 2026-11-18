package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type installerHarness struct {
	t                                   *testing.T
	root, home, tools, tmp, assets, log string
	env                                 []string
	payload                             []byte
}

func put(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func commandPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func newInstaller(t *testing.T) *installerHarness {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Bash installer is for Linux and macOS")
	}
	h := &installerHarness{t: t, root: t.TempDir(), payload: []byte("#!/bin/sh\nprintf 'fixture v1.2.3\\n'\n")}
	h.home = filepath.Join(h.root, "home")
	h.tools = filepath.Join(h.root, "tools")
	h.tmp = filepath.Join(h.root, "tmp")
	h.assets = filepath.Join(h.root, "assets")
	h.log = filepath.Join(h.root, "curl.log")
	for _, dir := range []string{h.home, h.tools, h.tmp, h.assets} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"tar", "gzip", "mktemp", "mkdir", "cp", "chmod", "mv", "rm"} {
		h.link(name)
	}
	// Only one verifier is initially exposed; fallback tests expose shasum alone.
	if path, err := exec.LookPath("sha256sum"); err == nil {
		if err := os.Symlink(path, filepath.Join(h.tools, "sha256sum")); err != nil {
			t.Fatal(err)
		}
	} else {
		put(t, filepath.Join(h.tools, "sha256sum"), []byte(fmt.Sprintf("#!/bin/bash\nexec %q -a 256 \"$@\"\n", commandPath(t, "shasum"))), 0700)
	}
	put(t, filepath.Join(h.tools, "uname"), []byte("#!/bin/bash\ncase $1 in -s) printf '%s\\n' \"${FAKE_SYSTEM:-Linux}\";; -m) printf '%s\\n' \"${FAKE_MACHINE:-x86_64}\";; *) exit 1;; esac\n"), 0700)
	cp := commandPath(t, "cp")
	fakeCurl := fmt.Sprintf(`#!/bin/bash
set -eu
[[ $1 == -q ]] || exit 90
shift
[[ $1 == --fail && $2 == --silent && $3 == --show-error && $4 == --location && $5 == --proto && $6 == '=https' && $7 == --proto-redir && $8 == '=https' ]] || exit 91
shift 8
[[ $1 == --output ]] || exit 92
output=$2
shift 2
if [[ ${1:-} == --write-out ]]; then
    [[ $2 == '%%{url_effective}' ]] || exit 93
    shift 2
fi
[[ $# == 1 && $1 == https://github.com/capimgrosso/httpgo/releases/* ]] || exit 94
url=$1
printf '%%s\n' "$url" >> "$CURL_LOG"
[[ -z ${FAIL_DOWNLOAD:-} || $url != *"$FAIL_DOWNLOAD"* ]] || { printf 'network/TLS failure\n' >&2; exit 22; }
if [[ $url == */latest ]]; then
    printf '%%s' "${LATEST_URL:-https://github.com/capimgrosso/httpgo/releases/tag/v1.2.3}"
else
    case $url in
        */download/v1.2.3/*|*/download/v1.2.4-rc.1/*) ;;
        *) printf 'release not found\n' >&2; exit 22 ;;
    esac
    %q "$ASSETS/${url##*/}" "$output"
fi
`, cp)
	put(t, filepath.Join(h.tools, "curl"), []byte(fakeCurl), 0700)
	h.env = []string{"HOME=" + h.home, "PATH=" + h.tools, "TMPDIR=" + h.tmp, "ASSETS=" + h.assets, "CURL_LOG=" + h.log}
	h.archive("httpgo", tar.TypeReg, h.payload)
	return h
}

func (h *installerHarness) link(name string) {
	h.t.Helper()
	if err := os.Symlink(commandPath(h.t, name), filepath.Join(h.tools, name)); err != nil {
		h.t.Fatal(err)
	}
}

func (h *installerHarness) archive(name string, kind byte, payload []byte) {
	h.t.Helper()
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	header := &tar.Header{Name: name, Typeflag: kind, Mode: 0755, Size: int64(len(payload))}
	if kind == tar.TypeSymlink || kind == tar.TypeLink {
		header.Linkname = "../../outside"
		header.Size = 0
		payload = nil
	}
	if err := tw.WriteHeader(header); err != nil {
		h.t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		h.t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		h.t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		h.t.Fatal(err)
	}
	h.assetsBytes(buffer.Bytes())
}

func (h *installerHarness) assetsBytes(data []byte) {
	h.t.Helper()
	var checksums strings.Builder
	for _, system := range []string{"linux", "darwin"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name := fmt.Sprintf("httpgo_%s_%s.tar.gz", system, arch)
			put(h.t, filepath.Join(h.assets, name), data, 0600)
			fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(data), name)
		}
	}
	put(h.t, filepath.Join(h.assets, "checksums.txt"), []byte(checksums.String()), 0600)
}

func (h *installerHarness) run(pipe bool, args ...string) (string, string, error) {
	h.t.Helper()
	script := filepath.Join(rootDir(h.t), "install.sh")
	var cmd *exec.Cmd
	if pipe {
		source, err := os.ReadFile(script)
		if err != nil {
			h.t.Fatal(err)
		}
		cmd = process(h.t, "/bin/bash", append([]string{"-s", "--"}, args...)...)
		cmd.Stdin = bytes.NewReader(source)
	} else {
		cmd = process(h.t, "/bin/bash", append([]string{script}, args...)...)
	}
	cmd.Env = h.env
	cmd.Dir = h.home
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	return out.String(), stderr.String(), err
}

func (h *installerHarness) assertClean() {
	h.t.Helper()
	files, err := os.ReadDir(h.tmp)
	if err != nil {
		h.t.Fatal(err)
	}
	if len(files) != 0 {
		h.t.Errorf("temporary files remain: %v", files)
	}
	matches, err := filepath.Glob(filepath.Join(h.home, ".local/bin/.httpgo.*"))
	if err != nil {
		h.t.Fatal(err)
	}
	if len(matches) != 0 {
		h.t.Errorf("staged files remain: %v", matches)
	}
	for _, shell := range []string{".bashrc", ".bash_profile", ".profile", ".zshrc"} {
		if _, err := os.Stat(filepath.Join(h.home, shell)); !os.IsNotExist(err) {
			h.t.Errorf("shell file changed: %s", shell)
		}
	}
}

func TestInstallerSelection(t *testing.T) {
	for _, tc := range []struct {
		system, machine, asset           string
		explicit, pipe, custom, fallback bool
	}{
		{"Linux", "x86_64", "linux_amd64", false, true, false, false},
		{"Linux", "amd64", "linux_amd64", true, false, true, true},
		{"Linux", "aarch64", "linux_arm64", false, false, false, false},
		{"Linux", "arm64", "linux_arm64", true, true, true, false},
		{"Darwin", "x86_64", "darwin_amd64", false, false, false, false},
		{"Darwin", "arm64", "darwin_arm64", false, true, false, true},
	} {
		t.Run(tc.system+tc.machine, func(t *testing.T) {
			h := newInstaller(t)
			h.env = append(h.env, "FAKE_SYSTEM="+tc.system, "FAKE_MACHINE="+tc.machine)
			if tc.fallback {
				if err := os.Remove(filepath.Join(h.tools, "sha256sum")); err != nil {
					t.Fatal(err)
				}
				if _, err := exec.LookPath("shasum"); err == nil {
					h.link("shasum")
				} else {
					// Exercise the fallback command protocol on Linux without shasum.
					// macOS runners exercise the real shasum implementation.
					script := fmt.Sprintf("#!/bin/bash\n[[ $1 == -a && $2 == 256 ]] || exit 90\nshift 2\nexec %q \"$@\"\n", commandPath(t, "sha256sum"))
					put(t, filepath.Join(h.tools, "shasum"), []byte(script), 0700)
				}
			}
			args := []string{}
			tag := "v1.2.3"
			if tc.explicit {
				tag = "v1.2.4-rc.1"
				args = append(args, "--version", tag)
			}
			bin := filepath.Join(h.home, ".local/bin")
			if tc.custom {
				bin = filepath.Join(h.home, "custom bin")
				args = append(args, "--bin-dir", bin)
			}
			// Reinstallation must work without confirmation, both through pipe and file.
			for range 2 {
				out, stderr, err := h.run(tc.pipe, args...)
				if err != nil || stderr != "" {
					t.Fatalf("install: %v; stdout=%s stderr=%s", err, out, stderr)
				}
				if !strings.Contains(out, tag) || !strings.Contains(out, bin+"/httpgo") || !strings.Contains(out, "PATH") {
					t.Errorf("success output = %s", out)
				}
				path := filepath.Join(bin, "httpgo")
				got, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatal(readErr)
				}
				info, statErr := os.Stat(path)
				if statErr != nil {
					t.Fatal(statErr)
				}
				if !bytes.Equal(got, h.payload) || info.Mode().Perm() != 0755 {
					t.Fatalf("installed bytes/mode = %q/%v", got, info.Mode())
				}
			}
			log, err := os.ReadFile(h.log)
			if err != nil {
				t.Fatal(err)
			}
			latestCount := strings.Count(string(log), "/latest\n")
			if (tc.explicit && latestCount != 0) || (!tc.explicit && latestCount != 2) {
				t.Fatalf("latest resolution count = %d: %s", latestCount, log)
			}
			if !strings.Contains(string(log), "/download/"+tag+"/httpgo_"+tc.asset+".tar.gz") || !strings.Contains(string(log), "/download/"+tag+"/checksums.txt") {
				t.Fatalf("wrong assets/tag: %s", log)
			}
			h.assertClean()
		})
	}
}

func TestInstallerFailuresPreserveExecutable(t *testing.T) {
	for _, tc := range []struct {
		name, diagnostic string
		args             []string
		change           func(*installerHarness)
	}{
		{"unsupported OS", "unsupported operating system", nil, func(h *installerHarness) { h.env = append(h.env, "FAKE_SYSTEM=Windows") }},
		{"unsupported arch", "unsupported architecture", nil, func(h *installerHarness) { h.env = append(h.env, "FAKE_MACHINE=riscv64") }},
		{"missing version", "missing value", []string{"--version"}, nil},
		{"missing directory", "missing value", []string{"--bin-dir"}, nil},
		{"unknown argument", "unknown argument", []string{"--insecure"}, nil},
		{"invalid tag", "invalid version", []string{"--version", "../../x"}, nil},
		{"missing tag", "cannot download", []string{"--version", "v9.9.9"}, nil},
		{"TLS latest", "cannot resolve", nil, func(h *installerHarness) { h.env = append(h.env, "FAIL_DOWNLOAD=latest") }},
		{"interrupted archive", "cannot download", nil, func(h *installerHarness) { h.env = append(h.env, "FAIL_DOWNLOAD=tar.gz") }},
		{"interrupted checksums", "cannot download checksums", nil, func(h *installerHarness) { h.env = append(h.env, "FAIL_DOWNLOAD=checksums.txt") }},
		{"missing package", "cannot download", nil, func(h *installerHarness) {
			if err := os.Remove(filepath.Join(h.assets, "httpgo_linux_amd64.tar.gz")); err != nil {
				h.t.Fatal(err)
			}
		}},
		{"tampered package", "SHA-256 mismatch", nil, func(h *installerHarness) {
			put(h.t, filepath.Join(h.assets, "httpgo_linux_amd64.tar.gz"), []byte("tampered"), 0600)
		}},
		{"missing checksum", "missing checksum", nil, func(h *installerHarness) { put(h.t, filepath.Join(h.assets, "checksums.txt"), []byte(""), 0600) }},
		{"wrong checksum", "SHA-256 mismatch", nil, func(h *installerHarness) {
			put(h.t, filepath.Join(h.assets, "checksums.txt"), []byte(strings.Repeat("0", 64)+"  httpgo_linux_amd64.tar.gz\n"), 0600)
		}},
		{"duplicate checksum", "duplicate checksum", nil, func(h *installerHarness) {
			b, e := os.ReadFile(filepath.Join(h.assets, "checksums.txt"))
			if e != nil {
				h.t.Fatal(e)
			}
			put(h.t, filepath.Join(h.assets, "checksums.txt"), append(b, b...), 0600)
		}},
		{"invalid gzip", "invalid release archive", nil, func(h *installerHarness) { h.assetsBytes([]byte("invalid gzip with valid hash")) }},
		{"missing executable", "unsafe archive", nil, func(h *installerHarness) { h.archive("other", tar.TypeReg, h.payload) }},
		{"unsafe path", "unsafe archive", nil, func(h *installerHarness) { h.archive("../../outside", tar.TypeReg, h.payload) }},
		{"symlink", "regular file", nil, func(h *installerHarness) { h.archive("httpgo", tar.TypeSymlink, nil) }},
		{"hardlink", "regular file", nil, func(h *installerHarness) { h.archive("httpgo", tar.TypeLink, nil) }},
		{"empty executable", "empty", nil, func(h *installerHarness) { h.archive("httpgo", tar.TypeReg, nil) }},
		{"untrusted latest", "official release", nil, func(h *installerHarness) { h.env = append(h.env, "LATEST_URL=https://evil.invalid/tag/v1.2.3") }},
		{"prerelease latest", "prerelease", nil, func(h *installerHarness) {
			h.env = append(h.env, "LATEST_URL=https://github.com/capimgrosso/httpgo/releases/tag/v1.2.4-rc.1")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newInstaller(t)
			bin := filepath.Join(h.home, ".local/bin")
			if err := os.MkdirAll(bin, 0700); err != nil {
				t.Fatal(err)
			}
			old := []byte("#!/bin/sh\nprintf 'old installation\\n'\n")
			destination := filepath.Join(bin, "httpgo")
			put(t, destination, old, 0755)
			if tc.change != nil {
				tc.change(h)
			}
			out, stderr, err := h.run(true, tc.args...)
			if err == nil || out != "" || !strings.Contains(stderr, tc.diagnostic) {
				t.Fatalf("failure: err=%v stdout=%q stderr=%q; want %q", err, out, stderr, tc.diagnostic)
			}
			got, readErr := os.ReadFile(destination)
			if readErr != nil {
				t.Fatal(readErr)
			}
			info, statErr := os.Stat(destination)
			if statErr != nil {
				t.Fatal(statErr)
			}
			if !bytes.Equal(got, old) || info.Mode().Perm() != 0755 {
				t.Fatalf("old executable changed: %q %v", got, info.Mode())
			}
			if _, err := os.Stat(filepath.Join(h.root, "outside")); !os.IsNotExist(err) {
				t.Fatalf("archive escaped: %v", err)
			}
			h.assertClean()
		})
	}
}

func TestInstallerIOFailures(t *testing.T) {
	for _, tool := range []string{"mkdir", "mktemp", "cp", "chmod", "mv"} {
		t.Run(tool, func(t *testing.T) {
			h := newInstaller(t)
			bin := filepath.Join(h.home, ".local/bin")
			if err := os.MkdirAll(bin, 0700); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(bin, "httpgo")
			old := []byte("previous executable")
			put(t, destination, old, 0755)
			if err := os.Remove(filepath.Join(h.tools, tool)); err != nil {
				t.Fatal(err)
			}
			real := commandPath(t, tool)
			// Inject an actual command failure only at destination I/O, not downloads.
			script := fmt.Sprintf("#!/bin/bash\nfor arg do case $arg in *'.local/bin'*) printf 'injected I/O failure\\n' >&2; exit 1;; esac; done\nexec %q \"$@\"\n", real)
			put(t, filepath.Join(h.tools, tool), []byte(script), 0700)
			out, stderr, err := h.run(false)
			if err == nil || out != "" || !strings.Contains(stderr, "cannot") {
				t.Fatalf("I/O failure: %v %q %q", err, out, stderr)
			}
			got, readErr := os.ReadFile(destination)
			if readErr != nil {
				t.Fatal(readErr)
			}
			info, statErr := os.Stat(destination)
			if statErr != nil {
				t.Fatal(statErr)
			}
			if !bytes.Equal(got, old) || info.Mode().Perm() != 0755 {
				t.Fatalf("old installation changed: %q %v", got, info.Mode())
			}
			h.assertClean()
		})
	}
}

func TestInstallerPrerequisites(t *testing.T) {
	for _, tool := range []string{"curl", "tar", "sha256sum", "uname", "mv"} {
		t.Run(tool, func(t *testing.T) {
			h := newInstaller(t)
			if err := os.Remove(filepath.Join(h.tools, tool)); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := h.run(false)
			if err == nil || out != "" || (!strings.Contains(stderr, tool) && !strings.Contains(stderr, "SHA-256")) {
				t.Fatalf("prerequisite: %v %q %q", err, out, stderr)
			}
			if _, err := os.Stat(filepath.Join(h.home, ".local")); !os.IsNotExist(err) {
				t.Errorf("destination modified: %v", err)
			}
			h.assertClean()
		})
	}
}

func TestInstallerNativePackage(t *testing.T) {
	h := newInstaller(t)
	binary := filepath.Join(t.TempDir(), "httpgo")
	cmd := process(t, "go", "build", "-trimpath", "-ldflags", "-X main.version=v1.2.3", "-o", binary, "./cmd/httpgo")
	cmd.Dir = rootDir(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	payload, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	h.archive("httpgo", tar.TypeReg, payload)
	// The fixture's platform aliases select assets; payload here is truly native.
	out, stderr, err := h.run(true)
	if err != nil {
		t.Fatalf("native install: %v %s %s", err, out, stderr)
	}
	version, err := process(t, filepath.Join(h.home, ".local/bin/httpgo"), "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(version), "v1.2.3") {
		t.Fatalf("native version: %v %s", err, version)
	}
	h.assertClean()
}
