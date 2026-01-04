//go:build plan9

package istty

import (
	"os"
	"testing"
)

func TestIsTerminal_Plan9Console(t *testing.T) {
	paths := []string{"/dev/cons", "/mnt/term/dev/cons"}
	var f *os.File
	var err error
	for _, path := range paths {
		f, err = os.Open(path)
		if err == nil {
			break
		}
	}
	if f == nil {
		t.Skipf("no console device available: last error %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if !IsTerminal(int(f.Fd())) {
		t.Fatalf("expected %s to be a terminal", f.Name())
	}
}
