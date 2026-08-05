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
	"io/fs"
	"os"
	"path/filepath"
)

type tokens struct {
	Theme      identity                     `json:"theme"`
	Foreground map[string]string            `json:"foreground"`
	Surfaces   map[string]map[string]string `json:"surfaces"`
	Palettes   map[string]palette           `json:"palettes"`
	Status     map[string]string            `json:"status"`

	SurfaceShape    map[string]map[string]float64 `json:"surfaceShape"`
	DecorationShape map[string]map[string]float64 `json:"decorationShape"`
	ButtonStyles    map[string]buttonStyle        `json:"buttonStyles"`
	Opacity         map[string]float64            `json:"opacity"`
}

// palette is a named colour variant: a surface set and the accent chosen to go
// with it. Pairing them here rather than offering two independent choices is
// what lets the installer ask one question instead of two.
type palette struct {
	Surfaces string `json:"surfaces"`
	Accent   string `json:"accent"`
}

// buttonStyle is one titlebar button treatment. The plate opacities differ
// between close and the rest because close is the only button that earns a
// colour of its own.
type buttonStyle struct {
	// Kind picks the treatment: "glyph" shows a symbol always and gains a plate
	// on hover; "circle" shows no symbol and gains colour on hover.
	Kind string `json:"kind"`

	PlateRadius float64 `json:"plateRadius"`

	ClosePlate   string `json:"closePlate"`
	CloseHover   string `json:"closeHover"`
	ClosePressed string `json:"closePressed"`

	PlainHover   string `json:"plainHover"`
	PlainPressed string `json:"plainPressed"`

	// GlyphSize is how big the symbol is on screen, in pixels. It assumes a
	// square button box, which is what keeps the glyph from being stretched.
	GlyphSize float64 `json:"glyphSize"`

	Rest string `json:"rest"`
	Dim  string `json:"dim"`

	// The circle treatment.
	CircleRadius   float64 `json:"circleRadius"`
	RestColor      string  `json:"restColor"`
	DimColor       string  `json:"dimColor"`
	Close          string  `json:"close"`
	Minimize       string  `json:"minimize"`
	Maximize       string  `json:"maximize"`
	PressedOpacity string  `json:"pressedOpacity"`

	Width  int `json:"width"`
	Height int `json:"height"`

	// MenuWidth sizes the titlebar's application icon. Aurorae gives every
	// button the same height, so this is the only per-button size there is: the
	// icon shrinks by width and the leftover height centres it, which is where
	// its breathing room above and below comes from.
	MenuWidth int `json:"menuWidth"`

	// NudgeTop is added to the margin that centres the button in the titlebar.
	// Centred and looking centred are not always the same thing, and this is
	// where that difference is admitted to rather than hidden in the artwork.
	NudgeTop int `json:"nudgeTop"`

	ButtonsOnLeft  string `json:"buttonsOnLeft"`
	ButtonsOnRight string `json:"buttonsOnRight"`
}

const (
	style      = "assets/plasma/desktoptheme/vanilla-box-dark"
	schemeDir  = "assets/color-schemes"
	auroraeDir = "assets/aurorae/themes/VanillaBoxDark"
	lookFeel   = "assets/plasma/look-and-feel/org.vanillabox.dark"
	variantDir = "assets/variants"

	// The theme as shipped. Every other point in the variant space is written
	// under variants/ and copied over an install by the option that names it.
	defaultPalette = "neutral"

	// Square is the default throughout, so that accepting every prompt gives a
	// theme with no rounded corners anywhere. It is also the only titlebar shape
	// without a compromise: the rounded variant cannot round its bottom corners
	// under Aurorae.
	defaultSurfaces = "square"
	defaultTitlebar = "square"
	defaultButtons  = "mac"

	// The titlebar row is the rc's TitleHeight plus the frame's own top border.
	titleHeight = 30
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

	files, err := allFiles(tk)
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

	return prune(root, files)
}

// prune deletes anything under variants/ the generator did not just write.
//
// Renaming a variant otherwise leaves the old one behind, committed and
// installable, describing a combination the manifest no longer offers. Only
// variants/ is swept: the rest of assets/ mixes generated files with
// hand-maintained ones, and nothing there should be deleted by a build.
func prune(root string, written map[string]string) error {
	dir := filepath.Join(root, filepath.FromSlash(variantDir))
	if _, err := os.Stat(dir); err != nil {
		return nil
	}

	var stale []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, ok := written[filepath.ToSlash(rel)]; !ok {
			stale = append(stale, path)
		}

		return nil
	})
	if err != nil {
		return err
	}

	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	// Directories the removals emptied would otherwise linger as the shape of a
	// variant that no longer exists.
	return removeEmptyDirs(dir)
}

func removeEmptyDirs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if err := removeEmptyDirs(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}

	if entries, err = os.ReadDir(dir); err == nil && len(entries) == 0 {
		return os.Remove(dir)
	}

	return nil
}

