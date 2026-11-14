//go:build linux

package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// A real Linux pseudoterminal, without a runtime or test dependency on C.
func TestPlainOutputOnRealTTY(t *testing.T) {
	for _, noColor := range []string{"", "1"} {
		t.Run("NO_COLOR="+noColor, func(t *testing.T) {
			t.Setenv("NO_COLOR", noColor)
			master, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = master.Close() }()
			var value uint32
			if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), 0x40045431, uintptr(unsafe.Pointer(&value))); errno != 0 {
				t.Fatal(errno)
			} // TIOCSPTLCK: unlock
			if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, master.Fd(), 0x80045430, uintptr(unsafe.Pointer(&value))); errno != 0 {
				t.Fatal(errno)
			} // TIOCGPTN: slave number
			slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", value), os.O_RDWR, 0600)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = slave.Close() }()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary(t), "--help")
			var stderr bytes.Buffer
			command.Stdout = slave
			command.Stderr = &stderr
			if err := command.Run(); err != nil {
				t.Fatal(err)
			}
			if err := slave.Close(); err != nil {
				t.Fatal(err)
			}
			output, err := io.ReadAll(master)
			if err != nil && !errors.Is(err, syscall.EIO) {
				t.Fatal(err)
			}
			if stderr.Len() != 0 || !strings.Contains(string(output), "httpgo test health.hurl") || bytes.Contains(output, []byte("\x1b")) {
				t.Fatalf("stdout=%q,stderr=%q", output, stderr.String())
			}
		})
	}
}
