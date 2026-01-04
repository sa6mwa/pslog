//go:build aix

package istty

import (
	"os"
	"testing"
)

func TestIsTerminal_AIXTTY(t *testing.T) {
	paths := []string{"/dev/tty", "/dev/console"}
	var f *os.File
	var err error
	for _, path := range paths {
		f, err = os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			break
		}
	}
	if f == nil {
		t.Skipf("no tty device available: last error %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if !IsTerminal(int(f.Fd())) {
		t.Fatalf("expected %s to be a terminal", f.Name())
	}
}