// colours resolves a palette into the surfaces it names merged with the
// foregrounds every palette shares, plus its accent.
func (tk *tokens) colours(name string) (map[string]string, string, error) {
	p, ok := tk.Palettes[name]
	if !ok {
		return nil, "", fmt.Errorf("no palette %q", name)
	}

	surfaces, ok := tk.Surfaces[p.Surfaces]
	if !ok {
		return nil, "", fmt.Errorf("palette %q names unknown surfaces %q", name, p.Surfaces)
	}

	merged := make(map[string]string, len(surfaces)+len(tk.Foreground))
	for k, v := range surfaces {
		merged[k] = v
	}
	for k, v := range tk.Foreground {
		merged[k] = v
	}

	return merged, p.Accent, nil
}

// allFiles is everything the generator writes: the theme as shipped, plus the
// variant trees the installer picks from.
func allFiles(tk *tokens) (map[string]string, error) {
	out, err := build(tk, defaultPalette, defaultSurfaces, defaultTitlebar, defaultButtons)
	if err != nil {
		return nil, err
	}

	extra, err := variants(tk)
	if err != nil {
		return nil, err
	}
	for path, content := range extra {
		out[path] = content
	}

	return out, nil
}

// variants writes one file per point in each axis that a component resolves at
// install time. The colours are a product of tint and accent because the shell
// reads the theme's own colors file rather than resolving the KDE accent
// through kdeglobals — baking it is correct either way, and costs only ini.
func variants(tk *tokens) (map[string]string, error) {
	out := map[string]string{}

	for name := range tk.Palettes {
		app, shell, err := schemes(tk, name)
		if err != nil {
			return nil, err
		}

		dir := variantDir + "/colors/" + name
		out[dir+"/VanillaBoxDark.colors"] = app
		out[dir+"/colors"] = shell

		// The window decoration is the only artwork that paints a palette colour
		// rather than deferring to the scheme, so it is the only thing a tint
		// generates beyond ini files. It is also the only artwork the titlebar
		// shape reaches, which makes it a product of the two.
		for shape := range tk.DecorationShape {
			deco, err := tk.decoration(name, shape)
			if err != nil {
				return nil, err
			}
			out[variantDir+"/decoration/"+name+"-"+shape+"/decoration.svg"] = deco
		}
	}

	for name, p := range tk.Palettes {
		out[variantDir+"/defaults/"+name+"/defaults"] = lookAndFeelDefaults(p.Accent, "", "")
	}

	for name := range tk.ButtonStyles {
		files, err := tk.titlebarButtons(name)
		if err != nil {
			return nil, err
		}
		for path, content := range files {
			out[variantDir+"/buttons/"+name+"/"+path] = content
		}
	}

	// Both surface shapes are written, the shipped one included. The transparency
	// switches draw their opaque copies out of whichever shape is chosen, so the
	// rounded tree has to exist as a variant and not only as the base.
	for shape := range tk.SurfaceShape {
		files, err := surfaces(tk, defaultPalette, shape)
		if err != nil {
			return nil, err
		}
		for path, content := range files {
			out[variantDir+"/surfaces/"+shape+"/"+path] = content
		}
	}

	return out, nil
}

// surfaces renders the artwork a corner radius reaches: the panel and popup
// frames across all three prefixes, and the stacked control states.
//
// Colours here are only the stylesheet fallbacks an editor shows, so the tint
// makes no difference to the bytes and the default one is used throughout.
func surfaces(tk *tokens, name, shape string) (map[string]string, error) {
	palette, accent, err := tk.colours(name)
	if err != nil {
		return nil, err
	}
	radii, ok := tk.SurfaceShape[shape]
	if !ok {
		return nil, fmt.Errorf("no surface shape %q", shape)
	}

	out := map[string]string{}
	for _, prefix := range []string{"", "opaque/", "solid/"} {
		for path, content := range frames(tk, palette, radii, prefix == "") {
			out[prefix+path] = content
		}
	}

	for path, c := range controls(palette, accent, tk.Status, radii["button"]) {
		out["widgets/"+path] = c.render()
	}

	return out, nil
}

// frames renders the four background surfaces at one radius. translucent picks
// the theme root's opacities; the opaque/ and solid/ prefixes Plasma falls back
// to when compositing is off never carry any.
func frames(tk *tokens, palette map[string]string, radii map[string]float64, translucent bool) map[string]string {
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

	if translucent {
		popup.Opacity = opacity(tk.Opacity["popup"])
		tooltip.Opacity = opacity(tk.Opacity["tooltip"])
		panel.Opacity = opacity(tk.Opacity["panel"])
	}

	return map[string]string{
		"widgets/background.svg":       popup.render(),
		"dialogs/background.svg":       popup.render(),
		"widgets/tooltip.svg":          tooltip.render(),
		"widgets/panel-background.svg": panel.render(),
	}
}

// schemes renders the application colour scheme and the Plasma style's copy for
// one tint and accent. They differ only in how they name themselves.
func schemes(tk *tokens, name string) (app, shell string, err error) {
	palette, accent, err := tk.colours(name)
	if err != nil {
		return "", "", err
	}

	base := scheme{palette: palette, accent: accent, status: tk.Status, Name: "Vanilla Box Dark"}

	a, s := base, base
	a.SchemeKey = "VanillaBoxDark"
	s.SchemeKey = "Vanilla Box Dark"

	return a.render(), s.render(), nil
}

