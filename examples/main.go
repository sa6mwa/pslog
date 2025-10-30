// pslog examples
package main

import (
	"fmt"
	"os"
	"time"

	"pkt.systems/pslog"
	"pkt.systems/pslog/ansi"
)

func main() {
	logger := pslog.New(os.Stdout).With("adapter", "pslog").With("mode", "console").LogLevel(pslog.TraceLevel)
	logger.Debug("Hello, this is logport's native adapter in console mode")
	logger.Info("This is a typical info message")
	logger.Warn("This is a warning message")
	logger.Error("This is an error message")
	fmt.Println("")

	logger = pslog.NewStructured(os.Stdout).With("adapter", "pslog").With("mode", "structured").WithLogLevel().With("num", 123)
	logger.Info("Hello, this is logport's native structured logger")
	logger.Warn("This is a warning message")

	popts := pslog.Options{
		Mode:       pslog.ModeStructured,
		TimeFormat: time.RFC3339Nano,
		UTC:        true,
	}
	logger = pslog.NewWithOptions(os.Stdout, popts).With("adapter", "pslog")
	logger.Info("This is in UTC")
	fmt.Println("")

	logger = pslog.NewStructured(os.Stdout).WithLogLevel().LogLevel(pslog.DebugLevel)
	logger.Trace("trace")
	logger.Debug("debug")
	logger.Info("info")
	logger.Warn("warn")
	logger.Error("error")

	logger = logger.LogLevel(pslog.TraceLevel)
	logger.Trace("after changing lovlevel to Trace, this should show")
	fmt.Println("")

	logger = pslog.New(os.Stdout).WithLogLevel()
	logger.Debug(
		"hello",
		"ts_iso", time.Now(), // Timestamp (ansi.Timestamp)
		"user", "alice", // Key/value (ansi.Key + ansi.String)
		"attempts", 3, // Number (ansi.Num)
		"latency_ms", 12.34, // Number (ansi.Num)
		"ok", true, // Bool (ansi.Bool)
		"status", nil, // Nil (ansi.Nil)
		"err", fmt.Errorf("disk full"), // String-colored error (ansi.String)
	)

	logger = pslog.NewStructured(os.Stdout).WithLogLevel()
	logger.Info(
		"hello",
		"ts_iso", time.Now(), // Timestamp (ansi.Timestamp)
		"user", "alice", // Key/value (ansi.Key + ansi.String)
		"attempts", 3, // Number (ansi.Num)
		"latency_ms", 12.34, // Number (ansi.Num)
		"ok", true, // Bool (ansi.Bool)
		"status", nil, // Nil (ansi.Nil)
		"err", fmt.Errorf("disk full"), // String-colored error (ansi.String)
	)

	paintTheWorld("pslog was here")

	fmt.Println("")
}

func paintTheWorld(msg string) {
	palettes := []struct {
		name    string
		palette ansi.Palette
	}{
		{name: "ansi.PaletteDefault", palette: ansi.PaletteDefault},
		{name: "ansi.PaletteOutrunElectric", palette: ansi.PaletteOutrunElectric},
		{name: "ansi.PaletteTokyoNight", palette: ansi.PaletteTokyoNight},
		{name: "ansi.PlaetteDoomDracula", palette: ansi.PaletteDoomDracula},
		{name: "ansi.PaletteDoomGruvbox", palette: ansi.PaletteDoomGruvbox},
		{name: "ansi.PaletteDoomIosvkem", palette: ansi.PaletteDoomIosvkem},
		{name: "ansi.PaletteDoomNord", palette: ansi.PaletteDoomNord},
		{name: "ansi.PaletteSolarizedNightfall", palette: ansi.PaletteSolarizedNightfall},
		{name: "ansi.PaletteCatppuccinMocha", palette: ansi.PaletteCatppuccinMocha},
		{name: "ansi.PaletteGruvboxLight", palette: ansi.PaletteGruvboxLight},
		{name: "ansi.PaletteMonokaiVibrant", palette: ansi.PaletteMonokaiVibrant},
		{name: "ansi.PaletteOneDarkAurora", palette: ansi.PaletteOneDarkAurora},
		{name: "ansi.PaletteSynthwave84", palette: ansi.PaletteSynthwave84},
	}

	for _, palette := range palettes {
		fmt.Println("")
		ansi.SetPalette(palette.palette)
		logger := pslog.NewStructured(os.Stdout).LogLevel(pslog.TraceLevel).WithLogLevel().With("num", 1337).With("cool", true).With("palette", palette.name)
		logger.Trace(msg)
		logger.Debug(msg)
		logger.Info(msg)
		logger.Warn(msg)
		logger.Error(msg)
		logger = pslog.New(os.Stdout).LogLevel(pslog.TraceLevel).WithLogLevel().With("num", 1337.73).With("cool", true).With("palette", palette.name)
		logger.Trace(msg)
		logger.Debug(msg)
		logger.Info(msg)
		logger.Warn(msg)
		logger.Error(msg)
	}

}
