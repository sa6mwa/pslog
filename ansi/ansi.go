// Package ansi provides the ANSI escape sequences and palette helpers used by
// pslog's colored logger variants. The exported strings can be overridden or
// swapped via SetPalette so callers can apply 16- or 256-colour schemes without
// touching pslog internals.
package ansi

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

// SetPalette sets the palette for the exported vars that pslog uses to colorize
// json and console log lines when the TTY is a terminal (or ForceColor is
// true). The ansi package ships with a few palettes if you don't want to roll
// your own, for example:
//
//	ansi.SetPalette(ansi.PaletteSynthwave84)
//	logger := pslog.New(os.Stdout).LogLevel(pslog.TraceLevel).WithLogLevel()
//	logger.Info("This will look cool", "promise", true, "num", 1337, "key", "val")
//	// Reset to the default palette
//	ansi.SetPalette(ansi.PaletteDefault)
//	// You can not re-use the same logger when changing palette as it has
//	// already cached trusted keys with colors, you must initialize a new
//	// logger...
//	logger.Debug("Colors will not be what you expect")
//	logger = logger.New(os.Stdout)
//	logger.Debug("This should be back to the default palette")
func SetPalette(palette Palette) {
	Key = f(palette.Key, Key)
	String = f(palette.String, String)
	Num = f(palette.Num, Num)
	Bool = f(palette.Bool, Bool)
	Nil = f(palette.Nil, Nil)
	Trace = f(palette.Trace, Trace)
	Debug = f(palette.Debug, Debug)
	Info = f(palette.Info, Info)
	Warn = f(palette.Warn, Warn)
	Error = f(palette.Error, Error)
	Fatal = f(palette.Fatal, Fatal)
	Panic = f(palette.Panic, Panic)
	NoLevel = f(palette.NoLevel, NoLevel)
	Timestamp = f(palette.Timestamp, Timestamp)
	MessageKey = f(palette.MessageKey, MessageKey)
	Message = f(palette.Message, Message)
}

