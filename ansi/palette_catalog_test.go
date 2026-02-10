package ansi

import "testing"

func TestPaletteByNameCanonical(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want Palette
	}{
		{name: "default", want: PaletteDefault},
		{name: "tokyo-night", want: PaletteTokyoNight},
		{name: "one-dark", want: PaletteOneDark},
		{name: "synthwave-84", want: PaletteSynthwave84},
		{name: "kanagawa", want: PaletteKanagawa},
	}

	for _, tc := range cases {
		got := PaletteByName(tc.name)
		if got == nil {
			t.Fatalf("expected palette %q to resolve", tc.name)
		}
		if *got != tc.want {
			t.Fatalf("palette %q mismatch", tc.name)
		}
	}
}

func TestPaletteByNameAliases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want Palette
	}{
		{name: "doom-iosvkem", want: PaletteIosvkem},
		{name: "doom_gruvbox", want: PaletteGruvbox},
		{name: "doomDracula", want: PaletteDracula},
		{name: "doomnord", want: PaletteNord},
		{name: "synthwave84", want: PaletteSynthwave84},
		{name: "PaletteTokyoNight", want: PaletteTokyoNight},
	}

	for _, tc := range cases {
		got := PaletteByName(tc.name)
		if got == nil {
			t.Fatalf("expected alias %q to resolve", tc.name)
		}
		if *got != tc.want {
			t.Fatalf("alias %q mismatch", tc.name)
		}
	}
}

func TestPaletteByNameInvalid(t *testing.T) {
	t.Parallel()

	got := PaletteByName("does-not-exist")
	if got == nil {
		t.Fatalf("expected unknown palette lookup to return default")
	}
	if *got != PaletteDefault {
		t.Fatalf("expected unknown palette lookup to return default palette")
	}
}

func TestAvailablePaletteNames(t *testing.T) {
	t.Parallel()

	names := AvailablePaletteNames()
	if len(names) < 20 {
		t.Fatalf("expected expanded palette list, got %d names", len(names))
	}
	required := map[string]bool{
		"default":         false,
		"iosvkem":         false,
		"one-dark":        false,
		"synthwave-84":    false,
		"solarized-light": false,
		"horizon":         false,
	}
	for _, name := range names {
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}
	for name, seen := range required {
		if !seen {
			t.Fatalf("expected palette name %q in catalog", name)
		}
	}
}
