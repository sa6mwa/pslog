// pslog examples
package main

import (
	"context"
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
		"duration", time.Microsecond*123,
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
		"duration", time.Microsecond*123,
		"ok", true, // Bool (ansi.Bool)
		"status", nil, // Nil (ansi.Nil)
		"err", fmt.Errorf("disk full"), // String-colored error (ansi.String)
	)

	paintTheWorld("pslog was here")

	fmt.Println("")

	logger = pslog.NewStructured(os.Stdout).WithLogLevel()
	logger.With(fmt.Errorf("this is a test error")).Info("testing the single With(err) field", "err", fmt.Errorf("inline error"), "text", "plain field")
	logger = pslog.New(os.Stdout).WithLogLevel()
	logger.With(fmt.Errorf("this is a test error")).Warn("testing the single With(err) field", "err", fmt.Errorf("inline error"), "text", "plain field")

	fmt.Println("")
	ctx := pslog.ContextWithLogger(context.Background(), pslog.New(os.Stdout).WithLogLevel().With("logger_src", "context"))
	pslog.Ctx(ctx).Info("This is pslog.Logger from the context")
	pslog.BCtx(ctx).Info("And this is a pslog.Base from the same context")
	pslog.Ctx(ctx).With(fmt.Errorf("oops")).Debug("This is the context logger with an error field")
	pslog.Ctx(ctx).With("fn", pslog.CurrentFn()).Debug("Current function in fn key")

	fmt.Println("")
	ctx = pslog.ContextWithLogger(ctx, pslog.NewWithOptions(os.Stdout, pslog.Options{
		Mode:         pslog.ModeStructured,
		CallerKeyval: true,
	}).WithLogLevel().With("logger_src", "context"))
	pslog.Ctx(ctx).Warn("fn should indicate main")
	anotherFunctionThanMain(ctx)
	pslog.Ctx(ctx).Debug("and we are back where fn should be main")

	fmt.Println("")
	ctx = pslog.ContextWithBaseLogger(ctx, pslog.NewWithOptions(os.Stdout, pslog.Options{
		Mode:         pslog.ModeConsole,
		CallerKeyval: true,
	}))
	pslog.Ctx(ctx).Info("fn should be main")
	anotherFunctionThanMain(ctx)
	pslog.Ctx(ctx).Debug("back where fn should be main")

	fmt.Println("")
	os.Setenv("LOG_MODE", "json")
	os.Setenv("LOG_LEVEL", "trace")
	os.Setenv("LOG_CALLER_KEYVAL", "t")
	ctx = pslog.ContextWithLogger(context.Background(), pslog.LoggerFromEnv().WithLogLevel())
	pslog.Ctx(ctx).Debug("this logger is from env")
	pslog.Ctx(ctx).With(fmt.Errorf("oops")).Error("logger from env")
}

func anotherFunctionThanMain(ctx context.Context) {
	pslog.Ctx(ctx).Info("fn should indicate anotherFunction than main")
	pslog.Ctx(ctx).With(fmt.Errorf("nok")).Error("error where fn should be anotherFunction than main")
}

func paintTheWorld(msg string) {
	palettes := []struct {
		name    string
		palette *ansi.Palette
	}{
		{name: "ansi.PaletteDefault", palette: &ansi.PaletteDefault},
		{name: "ansi.PaletteAyuMirage", palette: &ansi.PaletteAyuMirage},
		{name: "ansi.PaletteEverforest", palette: &ansi.PaletteEverforest},
		{name: "ansi.PaletteOutrunElectric", palette: &ansi.PaletteOutrunElectric},
		{name: "ansi.PaletteTokyoNight", palette: &ansi.PaletteTokyoNight},
		{name: "ansi.PlaetteDracula", palette: &ansi.PaletteDracula},
		{name: "ansi.PaletteGruvbox", palette: &ansi.PaletteGruvbox},
		{name: "ansi.PaletteIosvkem", palette: &ansi.PaletteIosvkem},
		{name: "ansi.PaletteNord", palette: &ansi.PaletteNord},
		{name: "ansi.PaletteSolarizedNightfall", palette: &ansi.PaletteSolarizedNightfall},
		{name: "ansi.PaletteCatppuccinMocha", palette: &ansi.PaletteCatppuccinMocha},
		{name: "ansi.PaletteGruvboxLight", palette: &ansi.PaletteGruvboxLight},
		{name: "ansi.PaletteMonokaiVibrant", palette: &ansi.PaletteMonokaiVibrant},
		{name: "ansi.PaletteOneDarkAurora", palette: &ansi.PaletteOneDarkAurora},
		{name: "ansi.PaletteSynthwave84", palette: &ansi.PaletteSynthwave84},
	}

	for _, palette := range palettes {
		fmt.Println("")
		logger := pslog.NewWithPalette(os.Stdout, pslog.ModeStructured, palette.palette).LogLevel(pslog.TraceLevel).WithLogLevel().With("num", 1337).With("cool", true).With("duration", time.Microsecond*123).With("palette", palette.name)
		logger.Trace(msg)
		logger.Debug(msg)
		logger.Info(msg)
		logger.Warn(msg)
		logger.Error(msg)
		logger = pslog.NewWithPalette(os.Stdout, pslog.ModeConsole, palette.palette).LogLevel(pslog.TraceLevel).WithLogLevel().With("num", 1337.73).With("cool", true).With("dur", time.Microsecond*123).With("palette", palette.name)
		logger.Trace(msg)
		logger.Debug(msg)
		logger.Info(msg)
		logger.Warn(msg)
		logger.Error(msg)
	}
}
