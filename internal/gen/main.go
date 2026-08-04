// Command gen writes the parts of the theme that follow from spec/tokens.json.
//
// Only files that participate in a variant axis are generated. Artwork that no
// axis touches — tabbar, line, plasmoidheading, tasks, and the widget artwork
// whose colours are already resolved at paint time — stays hand-maintained,
// because generating a file that never varies buys nothing and costs a template.
//
// Run it with `go generate ./...` from the repository root. Output is committed;
// see DESIGN.md.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type tokens struct {
	Palettes map[string]map[string]string `json:"palettes"`
	Accents  map[string]map[string]string `json:"accents"`
	Status   map[string]string            `json:"status"`

	SurfaceShape    map[string]map[string]float64 `json:"surfaceShape"`
	DecorationShape map[string]map[string]float64 `json:"decorationShape"`
	ButtonStyles    map[string]buttonStyle        `json:"buttonStyles"`
	Opacity         map[string]float64            `json:"opacity"`
}

// buttonStyle is one titlebar button treatment. The plate opacities differ
// between close and the rest because close is the only button that earns a
// colour of its own.
type buttonStyle struct {
	PlateRadius float64 `json:"plateRadius"`

	ClosePlate   string `json:"closePlate"`
	CloseHover   string `json:"closeHover"`
	ClosePressed string `json:"closePressed"`

	PlainHover   string `json:"plainHover"`
	PlainPressed string `json:"plainPressed"`

	Rest string `json:"rest"`
	Dim  string `json:"dim"`

	Width  int `json:"width"`
	Height int `json:"height"`

	ButtonsOnLeft  string `json:"buttonsOnLeft"`
	ButtonsOnRight string `json:"buttonsOnRight"`
}

const (
	style      = "assets/plasma/desktoptheme/vanilla-box-dark"
	schemeDir  = "assets/color-schemes"
	auroraeDir = "assets/aurorae/themes/VanillaBoxDark"
	lookFeel   = "assets/plasma/look-and-feel/org.vanillabox.dark"

	// The titlebar row is the rc's TitleHeight plus the frame's own top border.
	titleBorder = 1
)

