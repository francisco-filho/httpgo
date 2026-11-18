package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerRejectsExtraMembers(t *testing.T) {
	h := newInstaller(t)
	var buffer bytes.Buffer
	gz := gzip.NewWriter(&buffer)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"httpgo", "../outside"} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0755, Size: int64(len(h.payload))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(h.payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	h.assetsBytes(buffer.Bytes())
	out, stderr, err := h.run(true)
	if err == nil || out != "" || !strings.Contains(stderr, "unsafe archive") {
		t.Fatalf("extra members: %v %q %q", err, out, stderr)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".local")); !os.IsNotExist(err) {
		t.Fatalf("destination created: %v", err)
	}
	h.assertClean()
}

func TestInstallerPathAlreadyConfigured(t *testing.T) {
	h := newInstaller(t)
	bin := filepath.Join(h.home, "relative bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	h.env[1] = "PATH=" + h.tools + string(os.PathListSeparator) + bin
	out, stderr, err := h.run(false, "--bin-dir", "relative bin")
	if err != nil || stderr != "" || strings.Contains(out, "Add ") || !strings.Contains(out, bin+"/httpgo") {
		t.Fatalf("PATH: %v %q %q", err, out, stderr)
	}
	h.assertClean()
}

func TestInstallerHelp(t *testing.T) {
	h := newInstaller(t)
	out, stderr, err := h.run(true, "--help")
	if err != nil || stderr != "" || !strings.Contains(out, "--version") || !strings.Contains(out, "--bin-dir") {
		t.Fatalf("help: %v %q %q", err, out, stderr)
	}
	if _, err := os.Stat(h.log); !os.IsNotExist(err) {
		t.Fatalf("help made network request: %v", err)
	}
	h.assertClean()
}
