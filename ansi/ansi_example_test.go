package ansi_test

import (
	"fmt"

	"pkt.systems/pslog/ansi"
)

func ExampleSetPalette() {
	before := ansi.Snapshot()
	defer ansi.SetPalette(before)

	ansi.SetPalette(ansi.PaletteSynthwave84)
	after := ansi.Snapshot()
	fmt.Println(after.Key == ansi.PaletteSynthwave84.Key)

	// Output: true
}

func ExamplePaletteByName() {
	palette := ansi.PaletteByName("doom-nord")
	fmt.Println(palette == &ansi.PaletteNord)

	unknown := ansi.PaletteByName("not-a-real-palette")
	fmt.Println(unknown == &ansi.PaletteDefault)

	// Output:
	// true
	// true
}
