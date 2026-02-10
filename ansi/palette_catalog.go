package ansi

import (
	"sort"
	"strings"
)

var namedPalettes = map[string]*Palette{
	"default":             &PaletteDefault,
	"outrun-electric":     &PaletteOutrunElectric,
	"iosvkem":             &PaletteIosvkem,
	"gruvbox":             &PaletteGruvbox,
	"dracula":             &PaletteDracula,
	"nord":                &PaletteNord,
	"tokyo-night":         &PaletteTokyoNight,
	"solarized-nightfall": &PaletteSolarizedNightfall,
	"catppuccin-mocha":    &PaletteCatppuccinMocha,
	"gruvbox-light":       &PaletteGruvboxLight,
	"monokai-vibrant":     &PaletteMonokaiVibrant,
	"one-dark-aurora":     &PaletteOneDarkAurora,
	"synthwave-84":        &PaletteSynthwave84,
	"kanagawa":            &PaletteKanagawa,
	"rose-pine":           &PaletteRosePine,
	"rose-pine-dawn":      &PaletteRosePineDawn,
	"everforest":          &PaletteEverforest,
	"everforest-light":    &PaletteEverforestLight,
	"night-owl":           &PaletteNightOwl,
	"ayu-mirage":          &PaletteAyuMirage,
	"ayu-light":           &PaletteAyuLight,
	"one-light":           &PaletteOneLight,
	"one-dark":            &PaletteOneDark,
	"solarized-light":     &PaletteSolarizedLight,
	"solarized-dark":      &PaletteSolarizedDark,
	"github-light":        &PaletteGithubLight,
	"github-dark":         &PaletteGithubDark,
	"papercolor-light":    &PalettePapercolorLight,
	"papercolor-dark":     &PalettePapercolorDark,
	"oceanic-next":        &PaletteOceanicNext,
	"horizon":             &PaletteHorizon,
	"palenight":           &PalettePalenight,
}

var paletteAliases = map[string]string{
	"doom-iosvkem": "iosvkem",
	"doomiosvkem":  "iosvkem",
	"doom-gruvbox": "gruvbox",
	"doomgruvbox":  "gruvbox",
	"doom-dracula": "dracula",
	"doomdracula":  "dracula",
	"doom-nord":    "nord",
	"doomnord":     "nord",

	"outrunelectric":     "outrun-electric",
	"tokyonight":         "tokyo-night",
	"solarizednightfall": "solarized-nightfall",
	"catppuccinmocha":    "catppuccin-mocha",
	"gruvboxlight":       "gruvbox-light",
	"monokaivibrant":     "monokai-vibrant",
	"onedarkaurora":      "one-dark-aurora",
	"synthwave84":        "synthwave-84",
	"rosepine":           "rose-pine",
	"rosepinedawn":       "rose-pine-dawn",
	"nightowl":           "night-owl",
	"ayumirage":          "ayu-mirage",
	"ayulight":           "ayu-light",
	"onelight":           "one-light",
	"onedark":            "one-dark",
	"solarizedlight":     "solarized-light",
	"solarizeddark":      "solarized-dark",
	"githublight":        "github-light",
	"githubdark":         "github-dark",
	"papercolorlight":    "papercolor-light",
	"papercolordark":     "papercolor-dark",
	"oceanicnext":        "oceanic-next",
}

// PaletteByName resolves a built-in palette by its canonical name.
// Names are case-insensitive and support compatibility aliases.
func PaletteByName(name string) *Palette {
	normalized := normalizePaletteName(name)
	if normalized == "" {
		return &PaletteDefault
	}
	if canonical, ok := paletteAliases[normalized]; ok {
		normalized = canonical
	}
	if palette, ok := namedPalettes[normalized]; ok && palette != nil {
		return palette
	}
	return &PaletteDefault
}

// AvailablePaletteNames returns canonical built-in palette names in sorted order.
func AvailablePaletteNames() []string {
	names := make([]string, 0, len(namedPalettes))
	for name := range namedPalettes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizePaletteName(name string) string {
	s := strings.TrimSpace(strings.ToLower(name))
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if strings.HasPrefix(s, "palette-") {
		s = strings.TrimPrefix(s, "palette-")
	} else if strings.HasPrefix(s, "palette") {
		s = strings.TrimPrefix(s, "palette")
		s = strings.TrimLeft(s, "-")
	}
	return s
}
