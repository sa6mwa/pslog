package pslog

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestLoggerFatalAndPanicInternal(t *testing.T) {
	variants := []struct {
		name string
		opts Options
	}{
		{"structured", Options{Mode: ModeStructured, DisableTimestamp: true, NoColor: true}},
		{"jsoncolor", Options{Mode: ModeStructured, DisableTimestamp: true, ForceColor: true}},
		{"console_plain", Options{Mode: ModeConsole, DisableTimestamp: true, NoColor: true}},
		{"console_color", Options{Mode: ModeConsole, DisableTimestamp: true, ForceColor: true}},
	}

	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := NewWithOptions(nil, &buf, variant.opts)

			called := false
			origExit := exitProcess
			exitProcess = func() { called = true }
			t.Cleanup(func() { exitProcess = origExit })

			logger.Fatal("fatal", "err", errors.New("fatal"))
			if !called {
				t.Fatalf("exitProcess not called")
			}
			if !strings.Contains(buf.String(), "fatal") {
				t.Fatalf("fatal message not logged: %q", buf.String())
			}

			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("expected panic")
				}
			}()
			logger.Panic("panic", "err", errors.New("panic"))
		})
	}
}
