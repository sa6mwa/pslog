package pslog

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestLoggerFatalAndPanicInternal(t *testing.T) {
	var buf bytes.Buffer
	logger := NewWithOptions(&buf, Options{Mode: ModeStructured, DisableTimestamp: true, NoColor: true})

	called := false
	origExit := exitProcess
	exitProcess = func() { called = true }
	defer func() { exitProcess = origExit }()

	logger.Fatal("fatal", "err", errors.New("fatal"))
	if !called {
		t.Fatalf("exitProcess not called")
	}
	if !strings.Contains(buf.String(), "fatal") {
		t.Fatalf("fatal message not logged")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic")
		}
	}()
	logger.Panic("panic", "err", errors.New("panic"))
}