// titlebarButtons renders the four buttons and the layout file for one button
// style. The rc travels with them because its metrics are the button sizes, so
// the two cannot disagree about how big a button is.
//
// Buttons do not multiply by tint: they are painted in the foreground colours,
// and those are held still across every tint.
func (tk *tokens) titlebarButtons(name string) (map[string]string, error) {
	bs, ok := tk.ButtonStyles[name]
	if !ok {
		return nil, fmt.Errorf("no button style %q", name)
	}

	palette, _, err := tk.colours(defaultPalette)
	if err != nil {
		return nil, err
	}

	out := map[string]string{"VanillaBoxDarkrc": auroraeRC(palette, bs, titleHeight)}

	if bs.Kind == "circle" {
		// Maximize and restore are the same light: the button changes what it
		// does, not what it means.
		for file, colour := range map[string]string{
			"close": bs.Close, "minimize": bs.Minimize,
			"maximize": bs.Maximize, "restore": bs.Maximize,
		} {
			out[file+".svg"] = circleButton{
				Radius: bs.CircleRadius, Rest: bs.RestColor, Dim: bs.DimColor,
				Hover: colour, Pressed: bs.PressedOpacity,
			}.render()
		}

		return out, nil
	}

	plain := button{
		PlateFill: palette["text"], HoverOpacity: bs.PlainHover, PressedOpacity: bs.PlainPressed,
		Radius: bs.PlateRadius, GlyphFill: palette["text"], RestOpen: bs.Rest, DimOpacity: bs.Dim,
		GlyphSize: bs.GlyphSize, Box: float64(bs.Width),
	}

	for file, glyph := range map[string]string{
		"minimize": glyphMinimize, "maximize": glyphMaximize, "restore": glyphRestore,
	} {
		b := plain
		b.Glyph = glyph
		out[file+".svg"] = b.render()
	}

	closeBtn := plain
	closeBtn.PlateFill = bs.ClosePlate
	closeBtn.HoverOpacity = bs.CloseHover
	closeBtn.PressedOpacity = bs.ClosePressed
	closeBtn.Glyph = glyphClose
	out["close.svg"] = closeBtn.render()

	return out, nil
}

// decoration renders the window frame for one tint and decoration shape.
func (tk *tokens) decoration(name, decoShape string) (string, error) {
	palette, _, err := tk.colours(name)
	if err != nil {
		return "", err
	}
	deco, ok := tk.DecorationShape[decoShape]
	if !ok {
		return "", fmt.Errorf("no decoration shape %q", decoShape)
	}

	return decoration{
		Width: 40, TitleH: titleHeight + titleBorder, BodyH: 24, Step: 60,
		Radius: deco["titlebar"], Border: palette["elevatedAlt"], Backgnd: palette["background"],
	}.render(), nil
}

// build returns every generated file for one point in the variant space, keyed
// by repository-relative slash path.
func build(tk *tokens, name, shape, decoShape, buttons string) (map[string]string, error) {
	palette, acc, err := tk.colours(name)
	if err != nil {
		return nil, err
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

	// The theme root carries the translucent artwork. Plasma falls back to the
	// opaque/ and solid/ prefixes itself when compositing is off, so both ship
	// whatever the transparency options are set to.
	surf, err := surfaces(tk, name, shape)
	if err != nil {
		return nil, err
	}
	for path, content := range surf {
		out[style+"/"+path] = content
	}

	deco, err := tk.decoration(name, decoShape)
	if err != nil {
		return nil, err
	}
	out[auroraeDir+"/decoration.svg"] = deco

	btns, err := tk.titlebarButtons(buttons)
	if err != nil {
		return nil, err
	}
	for path, content := range btns {
		out[auroraeDir+"/"+path] = content
	}

	out[lookFeel+"/contents/defaults"] = lookAndFeelDefaults(acc, "", "")

	// Identity: the same handful of facts KDE wants in three formats, plus the
	// installer's own copy of the version.
	id := tk.Theme
	out[style+"/metadata.json"] = id.kPluginMetadata(id.StyleID, "X-Plasma-API", "5.0")
	out[lookFeel+"/metadata.json"] = id.kPluginMetadata(
		id.LookAndFeelID, "KPackageStructure", "Plasma/LookAndFeel")
	out[auroraeDir+"/metadata.desktop"] = id.desktopEntry()
	out["internal/theme/version.go"] = id.versionSource()

	return out, nil
}

// controls builds the widget artwork that shares the stacked nine-tile idiom:
// buttons, text fields and list items.
func controls(palette map[string]string, accent string, status map[string]string, radius float64) map[string]control {
	sheet := fmt.Sprintf(
		".ColorScheme-Text { color:%s; }.ColorScheme-Highlight { color:%s; }"+
			".ColorScheme-ButtonBackground { color:%s; }.ColorScheme-ButtonHover { color:%s; }",
		palette["text"], accent, palette["elevated"], status["hover"])

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
