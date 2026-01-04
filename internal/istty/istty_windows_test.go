//go:build windows

package istty

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestIsTerminal_Console(t *testing.T) {
	f, cleanup, err := openConsoleOut()
	if err != nil {
		if isConsoleUnavailable(err) {
			t.Skipf("console unavailable: %v", err)
		}
		t.Fatalf("open console: %v", err)
	}
	t.Cleanup(cleanup)

	if !IsTerminal(int(f.Fd())) {
		t.Fatalf("expected console handle to be a terminal")
	}
}

func openConsoleOut() (*os.File, func(), error) {
	f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err == nil {
		return f, func() { _ = f.Close() }, nil
	}

	allocated, allocErr := allocConsole()
	if allocErr != nil {
		return nil, nil, allocErr
	}

	f, err = os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		if allocated {
			_ = freeConsole()
		}
		return nil, nil, err
	}

	return f, func() {
		_ = f.Close()
		if allocated {
			_ = freeConsole()
		}
	}, nil
}

func allocConsole() (bool, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("AllocConsole")
	r1, _, err := proc.Call()
	if r1 == 0 {
		if err == syscall.Errno(0) {
			return false, syscall.EINVAL
		}
		return false, err
	}
	return true, nil
}

func freeConsole() error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("FreeConsole")
	r1, _, err := proc.Call()
	if r1 == 0 {
		if err == syscall.Errno(0) {
			return syscall.EINVAL
		}
		return err
	}
	return nil
}

func isConsoleUnavailable(err error) bool {
	return errors.Is(err, os.ErrPermission) ||
		errors.Is(err, syscall.ERROR_ACCESS_DENIED) ||
		errors.Is(err, syscall.ERROR_FILE_NOT_FOUND)
}
