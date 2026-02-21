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
	ctx := context.Background()
	l := pslog.NewWithPalette(ctx, os.Stdout, pslog.ModeStructured, &ansi.PaletteMonokaiVibrant).LogLevel(pslog.TraceLevel)
	c := pslog.NewWithPalette(ctx, os.Stdout, pslog.ModeConsole, &ansi.PaletteMonokaiVibrant).LogLevel(pslog.TraceLevel).With("logger", "pkt.systems/pslog", "mode", "console")

	c.Info("Hello 🤓")

	time.Sleep(2000 * time.Millisecond)

	l.Info("Structured is cooler 😎")

	time.Sleep(3000 * time.Millisecond)

	c.Warn("No 🤡")

	time.Sleep(3000 * time.Millisecond)

	l.Debug("🥱")

	time.Sleep(2200 * time.Millisecond)

	c.Trace("Bot! 🤖")

	time.Sleep(3000 * time.Millisecond)

	l.Info("AI helped us become fast", "fact", true)

	time.Sleep(2600 * time.Millisecond)

	c.Error("What?", "😮", true, "error", fmt.Errorf("does not compute"))

	time.Sleep(3500 * time.Millisecond)

	l = pslog.NewWithPalette(ctx, os.Stdout, pslog.ModeStructured, &ansi.PaletteSynthwave84).LogLevel(pslog.TraceLevel)
	c = pslog.NewWithPalette(ctx, os.Stdout, pslog.ModeConsole, &ansi.PaletteSynthwave84).LogLevel(pslog.TraceLevel).With("logger", "pkt.systems/pslog")

	l.Trace("AI also gave us cool colors")

	time.Sleep(3000 * time.Millisecond)

	c.Info("True 💯", "cool", true)
	time.Sleep(500 * time.Millisecond)
	c.Info("Get pslog now!", "command", "go get pkt.systems/pslog@latest")

}
