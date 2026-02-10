package pslog

import (
	"io"
	"sync"
	"testing"

	"pkt.systems/pslog/ansi"
)

func TestPaletteSwapConcurrentWithColorLogging(t *testing.T) {
	original := ansi.Snapshot()
	t.Cleanup(func() { ansi.SetPalette(original) })

	modes := []Mode{ModeConsole, ModeStructured}
	for _, mode := range modes {
		logger := NewWithOptions(io.Discard, Options{
			Mode:       mode,
			ForceColor: true,
			MinLevel:   TraceLevel,
		})

		var wg sync.WaitGroup
		for worker := 0; worker < 4; worker++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for i := 0; i < 2_000; i++ {
					logger.Info("palette-race", "mode", mode, "worker", id, "iter", i)
				}
			}(worker)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			palettes := []ansi.Palette{
				ansi.PaletteDefault,
				ansi.PaletteSynthwave84,
				ansi.PaletteTokyoNight,
				ansi.PaletteMonokaiVibrant,
			}
			for i := 0; i < 2_000; i++ {
				ansi.SetPalette(palettes[i%len(palettes)])
			}
		}()

		wg.Wait()
	}
}
