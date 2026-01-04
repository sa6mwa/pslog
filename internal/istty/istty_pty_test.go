//go:build linux || darwin || dragonfly || freebsd || netbsd || openbsd || solaris || zos

package istty

import (
	"testing"

	"github.com/creack/pty"
)

func TestIsTerminal_PTY(t *testing.T) {
	_, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("pty open: %v", err)
	}
	t.Cleanup(func() { _ = tty.Close() })

	if !IsTerminal(int(tty.Fd())) {
		t.Fatalf("expected pty slave to be a terminal")
	}
}