func main() {
	root := flag.String("root", ".", "repository root to write into")
	flag.Parse()

	if err := run(*root); err != nil {
		fmt.Fprintln(os.Stderr, "gen:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	tk, err := loadTokens(filepath.Join(root, "spec", "tokens.json"))
	if err != nil {
		return err
	}

	files, err := build(tk, "neutral", "sand", "rounded", "rounded", "windows")
	if err != nil {
		return err
	}

	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}

	return nil
}

// build returns every generated file for one point in the variant space, keyed
// by repository-relative slash path.
func build(tk *tokens, tint, accent, shape, decoShape, buttons string) (map[string]string, error) {
	palette, ok := tk.Palettes[tint]
	if !ok {
		return nil, fmt.Errorf("no palette %q", tint)
	}
	acc, ok := tk.Accents[accent]
	if !ok {
		return nil, fmt.Errorf("no accent %q", accent)
	}
	radii, ok := tk.SurfaceShape[shape]
	if !ok {
		return nil, fmt.Errorf("no surface shape %q", shape)
	}

	out := map[string]string{}

	// The application scheme names itself by id; the Plasma style's copy names
	// itself the way Plasma's own themes do, by display name.
	base := scheme{palette: palette, accent: acc, status: tk.Status, Name: "Vanilla Box Dark"}

	app := base
	app.SchemeKey = "VanillaBoxDark"
	out[schemeDir+"/VanillaBoxDark.colors"] = app.render()

	shell := base
	shell.SchemeKey = "Vanilla Box Dark"
	out[style+"/colors"] = shell.render()

	popup := frame{
		Size: 44, Canvas: 60, Tile: 10, Radius: radii["popup"],
		Fallback: palette["background"], Mask: true, HintSize: 8, HintY: 48,
	}
	tooltip := frame{
		Size: 44, Canvas: 60, Tile: 10, Radius: radii["popup"],
		Fallback: palette["elevated"], Mask: true, HintSize: 4, HintY: 48,
	}
	panel := frame{
		Size: 40, Canvas: 56, Tile: 8, Radius: radii["panel"],
		Fallback: palette["background"], HintSize: 2, HintY: 44, Inline: true,
	}

	// The theme root carries the translucent artwork. Plasma falls back to the
	// opaque/ and solid/ prefixes itself when compositing is off, so both ship
	// whatever the transparency options are set to.
	for _, prefix := range []string{"", "opaque/", "solid/"} {
		p, t, pan := popup, tooltip, panel
		if prefix == "" {
			p.Opacity = opacity(tk.Opacity["popup"])
			t.Opacity = opacity(tk.Opacity["tooltip"])
			pan.Opacity = opacity(tk.Opacity["panel"])
		}

		out[style+"/"+prefix+"widgets/background.svg"] = p.render()
		out[style+"/"+prefix+"dialogs/background.svg"] = p.render()
		out[style+"/"+prefix+"widgets/tooltip.svg"] = t.render()
		out[style+"/"+prefix+"widgets/panel-background.svg"] = pan.render()
	}

	for path, c := range controls(palette, acc, tk.Status, radii["button"]) {
		out[style+"/widgets/"+path] = c.render()
	}

	deco, ok := tk.DecorationShape[decoShape]
	if !ok {
		return nil, fmt.Errorf("no decoration shape %q", decoShape)
	}
	bs, ok := tk.ButtonStyles[buttons]
	if !ok {
		return nil, fmt.Errorf("no button style %q", buttons)
	}

	titleHeight := 30

	out[auroraeDir+"/decoration.svg"] = decoration{
		Width: 40, TitleH: titleHeight + titleBorder, BodyH: 24, Step: 60,
		Radius: deco["titlebar"], Border: palette["elevatedAlt"], Backgnd: palette["background"],
	}.render()

	plain := button{
		PlateFill: palette["text"], HoverOpacity: bs.PlainHover, PressedOpacity: bs.PlainPressed,
		Radius: bs.PlateRadius, GlyphFill: palette["text"], RestOpen: bs.Rest, DimOpacity: bs.Dim,
	}
	closeBtn := plain
	closeBtn.PlateFill = bs.ClosePlate
	closeBtn.HoverOpacity = bs.CloseHover
	closeBtn.PressedOpacity = bs.ClosePressed

	for name, glyph := range map[string]string{
		"minimize": glyphMinimize,
		"maximize": glyphMaximize,
		"restore":  glyphRestore,
	} {
		b := plain
		b.Glyph = glyph
		out[auroraeDir+"/"+name+".svg"] = b.render()
	}

	closeBtn.Glyph = glyphClose
	out[auroraeDir+"/close.svg"] = closeBtn.render()

	out[auroraeDir+"/VanillaBoxDarkrc"] = auroraeRC(palette, bs, titleHeight)
	out[lookFeel+"/contents/defaults"] = lookAndFeelDefaults(acc, bs.ButtonsOnLeft, bs.ButtonsOnRight)

	return out, nil
}

// controls builds the widget artwork that shares the stacked nine-tile idiom:
// buttons, text fields and list items.
func controls(palette, acc, status map[string]string, radius float64) map[string]control {
	sheet := fmt.Sprintf(
		".ColorScheme-Text { color:%s; }.ColorScheme-Highlight { color:%s; }"+
			".ColorScheme-ButtonBackground { color:%s; }.ColorScheme-ButtonHover { color:%s; }",
		palette["text"], acc["highlight"], palette["elevated"], status["hover"])

	const (
		text    = "ColorScheme-Text"
		hi      = "ColorScheme-Highlight"
		btnBg   = "ColorScheme-ButtonBackground"
		btnHvr  = "ColorScheme-ButtonHover"
		fullTop = 6
	)

	// set places a state at its slot down the canvas, so the offsets stay
	// derived rather than repeated.
	set := func(slot int, prefix string, tile int, layers ...layer) tileSet {
		return tileSet{Prefix: prefix, Y: slot * controlStep, Tile: tile, Radius: radius, Layers: layers}
	}

	return map[string]control{
		"button.svg": {
			Height: 7 * controlStep, Style: sheet,
			Sets: []tileSet{
				set(0, "normal", fullTop, layer{btnBg, "1.0"}),
				set(1, "hover", fullTop, layer{btnBg, "1.0"}, layer{btnHvr, "0.25"}),
				set(2, "pressed", fullTop, layer{btnBg, "1.0"}, layer{btnHvr, "0.45"}),
				set(3, "focus", fullTop),
				set(4, "toolbutton-hover", fullTop, layer{text, "0.08"}),
				set(5, "toolbutton-pressed", fullTop, layer{text, "0.15"}),
				set(6, "toolbutton-focus", fullTop),
			},
		},
		"lineedit.svg": {
			Height: 3 * controlStep, Style: sheet,
			Sets: []tileSet{
				set(0, "base", fullTop, layer{text, "0.06"}),
				set(1, "hover", fullTop, layer{text, "0"}),
				set(2, "focus", fullTop, layer{text, "0"}),
			},
			Hints: []string{"hint-focus-over-base", "hint-hover-over-base"},
		},
		"viewitem.svg": {
			Height: 5 * controlStep, Style: sheet,
			Sets: []tileSet{
				// The resting state draws nothing, and its tiles are inset 3 rather
				// than 6 so a list row's margins do not follow the hover radius.
				set(0, "normal", 3),
				set(1, "hover", fullTop, layer{hi, "0.25"}),
				set(2, "selected", fullTop, layer{hi, "0.25"}),
				set(3, "selected+hover", fullTop, layer{hi, "0.25"}),
				set(4, "focus", fullTop, layer{text, "0.08"}),
			},
		},
	}
}

// opacity renders a fill-opacity, treating a fully opaque surface as no
// attribute at all rather than an explicit 1.
func opacity(v float64) string {
	if v == 0 || v == 1 {
		return ""
	}

	return n(v)
}

func loadTokens(path string) (*tokens, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	tk := &tokens{}
	if err := json.Unmarshal(data, tk); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return tk, nil
}
