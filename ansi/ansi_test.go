package ansi

import "testing"

func TestSetPaletteOverridesValues(t *testing.T) {
	original := Palette{
		Key:        Key,
		String:     String,
		Num:        Num,
		Bool:       Bool,
		Nil:        Nil,
		Trace:      Trace,
		Debug:      Debug,
		Info:       Info,
		Warn:       Warn,
		Error:      Error,
		Fatal:      Fatal,
		Panic:      Panic,
		NoLevel:    NoLevel,
		Timestamp:  Timestamp,
		MessageKey: MessageKey,
		Message:    Message,
	}

	palette := Palette{
		Key:        "KEY",
		String:     "STR",
		Num:        "NUM",
		Bool:       "BOOL",
		Nil:        "NIL",
		Trace:      "TRACE",
		Debug:      "DEBUG",
		Info:       "INFO",
		Warn:       "WARN",
		Error:      "ERROR",
		Fatal:      "FATAL",
		Panic:      "PANIC",
		NoLevel:    "NOLEVEL",
		Timestamp:  "TS",
		MessageKey: "MSGKEY",
		Message:    "MSG",
	}

	SetPalette(palette)

	t.Cleanup(func() {
		SetPalette(original)
	})

	if Key != "KEY" || String != "STR" || Num != "NUM" {
		t.Fatalf("palette not applied: %q %q %q", Key, String, Num)
	}
	if Message != "MSG" || MessageKey != "MSGKEY" {
		t.Fatalf("message palette not applied")
	}
}