func f(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

// Palettes...

// PaletteDefault is the default palette in pslog.
var PaletteDefault = Palette{
	Key:        Cyan,
	String:     BrightBlue,
	Num:        Magenta,
	Bool:       Yellow,
	Nil:        Faint,
	Trace:      Blue,
	Debug:      Green,
	Info:       BrightGreen,
	Warn:       BrightYellow,
	Error:      BrightRed,
	Fatal:      BrightRed,
	Panic:      BrightRed,
	NoLevel:    Faint,
	Timestamp:  Faint,
	MessageKey: Cyan,
	Message:    Bold,
}

// PaletteOutrunElectric delivers an outrun electric palette with neon pinks and blues.
var PaletteOutrunElectric = Palette{
	Key:        "\x1b[38;5;201m",   // electric magenta
	String:     "\x1b[38;5;81m",    // neon cyan
	Num:        "\x1b[38;5;99m",    // vibrant purple
	Bool:       "\x1b[38;5;69m",    // teal
	Nil:        "\x1b[38;5;60m",    // muted indigo
	Trace:      "\x1b[38;5;33m",    // cool blue
	Debug:      "\x1b[38;5;39m",    // vibrant azure
	Info:       "\x1b[38;5;45m",    // electric teal
	Warn:       "\x1b[38;5;129m",   // neon purple
	Error:      "\x1b[38;5;205m",   // hot pink near red
	Fatal:      "\x1b[38;5;206m",   // intense magenta
	Panic:      "\x1b[38;5;213m",   // vivid fuchsia
	NoLevel:    "\x1b[38;5;59m",    // dimmed slate
	Timestamp:  "\x1b[38;5;117m",   // bright aqua
	MessageKey: "\x1b[38;5;33m",    // cool blue
	Message:    "\x1b[1;38;5;219m", // bold pastel pink
}

// PaletteDoomIosvkem mirrors doom-emacs' iosvkem theme with dusky oranges and seafoam greens.
var PaletteDoomIosvkem = Palette{
	Key:        "\x1b[38;5;222m",
	String:     "\x1b[38;5;216m",
	Num:        "\x1b[38;5;109m",
	Bool:       "\x1b[38;5;151m",
	Nil:        "\x1b[38;5;244m",
	Trace:      "\x1b[38;5;66m",
	Debug:      "\x1b[38;5;72m",
	Info:       "\x1b[38;5;114m",
	Warn:       "\x1b[38;5;208m",
	Error:      "\x1b[38;5;203m",
	Fatal:      "\x1b[38;5;197m",
	Panic:      "\x1b[38;5;199m",
	NoLevel:    "\x1b[38;5;240m",
	Timestamp:  "\x1b[38;5;242m",
	MessageKey: "\x1b[38;5;67m",
	Message:    "\x1b[1;38;5;223m",
}

// PaletteDoomGruvbox echoes doom-gruvbox colours with earthy reds and ambers.
var PaletteDoomGruvbox = Palette{
	Key:        "\x1b[38;5;214m",
	String:     "\x1b[38;5;178m",
	Num:        "\x1b[38;5;108m",
	Bool:       "\x1b[38;5;142m",
	Nil:        "\x1b[38;5;101m",
	Trace:      "\x1b[38;5;66m",
	Debug:      "\x1b[38;5;72m",
	Info:       "\x1b[38;5;107m",
	Warn:       "\x1b[38;5;208m",
	Error:      "\x1b[38;5;167m",
	Fatal:      "\x1b[38;5;160m",
	Panic:      "\x1b[38;5;161m",
	NoLevel:    "\x1b[38;5;95m",
	Timestamp:  "\x1b[38;5;137m",
	MessageKey: "\x1b[38;5;172m",
	Message:    "\x1b[1;38;5;221m",
}

// PaletteDoomDracula mirrors doom-dracula with pink, purple, and cyan accents.
var PaletteDoomDracula = Palette{
	Key:        "\x1b[38;5;219m",
	String:     "\x1b[38;5;141m",
	Num:        "\x1b[38;5;111m",
	Bool:       "\x1b[38;5;81m",
	Nil:        "\x1b[38;5;240m",
	Trace:      "\x1b[38;5;60m",
	Debug:      "\x1b[38;5;98m",
	Info:       "\x1b[38;5;117m",
	Warn:       "\x1b[38;5;219m",
	Error:      "\x1b[38;5;204m",
	Fatal:      "\x1b[38;5;198m",
	Panic:      "\x1b[38;5;199m",
	NoLevel:    "\x1b[38;5;59m",
	Timestamp:  "\x1b[38;5;95m",
	MessageKey: "\x1b[38;5;147m",
	Message:    "\x1b[1;38;5;225m",
}

// PaletteDoomNord channels doom-nord with cool glacier blues.
var PaletteDoomNord = Palette{
	Key:        "\x1b[38;5;153m",
	String:     "\x1b[38;5;152m",
	Num:        "\x1b[38;5;109m",
	Bool:       "\x1b[38;5;115m",
	Nil:        "\x1b[38;5;245m",
	Trace:      "\x1b[38;5;67m",
	Debug:      "\x1b[38;5;74m",
	Info:       "\x1b[38;5;117m",
	Warn:       "\x1b[38;5;179m",
	Error:      "\x1b[38;5;210m",
	Fatal:      "\x1b[38;5;204m",
	Panic:      "\x1b[38;5;205m",
	NoLevel:    "\x1b[38;5;103m",
	Timestamp:  "\x1b[38;5;109m",
	MessageKey: "\x1b[38;5;110m",
	Message:    "\x1b[1;38;5;195m",
}

// PaletteTokyoNight draws on Tokyo Night's neon blues, violets, and warm highlights.
var PaletteTokyoNight = Palette{
	Key:        "\x1b[38;5;69m",
	String:     "\x1b[38;5;110m",
	Num:        "\x1b[38;5;176m",
	Bool:       "\x1b[38;5;117m",
	Nil:        "\x1b[38;5;244m",
	Trace:      "\x1b[38;5;63m",
	Debug:      "\x1b[38;5;67m",
	Info:       "\x1b[38;5;111m",
	Warn:       "\x1b[38;5;173m",
	Error:      "\x1b[38;5;210m",
	Fatal:      "\x1b[38;5;205m",
	Panic:      "\x1b[38;5;219m",
	NoLevel:    "\x1b[38;5;239m",
	Timestamp:  "\x1b[38;5;109m",
	MessageKey: "\x1b[38;5;74m",
	Message:    "\x1b[1;38;5;218m",
}

// PaletteSolarizedNightfall adapts Solarized Night with teal highlights and amber warnings.
var PaletteSolarizedNightfall = Palette{
	Key:        "\x1b[38;5;37m",
	String:     "\x1b[38;5;86m",
	Num:        "\x1b[38;5;61m",
	Bool:       "\x1b[38;5;65m",
	Nil:        "\x1b[38;5;239m",
	Trace:      "\x1b[38;5;24m",
	Debug:      "\x1b[38;5;30m",
	Info:       "\x1b[38;5;36m",
	Warn:       "\x1b[38;5;136m",
	Error:      "\x1b[38;5;160m",
	Fatal:      "\x1b[38;5;166m",
	Panic:      "\x1b[38;5;161m",
	NoLevel:    "\x1b[38;5;238m",
	Timestamp:  "\x1b[38;5;244m",
	MessageKey: "\x1b[38;5;33m",
	Message:    "\x1b[1;38;5;230m",
}

// PaletteCatppuccinMocha recreates Catppuccin Mocha with soft pastels and rosewater highlights.
var PaletteCatppuccinMocha = Palette{
	Key:        "\x1b[38;5;217m",
	String:     "\x1b[38;5;183m",
	Num:        "\x1b[38;5;147m",
	Bool:       "\x1b[38;5;152m",
	Nil:        "\x1b[38;5;244m",
	Trace:      "\x1b[38;5;104m",
	Debug:      "\x1b[38;5;109m",
	Info:       "\x1b[38;5;150m",
	Warn:       "\x1b[38;5;216m",
	Error:      "\x1b[38;5;211m",
	Fatal:      "\x1b[38;5;205m",
	Panic:      "\x1b[38;5;204m",
	NoLevel:    "\x1b[38;5;240m",
	Timestamp:  "\x1b[38;5;110m",
	MessageKey: "\x1b[38;5;182m",
	Message:    "\x1b[1;38;5;223m",
}

// PaletteGruvboxLight is a Gruvbox light variant with warm browns and turquoise hints.
var PaletteGruvboxLight = Palette{
	Key:        "\x1b[38;5;130m",
	String:     "\x1b[38;5;108m",
	Num:        "\x1b[38;5;66m",
	Bool:       "\x1b[38;5;142m",
	Nil:        "\x1b[38;5;180m",
	Trace:      "\x1b[38;5;109m",
	Debug:      "\x1b[38;5;114m",
	Info:       "\x1b[38;5;73m",
	Warn:       "\x1b[38;5;173m",
	Error:      "\x1b[38;5;167m",
	Fatal:      "\x1b[38;5;161m",
	Panic:      "\x1b[38;5;125m",
	NoLevel:    "\x1b[38;5;181m",
	Timestamp:  "\x1b[38;5;180m",
	MessageKey: "\x1b[38;5;136m",
	Message:    "\x1b[1;38;5;223m",
}

// PaletteMonokaiVibrant supplies a Monokai-inspired mix of neon yellows and minty greens.
var PaletteMonokaiVibrant = Palette{
	Key:        "\x1b[38;5;229m",
	String:     "\x1b[38;5;121m",
	Num:        "\x1b[38;5;198m",
	Bool:       "\x1b[38;5;118m",
	Nil:        "\x1b[38;5;59m",
	Trace:      "\x1b[38;5;104m",
	Debug:      "\x1b[38;5;114m",
	Info:       "\x1b[38;5;121m",
	Warn:       "\x1b[38;5;215m",
	Error:      "\x1b[38;5;197m",
	Fatal:      "\x1b[38;5;161m",
	Panic:      "\x1b[38;5;201m",
	NoLevel:    "\x1b[38;5;240m",
	Timestamp:  "\x1b[38;5;103m",
	MessageKey: "\x1b[38;5;141m",
	Message:    "\x1b[1;38;5;229m",
}

// PaletteOneDarkAurora reflects the One Dark Aurora theme with cyan, violet, and crimson tones.
var PaletteOneDarkAurora = Palette{
	Key:        "\x1b[38;5;110m",
	String:     "\x1b[38;5;147m",
	Num:        "\x1b[38;5;141m",
	Bool:       "\x1b[38;5;115m",
	Nil:        "\x1b[38;5;59m",
	Trace:      "\x1b[38;5;24m",
	Debug:      "\x1b[38;5;31m",
	Info:       "\x1b[38;5;38m",
	Warn:       "\x1b[38;5;178m",
	Error:      "\x1b[38;5;203m",
	Fatal:      "\x1b[38;5;197m",
	Panic:      "\x1b[38;5;199m",
	NoLevel:    "\x1b[38;5;240m",
	Timestamp:  "\x1b[38;5;109m",
	MessageKey: "\x1b[38;5;75m",
	Message:    "\x1b[1;38;5;189m",
}

// PaletteSynthwave84 channels synthwave aesthetics with glowing magentas, cyans, and gold accents.
var PaletteSynthwave84 = Palette{
	Key:        "\x1b[38;5;198m",
	String:     "\x1b[38;5;51m",
	Num:        "\x1b[38;5;207m",
	Bool:       "\x1b[38;5;219m",
	Nil:        "\x1b[38;5;102m",
	Trace:      "\x1b[38;5;63m",
	Debug:      "\x1b[38;5;69m",
	Info:       "\x1b[38;5;81m",
	Warn:       "\x1b[38;5;220m",
	Error:      "\x1b[38;5;205m",
	Fatal:      "\x1b[38;5;200m",
	Panic:      "\x1b[38;5;201m",
	NoLevel:    "\x1b[38;5;60m",
	Timestamp:  "\x1b[38;5;69m",
	MessageKey: "\x1b[38;5;45m",
	Message:    "\x1b[1;38;5;219m",
}
