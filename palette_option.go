package pslog

import "pkt.systems/pslog/ansi"

func resolvePaletteOption(palette *ansi.Palette) *ansi.Palette {
	if palette != nil {
		return palette
	}
	return &ansi.PaletteDefault
}
