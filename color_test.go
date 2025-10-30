package pslog

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"pkt.systems/pslog/ansi"
)

func TestColorPaletteRegression(t *testing.T) {
	restore := overrideANSIForTest()
	defer restore()

	now := time.Unix(1_698_000_000, 0).UTC()

	structured := captureLogOutput(t, ModeStructured, now)
	assertContains(t, structured, "[KEY]\"ts\"")
	assertContains(t, structured, "[TIMESTAMP]")
	assertContains(t, structured, "[MSGKEY]\"msg\"")
	assertContains(t, structured, "[MSG]\"hello\"")
	assertContains(t, structured, "[STR]\"alice\"")
	assertContains(t, structured, "[ERR]")
	assertContains(t, structured, "[NUM]42")
	assertContains(t, structured, "[NUM]3.14")
	assertContains(t, structured, "[BOOL]true")

	console := captureLogOutput(t, ModeConsole, now)
	assertContains(t, console, "[TIMESTAMP]")
	assertContains(t, console, "[MSG]hello")
	assertContains(t, console, "[ERR]")
	assertContains(t, console, "[STR]alice")
	assertContains(t, console, "[KEY]ts_iso=")
	assertContains(t, console, "[NUM]42")
	assertContains(t, console, "[NUM]3.14")
	assertContains(t, console, "[BOOL]true")
}

func captureLogOutput(t *testing.T, mode Mode, now time.Time) string {
	t.Helper()

	buf := &bytes.Buffer{}
	logger := NewWithOptions(buf, Options{
		Mode:       mode,
		ForceColor: true,
		TimeFormat: time.RFC3339Nano,
	})

	logger.Info("hello",
		"ts_iso", now,
		"user", "alice",
		"count", 42,
		"pi", 3.14,
		"ok", true,
		"err", errors.New("boom"),
	)

	return buf.String()
}

func assertContains(t *testing.T, output, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("log output missing %q\noutput: %s", want, output)
	}
}

func overrideANSIForTest() func() {
	snap := ansiSnapshot{
		Key:        ansi.Key,
		String:     ansi.String,
		Timestamp:  ansi.Timestamp,
		MessageKey: ansi.MessageKey,
		Message:    ansi.Message,
		Error:      ansi.Error,
		Num:        ansi.Num,
		Bool:       ansi.Bool,
		Nil:        ansi.Nil,
	}

	ansi.Key = "[KEY]"
	ansi.String = "[STR]"
	ansi.Timestamp = "[TIMESTAMP]"
	ansi.MessageKey = "[MSGKEY]"
	ansi.Message = "[MSG]"
	ansi.Error = "[ERR]"
	ansi.Num = "[NUM]"
	ansi.Bool = "[BOOL]"
	ansi.Nil = "[NIL]"

	return func() {
		ansi.Key = snap.Key
		ansi.String = snap.String
		ansi.Timestamp = snap.Timestamp
		ansi.MessageKey = snap.MessageKey
		ansi.Message = snap.Message
		ansi.Error = snap.Error
		ansi.Num = snap.Num
		ansi.Bool = snap.Bool
		ansi.Nil = snap.Nil
	}
}

type ansiSnapshot struct {
	Key        string
	String     string
	Timestamp  string
	MessageKey string
	Message    string
	Error      string
	Num        string
	Bool       string
	Nil        string
}
