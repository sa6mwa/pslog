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
	palette := overrideANSIForTest()

	now := time.Unix(1_698_000_000, 0).UTC()

	structured := captureLogOutput(t, ModeStructured, now, palette)
	assertContains(t, structured, "[KEY]\"ts\"")
	assertContains(t, structured, "[TIMESTAMP]")
	assertContains(t, structured, "[MSGKEY]\"msg\"")
	assertContains(t, structured, "[MSG]\"hello\"")
	assertContains(t, structured, "[STR]\"alice\"")
	assertContains(t, structured, "[ERR]")
	assertContains(t, structured, "[NUM]42")
	assertContains(t, structured, "[NUM]3.14")
	assertContains(t, structured, "[BOOL]true")

	console := captureLogOutput(t, ModeConsole, now, palette)
	assertContains(t, console, "[TIMESTAMP]")
	assertContains(t, console, "[MSG]hello")
	assertContains(t, console, "[ERR]")
	assertContains(t, console, "[STR]alice")
	assertContains(t, console, "[KEY]ts_iso=")
	assertContains(t, console, "[NUM]42")
	assertContains(t, console, "[NUM]3.14")
	assertContains(t, console, "[BOOL]true")
}

func captureLogOutput(t *testing.T, mode Mode, now time.Time, palette ansi.Palette) string {
	t.Helper()

	buf := &bytes.Buffer{}
	logger := NewWithOptions(buf, Options{
		Mode:       mode,
		ForceColor: true,
		TimeFormat: time.RFC3339Nano,
		Palette:    &palette,
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

func overrideANSIForTest() ansi.Palette {
	return ansi.Palette{
		Key:        "[KEY]",
		String:     "[STR]",
		Num:        "[NUM]",
		Bool:       "[BOOL]",
		Nil:        "[NIL]",
		Trace:      "[TRC]",
		Debug:      "[DBG]",
		Info:       "[INF]",
		Warn:       "[WRN]",
		Error:      "[ERR]",
		Fatal:      "[FTL]",
		Panic:      "[PNC]",
		NoLevel:    "[NOL]",
		Timestamp:  "[TIMESTAMP]",
		MessageKey: "[MSGKEY]",
		Message:    "[MSG]",
	}
}
