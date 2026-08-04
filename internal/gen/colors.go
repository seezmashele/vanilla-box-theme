package main

import (
	"fmt"
	"strconv"
	"strings"
)

// colorSet is one [Colors:*] section. Only the backgrounds and the focus
// decoration vary between sections; every other role is shared, which is why
// they are filled in by scheme rather than repeated per section.
type colorSet struct {
	Name             string
	BackgroundAlt    string
	BackgroundNormal string
	DecorationFocus  string

	// Selection inverts: its foreground sits on the highlight rather than on a
	// surface, so it carries its own text colours.
	ForegroundNormal   string
	ForegroundInactive string
}

// scheme renders a KColorScheme ini. The same content serves the application
// colour scheme and the Plasma style's own colors file; they differ only in the
// [General] ColorScheme key, which names the scheme by id in one and by display
// name in the other.
type scheme struct {
	palette map[string]string
	accent  map[string]string
	status  map[string]string

	SchemeKey string
	Name      string
}

func (s scheme) render() string {
	p, a, st := s.palette, s.accent, s.status

	sets := []colorSet{
		{Name: "Button", BackgroundAlt: p["backgroundAlt"], BackgroundNormal: p["elevated"], DecorationFocus: p["elevated"]},
		{Name: "Complementary", BackgroundAlt: p["backgroundAlt"], BackgroundNormal: p["background"], DecorationFocus: p["focusDim"]},
		{Name: "Header", BackgroundAlt: p["backgroundAlt"], BackgroundNormal: p["background"], DecorationFocus: p["background"]},
		{
			Name: "Selection", BackgroundAlt: a["highlight"], BackgroundNormal: a["highlight"],
			DecorationFocus:  a["highlight"],
			ForegroundNormal: p["onHighlight"], ForegroundInactive: p["onHighlight"],
		},
		{Name: "Tooltip", BackgroundAlt: p["elevatedAlt"], BackgroundNormal: p["elevated"], DecorationFocus: p["elevated"]},
		{Name: "View", BackgroundAlt: p["backgroundAlt"], BackgroundNormal: p["view"], DecorationFocus: p["focusDim"]},
		{Name: "Window", BackgroundAlt: p["backgroundAlt"], BackgroundNormal: p["background"], DecorationFocus: p["background"]},
	}

	var b strings.Builder

	b.WriteString(`[ColorEffects:Disabled]
ColorAmount=0
ColorEffect=0
ContrastAmount=0.65
ContrastEffect=1
IntensityAmount=0.1
IntensityEffect=2

[ColorEffects:Inactive]
ChangeSelectionColor=true
ColorAmount=0.025
ColorEffect=2
ContrastAmount=0.1
ContrastEffect=2
Enable=false
IntensityAmount=0
IntensityEffect=0

`)

	for _, set := range sets {
		normal, inactive := p["text"], p["textInactive"]
		if set.ForegroundNormal != "" {
			normal, inactive = set.ForegroundNormal, set.ForegroundInactive
		}

		fmt.Fprintf(&b, "[Colors:%s]\n", set.Name)
		for _, kv := range [][2]string{
			{"BackgroundAlternate", set.BackgroundAlt},
			{"BackgroundNormal", set.BackgroundNormal},
			{"DecorationFocus", set.DecorationFocus},
			{"DecorationHover", st["hover"]},
			{"ForegroundActive", a["highlight"]},
			{"ForegroundInactive", inactive},
			{"ForegroundLink", a["highlight"]},
			{"ForegroundNegative", st["negative"]},
			{"ForegroundNeutral", st["neutral"]},
			{"ForegroundNormal", normal},
			{"ForegroundPositive", st["positive"]},
			{"ForegroundVisited", st["visited"]},
		} {
			fmt.Fprintf(&b, "%s=%s\n", kv[0], rgb(kv[1]))
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "[General]\nColorScheme=%s\nName=%s\nshadeSortColumn=true\n\n", s.SchemeKey, s.Name)
	b.WriteString("[KDE]\ncontrast=4\n\n")

	fmt.Fprintf(&b, "[WM]\nactiveBackground=%s\nactiveBlend=%s\nactiveForeground=%s\n",
		rgb(p["background"]), rgb(p["text"]), rgb(p["text"]))
	fmt.Fprintf(&b, "inactiveBackground=%s\ninactiveBlend=%s\ninactiveForeground=%s\n",
		rgb(p["background"]), rgb(p["textInactive"]), rgb(p["textInactive"]))

	return b.String()
}

// rgb converts #rrggbb into the comma-separated triple KColorScheme files use.
func rgb(hex string) string {
	v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		panic("tokens: not a #rrggbb colour: " + hex)
	}

	return fmt.Sprintf("%d,%d,%d", v>>16&0xff, v>>8&0xff, v&0xff)
}
