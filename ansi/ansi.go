// Package ansi provides the ANSI escape sequences and palette helpers used by
// pslog's colored logger variants. The exported strings can be overridden or
// swapped via SetPalette so callers can apply 16- or 256-colour schemes without
// touching pslog internals.
package ansi

import "sync"

// Reset is the ANSI escape code that clears all terminal styling; the
// remaining constants expose common ANSI color sequences used by pslog.
const (
	Reset         = "\x1b[0m"
	Bold          = "\x1b[1m"
	Faint         = "\x1b[90m"
	Red           = "\x1b[31m"
	Green         = "\x1b[32m"
	Yellow        = "\x1b[33m"
	Blue          = "\x1b[34m"
	Magenta       = "\x1b[35m"
	Cyan          = "\x1b[36m"
	Gray          = "\x1b[37m"
	BrightRed     = "\x1b[1;31m"
	BrightGreen   = "\x1b[1;32m"
	BrightYellow  = "\x1b[1;33m"
	BrightBlue    = "\x1b[1;34m"
	BrightMagenta = "\x1b[1;35m"
	BrightCyan    = "\x1b[1;36m"
	BrightWhite   = "\x1b[1;37m" // same as ansiBold essentially
)

// Semantic aliases that describe how pslog uses the colours. These
var (
	Key        = Cyan
	String     = BrightBlue
	Num        = Magenta
	Bool       = Yellow
	Nil        = Faint
	Trace      = Blue
	Debug      = Green
	Info       = BrightGreen
	Warn       = BrightYellow
	Error      = BrightRed
	Fatal      = BrightRed
	Panic      = BrightRed
	NoLevel    = Faint
	Timestamp  = Faint
	MessageKey = Cyan
	Message    = Bold
)

var paletteMu sync.RWMutex

// Palette is the input type to SetPalette, see the Palette* variables for
// examples.
type Palette struct {
	Key        string
	String     string
	Num        string
	Bool       string
	Nil        string
	Trace      string
	Debug      string
	Info       string
	Warn       string
	Error      string
	Fatal      string
	Panic      string
	NoLevel    string
	Timestamp  string
	MessageKey string
	Message    string
}

// SetPalette sets the package-level ANSI color variables exposed by this
// package. pslog loggers can also use explicit palette selection through
// pslog.Options.Palette (*ansi.Palette) / pslog.NewWithPalette.
//
//	ansi.SetPalette(ansi.PaletteSynthwave84)
//	// Reset to default
//	ansi.SetPalette(ansi.PaletteDefault)
func SetPalette(palette Palette) {
	paletteMu.Lock()
	defer paletteMu.Unlock()

	current := snapshotLocked()
	Key = f(palette.Key, current.Key)
	String = f(palette.String, current.String)
	Num = f(palette.Num, current.Num)
	Bool = f(palette.Bool, current.Bool)
	Nil = f(palette.Nil, current.Nil)
	Trace = f(palette.Trace, current.Trace)
	Debug = f(palette.Debug, current.Debug)
	Info = f(palette.Info, current.Info)
	Warn = f(palette.Warn, current.Warn)
	Error = f(palette.Error, current.Error)
	Fatal = f(palette.Fatal, current.Fatal)
	Panic = f(palette.Panic, current.Panic)
	NoLevel = f(palette.NoLevel, current.NoLevel)
	Timestamp = f(palette.Timestamp, current.Timestamp)
	MessageKey = f(palette.MessageKey, current.MessageKey)
	Message = f(palette.Message, current.Message)
}

// Snapshot returns the current ANSI palette values.
//
// Typical usage in tests:
//
//	snap := ansi.Snapshot()
//	defer ansi.SetPalette(snap)
//	ansi.SetPalette(ansi.PaletteSynthwave84)
//	// run assertions...
func Snapshot() Palette {
	paletteMu.RLock()
	defer paletteMu.RUnlock()
	return snapshotLocked()
}

func snapshotLocked() Palette {
	return Palette{
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
}

func f(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}
