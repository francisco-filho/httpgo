// Package release tests the distribution scripts as real processes.
package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func rootDir(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func process(t *testing.T, name string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	return exec.CommandContext(ctx, name, args...)
}

func TestReleaseTags(t *testing.T) {
	for _, tc := range []struct{ tag, want string }{
		{"v1.2.3", "stable"}, {"v0.0.0", "stable"}, {"v1.2.3+build.01", "stable"},
		{"v1.2.3-rc.1", "prerelease"}, {"v1.2.3-0.a-1+build", "prerelease"},
		{"1.2.3", ""}, {"v01.2.3", ""}, {"v1.2", ""}, {"v1.2.3-01", ""},
		{"v1.2.3-rc..1", ""}, {"v1.2.3+", ""}, {"v1.2.3;touch x", ""},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			cmd := process(t, "bash", filepath.Join(rootDir(t), "scripts/package-release.sh"), "--validate-tag", tc.tag)
			out, err := cmd.CombinedOutput()
			if tc.want == "" {
				if err == nil {
					t.Fatalf("invalid tag accepted: %s", out)
				}
				return
			}
			if err != nil || strings.TrimSpace(string(out)) != tc.want {
				t.Fatalf("got %s, %v; want %s", out, err, tc.want)
			}
		})
	}
}

func TestPackageRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release tooling targets Linux and macOS")
	}
	output := filepath.Join(t.TempDir(), "assets")
	tag, commit, date := "v1.2.3-rc.1", strings.Repeat("a", 40), "2026-11-18T00:00:00Z"
	cmd := process(t, "bash", filepath.Join(rootDir(t), "scripts/package-release.sh"), tag, output)
	cmd.Env = append(os.Environ(), "RELEASE_COMMIT="+commit, "RELEASE_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	checksums, err := os.ReadFile(filepath.Join(output, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(checksums)), "\n")
	if len(lines) != 4 {
		t.Fatalf("checksums = %q", checksums)
	}
	for _, osName := range []string{"linux", "darwin"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name := fmt.Sprintf("httpgo_%s_%s.tar.gz", osName, arch)
			data, readErr := os.ReadFile(filepath.Join(output, name))
			if readErr != nil {
				t.Fatal(readErr)
			}
			sum := sha256.Sum256(data)
			if !strings.Contains(string(checksums), hex.EncodeToString(sum[:])+"  "+name+"\n") {
				t.Fatalf("missing hash for %s", name)
			}
			gz, gzipErr := gzip.NewReader(bytes.NewReader(data))
			if gzipErr != nil {
				t.Fatal(gzipErr)
			}
			tr := tar.NewReader(gz)
			header, tarErr := tr.Next()
			if tarErr != nil {
				t.Fatal(tarErr)
			}
			if header.Name != "httpgo" || header.Typeflag != tar.TypeReg || header.Mode&0111 == 0 {
				t.Fatalf("invalid header: %+v", header)
			}
			executable, copyErr := io.ReadAll(tr)
			if copyErr != nil {
				t.Fatal(copyErr)
			}
			if _, nextErr := tr.Next(); nextErr != io.EOF {
				t.Fatalf("extra member: %v", nextErr)
			}
			if closeErr := gz.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			path := filepath.Join(t.TempDir(), "httpgo")
			if writeErr := os.WriteFile(path, executable, 0700); writeErr != nil {
				t.Fatal(writeErr)
			}
			info, infoErr := buildinfo.ReadFile(path)
			if infoErr != nil {
				t.Fatal(infoErr)
			}
			settings := map[string]string{}
			for _, setting := range info.Settings {
				settings[setting.Key] = setting.Value
			}
			for key, want := range map[string]string{"GOOS": osName, "GOARCH": arch, "CGO_ENABLED": "0", "-trimpath": "true"} {
				if settings[key] != want {
					t.Errorf("%s: %s = %q; want %q", name, key, settings[key], want)
				}
			}
			// Read-only inspection of every target; do not mistake this for native execution.
			for _, want := range []string{tag, commit, date} {
				if !bytes.Contains(executable, []byte(want)) {
					t.Errorf("%s lacks metadata %s", name, want)
				}
			}
			if osName == runtime.GOOS && arch == runtime.GOARCH {
				version, runErr := process(t, path, "--version").CombinedOutput()
				if runErr != nil {
					t.Fatalf("native version: %v: %s", runErr, version)
				}
				for _, want := range []string{tag, commit, date} {
					if !strings.Contains(string(version), want) {
						t.Errorf("version = %s; missing %s", version, want)
					}
				}
			}
		}
	}
	if _, statErr := os.Stat(filepath.Join(rootDir(t), "install.sh")); statErr == nil {
		source, readErr := os.ReadFile(filepath.Join(rootDir(t), "install.sh"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		asset, readErr := os.ReadFile(filepath.Join(output, "install.sh"))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !bytes.Equal(source, asset) {
			t.Fatal("installer asset differs from source")
		}
	}
	// Preparation errors must never yield a usable output directory.
	failed := filepath.Join(t.TempDir(), "failed")
	cmd = process(t, "bash", filepath.Join(rootDir(t), "scripts/package-release.sh"), "v1.0.0", failed)
	cmd.Env = append(os.Environ(), "RELEASE_COMMIT=invalid")
	if out, runErr := cmd.CombinedOutput(); runErr == nil {
		t.Fatalf("invalid metadata succeeded: %s", out)
	}
	if _, statErr := os.Stat(failed); !os.IsNotExist(statErr) {
		t.Fatalf("failed output exists: %v", statErr)
	}
}
