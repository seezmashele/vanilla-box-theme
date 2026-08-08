package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedFilesAreCommitted is the generator's whole safety property: what
// the tokens describe and what the repository ships are the same bytes. It
// fails whenever assets/ is edited by hand instead of through spec/tokens.json,
// which is the mistake the generator exists to make impossible.
func TestGeneratedFilesAreCommitted(t *testing.T) {
	const root = "../.."

	tk, err := loadTokens(filepath.Join(root, "spec", "tokens.json"))
	if err != nil {
		t.Fatalf("loadTokens: %v", err)
	}

	ic, err := loadIcons(root)
	if err != nil {
		t.Fatalf("loadIcons: %v", err)
	}

	files, err := allFiles(tk, ic)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("the generator produced nothing")
	}

	for path, want := range files {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("%s: %v — run go generate ./...", path, err)

			continue
		}

		if string(got) != want {
			t.Errorf("%s is out of date with spec/tokens.json — run go generate ./...", path)
		}
	}
}

// TestSquareSurfacesDropTheirCurves checks the axis the generator was built for
// before any variant depends on it: at radius zero the corner tiles must be
// plain lines, or a square variant would ship rounded artwork.
func TestSquareSurfacesDropTheirCurves(t *testing.T) {
	square := frame{
		Size: 44, Canvas: 60, Tile: 10, Radius: 0,
		Fallback: "#292929", Mask: true, HintSize: 8, HintY: 48,
	}

	svg := square.render()

	if want := `d="M0,10 L0,0 L10,0 L10,10 Z"`; !strings.Contains(svg, want) {
		t.Errorf("radius 0 should emit a plain corner tile, want %s in:\n%s", want, firstTile(svg))
	}
	if strings.Contains(svg, " C") {
		t.Error("radius 0 emitted a curve command")
	}
}

func firstTile(svg string) string {
	return element(svg, "topleft")
}

// element returns the one line of the sheet carrying the given tile id.
func element(svg, id string) string {
	start := strings.Index(svg, `id="`+id+`"`)
	if start < 0 {
		return ""
	}
	start = strings.LastIndex(svg[:start], "<")

	return svg[start : start+strings.Index(svg[start:], "\n")]
}

// outlined is the tooltip's frame: the one container that carries a border.
func outlined() frame {
	return frame{
		Size: 44, Canvas: 60, Tile: 10, Radius: 8,
		Fallback: "#2f2f2f", Mask: true, HintSize: 4, HintY: 48,
		Border: "0.1", BorderFallback: "#e8e4dd", Shadow: true,
	}
}

// TestShadowPrefixIsPresentAndEmpty is what keeps the tooltip to one border.
//
// org.kde.plasma.components.ToolTip draws this sheet twice, once with prefix
// "shadow" for the drop shadow and once plain. KSvg decides a prefix exists by
// looking for <prefix>-center and clears the prefix when it finds none, so a
// sheet with no shadow tiles draws its own frame a second time, inflated by the
// margins — which is one border while the frame is a flat fill and two the
// moment it has an outline.
func TestShadowPrefixIsPresentAndEmpty(t *testing.T) {
	svg := outlined().render()

	// The probe KSvg uses. Everything else here depends on it being found.
	if !strings.Contains(svg, `id="shadow-center"`) {
		t.Fatal("no shadow-center: KSvg would clear the prefix and draw the frame twice")
	}

	for _, name := range []string{
		"topleft", "topright", "bottomleft", "bottomright",
		"top", "bottom", "left", "right", "center",
	} {
		el := element(svg, "shadow-"+name)
		if el == "" {
			t.Errorf("no shadow-%s in the sheet", name)

			continue
		}
		if !strings.Contains(el, `style="fill:none"`) {
			t.Errorf("shadow-%s paints something, so it would show as a second border:\n%s", name, el)
		}
	}

	// The prefix reports the frame's own insets, so adding it does not move the
	// tooltip: these are the margins KSvg already derived from the fallback.
	for _, side := range []string{"top", "bottom", "left", "right"} {
		if !strings.Contains(svg, `id="shadow-hint-`+side+`-margin"`) {
			t.Errorf("no shadow-hint-%s-margin", side)
		}
	}
}

// TestOutlineIsConcentricWithTheSurface covers how the border is built: the
// surface is the whole tile and the outline is a ring between that shape and
// the same shape a pixel in, so the two curves stay parallel. Drawing the inner
// path at the outer radius would leave the border thicker at the corners than
// along the edges.
func TestOutlineIsConcentricWithTheSurface(t *testing.T) {
	svg := outlined().render()
	tile := element(svg, "topleft")

	// Radius 8 at the outer edge, 7 a pixel in — and cornerFactor puts the
	// inner control point at 1+7*0.4478.
	outer := `M0,10 L0,8 C0,3.582 3.582,0 8,0 L10,0 L10,10 Z`
	inner := `M1,10 L1,8 C1,4.135 4.135,1 8,1 L10,1 L10,10 Z`

	if !strings.Contains(tile, fmt.Sprintf(`d="%s" class="ColorScheme-Background"`, outer)) {
		t.Errorf("surface should cover the whole corner, want %s in:\n%s", outer, tile)
	}
	// Both subpaths in one d, so even-odd drops the inner shape out and leaves a
	// ring. Two separate paths would paint the inner shape rather than cut it.
	if !strings.Contains(tile, fmt.Sprintf(`d="%s %s" fill-rule="evenodd"`, outer, inner)) {
		t.Errorf("outline should be a ring a pixel inside it, want %s in:\n%s", inner, tile)
	}
	// The outline goes through the stylesheet so it follows the tint rather than
	// baking one palette's border into artwork every palette shares.
	if !strings.Contains(svg, `.ColorScheme-Text { color:#e8e4dd; }`) {
		t.Errorf("the outline's class should be declared in the stylesheet:\n%s", svg)
	}
}

// TestOutlineIsPaintedOverTheSurface is the bug the concentricity test could not
// see. The outline is a tenth of the text colour, so it is only a border while
// it has the surface underneath it to be a tenth of. Painted first, with the
// background inset over it, the frame's outermost pixel is ninety percent
// transparent and the border reads as a gap onto whatever is behind the tooltip.
//
// The window decoration does stack them the other way round, and that is where
// this went wrong: its border is an opaque literal, so the order does not matter
// to it.
func TestOutlineIsPaintedOverTheSurface(t *testing.T) {
	svg := outlined().render()

	// The corners carry the ring and the edges a plain strip, so check one of
	// each: in both the surface has to be the first shape in the group.
	for _, id := range []string{"topleft", "topright", "bottomleft", "bottomright", "top", "bottom", "left", "right"} {
		tile := element(svg, id)
		surface := strings.Index(tile, "ColorScheme-Background")
		outline := strings.Index(tile, "ColorScheme-Text")

		switch {
		case surface < 0:
			t.Errorf("%s paints no surface under its outline:\n%s", id, tile)
		case outline < 0:
			t.Errorf("%s carries no outline:\n%s", id, tile)
		case outline < surface:
			t.Errorf("%s paints its outline under the surface, so the border is transparent:\n%s", id, tile)
		}
	}
}

// TestOutlineLeavesTheTileSeamsBare keeps the border to the frame's outer edge.
// Every tile is drawn against its neighbours, so an outline on a shared boundary
// would draw a line across the middle of the tooltip.
func TestOutlineLeavesTheTileSeamsBare(t *testing.T) {
	svg := outlined().render()

	// The centre is interior on all four sides and never carries an outline.
	if tile := element(svg, "center"); strings.Contains(tile, "ColorScheme-Text") {
		t.Errorf("the centre carries an outline:\n%s", tile)
	}
	// A strip's outline is one pixel on the frame's edge, not the whole tile:
	// top spans the full 10px tile and its border only the first row.
	tile := element(svg, "top")
	if !strings.Contains(tile, `<rect x="10" y="0" width="24" height="10" class="ColorScheme-Background"`) {
		t.Errorf("top's surface should cover the whole tile:\n%s", tile)
	}
	if !strings.Contains(tile, `<rect x="10" y="0" width="24" height="1" class="ColorScheme-Text"`) {
		t.Errorf("top's outline should be one pixel on the frame's edge:\n%s", tile)
	}
}

// TestOutlineLeavesTheMaskWhole is the half of the border that is invisible
// until it is wrong: the mask is the blur region, not artwork, so an outline
// drawn into it would cut a ring out of the blur instead of showing as a
// border.
func TestOutlineLeavesTheMaskWhole(t *testing.T) {
	svg := outlined().render()

	for _, id := range []string{"mask-topleft", "mask-top", "mask-center"} {
		tile := element(svg, id)
		if tile == "" {
			t.Errorf("no %s in the sheet", id)

			continue
		}
		if strings.Contains(tile, "ColorScheme-Text") {
			t.Errorf("%s carries the outline:\n%s", id, tile)
		}
	}
}

// TestUnoutlinedFramesAreUntouched pins the panel and popup artwork against the
// border work: they share the corner builders with the tooltip, and a frame
// with no outline must still emit one flat shape per tile.
func TestUnoutlinedFramesAreUntouched(t *testing.T) {
	plain := outlined()
	plain.Border, plain.BorderFallback = "", ""

	tile := element(plain.render(), "topleft")

	want := `<g id="topleft"><path d="M0,10 L0,8 C0,3.582 3.582,0 8,0 L10,0 L10,10 Z" class="ColorScheme-Background" style="fill:currentColor"/></g>`
	if tile != want {
		t.Errorf("a frame with no outline changed shape:\n got %s\nwant %s", tile, want)
	}
}

// TestSidebarMovesOnlyTheWindowBackground pins what the sidebar choice is
// allowed to touch. KColorScheme has no sidebar role — a places panel paints
// with the window background — so the option works by moving that one role, and
// the risk is it dragging the rest of the scheme with it.
//
// Header staying put is the point: a sidebar merged with the file list still
// wants a toolbar above it that is not.
func TestSidebarMovesOnlyTheWindowBackground(t *testing.T) {
	tk := testTokens(t)

	windowed, err := appScheme(tk, defaultPalette, sidebarWindow)
	if err != nil {
		t.Fatalf("appScheme: %v", err)
	}
	viewed, err := appScheme(tk, defaultPalette, sidebarView)
	if err != nil {
		t.Fatalf("appScheme: %v", err)
	}

	assertWindowBackgroundMove(t, tk, "sidebar", windowed, viewed)
}

// TestPanelTintMovesOnlyTheShell is the same pinning for the shell's copy of
// the scheme, which asks its own question: the panel, the launcher and applet
// popups all resolve ColorScheme-Background against [Colors:Window], so they
// move together and nothing else may.
func TestPanelTintMovesOnlyTheShell(t *testing.T) {
	tk := testTokens(t)

	chrome, err := shellScheme(tk, defaultPalette, panelChrome)
	if err != nil {
		t.Fatalf("shellScheme: %v", err)
	}
	dark, err := shellScheme(tk, defaultPalette, panelDark)
	if err != nil {
		t.Fatalf("shellScheme: %v", err)
	}

	assertWindowBackgroundMove(t, tk, "panel-tint", chrome, dark)
}

// TestTheTwoSchemesTakeSeparateAxes is the decoupling itself. The sidebar
// question is about a dock panel in a Qt application and the panel question is
// about the desktop; they move the same role in two different files, and the
// files have to be able to disagree. Sharing a flag once made choosing the
// merged sidebar darken the panel as a side effect.
func TestTheTwoSchemesTakeSeparateAxes(t *testing.T) {
	tk := testTokens(t)

	for _, palette := range []string{defaultPalette, "forest"} {
		app := make([]string, 0, 2)
		for _, sidebar := range []string{sidebarWindow, sidebarView} {
			s, err := appScheme(tk, palette, sidebar)
			if err != nil {
				t.Fatalf("appScheme: %v", err)
			}
			app = append(app, s)
		}

		shell := make([]string, 0, 2)
		for _, panel := range []string{panelChrome, panelDark} {
			s, err := shellScheme(tk, palette, panel)
			if err != nil {
				t.Fatalf("shellScheme: %v", err)
			}
			shell = append(shell, s)
		}

		// Each axis moves its own file, and the manifest resolves the two from
		// separate trees, so neither can reach the other's.
		if app[0] == app[1] {
			t.Errorf("%s: the sidebar choice leaves the application scheme unchanged", palette)
		}
		if shell[0] == shell[1] {
			t.Errorf("%s: the panel choice leaves the shell scheme unchanged", palette)
		}
	}
}

// TestTooltipSitsOffThePopupBackground guards the surface the tooltip is for.
// It is the one container deliberately on the view colour, so that it reads as
// a dark card over whatever it covers rather than as one more shade of it —
// which stops being true the moment the popups it appears over are the view
// colour too.
func TestTooltipSitsOffThePopupBackground(t *testing.T) {
	tk := testTokens(t)

	shipped, err := shellScheme(tk, defaultPalette, defaultPanel)
	if err != nil {
		t.Fatalf("shellScheme: %v", err)
	}

	popup, tooltip := background(shipped, "Window"), background(shipped, "Tooltip")
	if popup == tooltip {
		t.Errorf("popups and tooltips both paint %s, so a tooltip over a popup "+
			"shows only its outline", popup)
	}
}

// assertWindowBackgroundMove checks one scheme rendered at both ends of an axis
// that moves the window background. Both axes move the same role in the same
// way, so they are worth holding to one table: what varies between them is only
// which file is asking.
func assertWindowBackgroundMove(t *testing.T, tk *tokens, axis, held, moved string) {
	t.Helper()

	surfaces := tk.Surfaces[tk.Palettes[defaultPalette].Surfaces]
	chrome, view := rgb(surfaces["background"]), rgb(surfaces["view"])

	for _, tc := range []struct {
		section     string
		held, moved string
		description string
	}{
		{"Window", chrome, view, "the role the choice paints with"},
		{"Complementary", chrome, view, "follows Window so the two cannot disagree"},
		{"Header", chrome, chrome, "the toolbar stays on the chrome colour"},
		{"View", view, view, "already the view colour; the option is what meets it"},
		{"Tooltip", view, view, "a card over the surface, not the surface"},
	} {
		if got := background(moved, tc.section); got != tc.moved {
			t.Errorf("%s moved: [Colors:%s] BackgroundNormal = %s, want %s (%s)",
				axis, tc.section, got, tc.moved, tc.description)
		}
		if got := background(held, tc.section); got != tc.held {
			t.Errorf("%s held: [Colors:%s] BackgroundNormal = %s, want %s (%s)",
				axis, tc.section, got, tc.held, tc.description)
		}
	}
}

// testTokens loads the theme's own tokens, which is what every test that cares
// about a colour measures against.
func testTokens(t *testing.T) *tokens {
	t.Helper()

	tk, err := loadTokens(filepath.Join("../..", "spec", "tokens.json"))
	if err != nil {
		t.Fatalf("loadTokens: %v", err)
	}

	return tk
}

// background reads one section's BackgroundNormal out of a rendered scheme.
func background(ini, section string) string {
	rest, ok := strings.CutPrefix(ini[strings.Index(ini, "[Colors:"+section+"]"):], "[Colors:"+section+"]\n")
	if !ok {
		return ""
	}

	for _, line := range strings.Split(rest, "\n") {
		if v, found := strings.CutPrefix(line, "BackgroundNormal="); found {
			return v
		}
		if strings.HasPrefix(line, "[") {
			break
		}
	}

	return ""
}

// TestEveryMappedSourceIsVendored is the failure that would otherwise wait
// until someone else ran the generator: a mapping is a line of JSON, and the
// artwork it names has to be fetched separately and committed.
func TestEveryMappedSourceIsVendored(t *testing.T) {
	const root = "../.."

	ic, err := loadIcons(root)
	if err != nil {
		t.Fatalf("loadIcons: %v", err)
	}

	names, err := ic.names()
	if err != nil {
		t.Fatalf("names: %v", err)
	}

	if len(names) == 0 {
		t.Fatal("the mapping is empty")
	}

	for name, phosphor := range names {
		if ic.sources[phosphor] == "" {
			t.Errorf("%s maps to %s, which is not vendored", name, phosphor)
		}
	}
}

// TestIconsArePaintedOrDefer pins the fork, in both directions, because the two
// halves undo each other: an icon that carries a colour and the ColorScheme
// class would have the colour replaced at load time and look like the colour
// had never been set. Whichever way the spec goes, only one may reach the file.
func TestIconsArePaintedOrDefer(t *testing.T) {
	painted := icon{Paths: []string{`<path d="M0,0"/>`}, Color: "#ebebeb", Fallback: "#e8e4dd"}.render()

	if !strings.Contains(painted, `<g fill="#ebebeb"`) {
		t.Errorf("a painted icon does not carry its colour:\n%s", painted)
	}

	for _, unwanted := range []string{"current-color-scheme", "ColorScheme-Text", "currentColor"} {
		if strings.Contains(painted, unwanted) {
			t.Errorf("a painted icon still defers through %s, which would overwrite it:\n%s", unwanted, painted)
		}
	}

	// Leaving the colour out is the KDE default, and stays reachable.
	deferred := icon{Paths: []string{`<path d="M0,0"/>`}, Fallback: "#e8e4dd"}.render()

	for _, want := range []string{
		`id="current-color-scheme"`,
		`class="ColorScheme-Text"`,
		`fill="currentColor"`,
		`.ColorScheme-Text { color:#e8e4dd; }`,
	} {
		if !strings.Contains(deferred, want) {
			t.Errorf("an icon with no colour of its own is missing %s:\n%s", want, deferred)
		}
	}
}

// TestBatteryFamilyExpands checks the rule that stands in for a hundred
// filenames. The charging override is the interesting half: it has to beat the
// level's icon, while a power profile changes only the name.
func TestBatteryFamilyExpands(t *testing.T) {
	const root = "../.."

	ic, err := loadIcons(root)
	if err != nil {
		t.Fatalf("loadIcons: %v", err)
	}

	names, err := ic.names()
	if err != nil {
		t.Fatalf("names: %v", err)
	}

	for name, want := range map[string]string{
		"status/battery-000":                            "battery-empty",
		"status/battery-100":                            "battery-full",
		"status/battery-050-profile-balanced":           "battery-medium",
		"status/battery-050-charging":                   "battery-charging",
		"status/battery-100-charging-profile-powersave": "battery-charging",
		"status/network-wireless-0":                     "wifi-none",
		"status/network-wireless-connected-100":         "wifi-high",
		"status/network-wireless-60-limited":            "wifi-x",
	} {
		if got := names[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// TestGlyphSizeIsWhatItSays checks the arithmetic between the pixels asked for
// and the transform that delivers them. The spec names a glyph size because
// that is the decision; everything downstream is a factor nobody should have to
// verify by rasterising an icon and measuring it.
func TestGlyphSizeIsWhatItSays(t *testing.T) {
	const root = "../.."

	ic, err := loadIcons(root)
	if err != nil {
		t.Fatalf("loadIcons: %v", err)
	}

	// What an icon filling Phosphor's grid measures once scaled, in pixels. The
	// tolerance is what rounding the scale to three decimals can cost: half a
	// unit in its last place, across the unscaled glyph.
	const tolerance = 0.0005 * iconSize * iconGrid / iconBox

	got := iconSize * iconGrid / iconBox * ic.glyphScale()
	if diff := got - ic.Glyph; diff > tolerance || diff < -tolerance {
		t.Errorf("a full glyph measures %.3fpx, want the %vpx spec/icons.json asks for", got, ic.Glyph)
	}

	// Centred: half the freed space on each side. Pinned as the rendered string
	// because an off-centre transform still looks plausible in isolation and
	// only shows up as icons sitting high and left in a panel.
	svg := icon{Paths: []string{`<path d="M0,0"/>`}, Fallback: "#e8e4dd", Scale: 0.895}.render()

	if want := `transform="translate(13.44,13.44) scale(0.895)"`; !strings.Contains(svg, want) {
		t.Errorf("want %s in:\n%s", want, svg)
	}

	// An unscaled icon carries no transform at all rather than an identity one.
	for _, scale := range []float64{0, 1} {
		plain := icon{Paths: []string{`<path d="M0,0"/>`}, Scale: scale}.render()
		if strings.Contains(plain, "transform") {
			t.Errorf("scale %v should emit no transform:\n%s", scale, plain)
		}
	}
}

// TestSwatchesMatchTheSurfaces keeps the two hand-written copies of a colour in
// step. The manifest carries swatches so the preferences screen can draw the
// palette it is offering, and spec/tokens.json is where those colours actually
// come from — a swatch that drifts shows the user one colour and installs
// another, which is worse than showing none.
//
// The pair is the panel background and the elevated surface, in that order:
// what the desktop is mostly made of, and the shade sitting on top of it.
func TestSwatchesMatchTheSurfaces(t *testing.T) {
	const root = "../.."

	tk, err := loadTokens(filepath.Join(root, "spec", "tokens.json"))
	if err != nil {
		t.Fatalf("loadTokens: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "assets", "theme.json"))
	if err != nil {
		t.Fatal(err)
	}

	var manifest struct {
		Components []struct {
			Options []struct {
				ID     string `json:"id"`
				Values []struct {
					ID     string   `json:"id"`
					Swatch []string `json:"swatch"`
				} `json:"values"`
			} `json:"options"`
		} `json:"components"`
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}

	var seen int

	for _, c := range manifest.Components {
		for _, o := range c.Options {
			if o.ID != "palette" {
				continue
			}

			for _, v := range o.Values {
				p, ok := tk.Palettes[v.ID]
				if !ok {
					t.Errorf("theme.json offers palette %q, which spec/tokens.json does not define", v.ID)

					continue
				}

				seen++

				surfaces, ok := tk.Surfaces[p.Surfaces]
				if !ok {
					t.Errorf("palette %q names unknown surfaces %q", v.ID, p.Surfaces)

					continue
				}

				want := []string{surfaces["background"], surfaces["elevatedAlt"]}
				if len(v.Swatch) != len(want) {
					t.Errorf("%s has %d swatches, want %d", v.ID, len(v.Swatch), len(want))

					continue
				}

				for i, got := range v.Swatch {
					if got != want[i] {
						t.Errorf("%s swatch %d is %s but its surface is %s — "+
							"edit spec/tokens.json and assets/theme.json together", v.ID, i, got, want[i])
					}
				}
			}
		}
	}

	if seen != len(tk.Palettes) {
		t.Errorf("the manifest offers %d palettes, spec/tokens.json defines %d", seen, len(tk.Palettes))
	}
}

// TestManifestDefaultsMatchTheShippedArtwork ties the option the installer
// preselects to the variant the generator bakes into the shipped tree. They are
// two separate statements of the same intent and they drifted once already: the
// style shipped square containers while theme.json preselected "rounded", so a
// default install laid the rounded overlay over a square base and the constant
// here described a default nobody received.
//
// Where a value carries no overlay the drift is not even redundant work. The
// titlebar is resolved from {palette}-{titlebar}, so its defaultValue is the
// only thing that decides what a fresh install renders.
func TestManifestDefaultsMatchTheShippedArtwork(t *testing.T) {
	const root = "../.."

	data, err := os.ReadFile(filepath.Join(root, "assets", "theme.json"))
	if err != nil {
		t.Fatal(err)
	}

	var manifest struct {
		Components []struct {
			Options []struct {
				ID           string `json:"id"`
				DefaultValue string `json:"defaultValue"`
			} `json:"options"`
		} `json:"components"`
	}

	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"palette":    defaultPalette,
		"sidebar":    defaultSidebar,
		"panel-tint": defaultPanel,
		"containers": defaultContainers,
		"elements":   defaultElements,
		"titlebar":   defaultTitlebar,
		"buttons":    defaultButtons,
	}

	seen := map[string]bool{}

	for _, c := range manifest.Components {
		for _, o := range c.Options {
			w, ok := want[o.ID]
			if !ok {
				continue
			}

			seen[o.ID] = true

			if o.DefaultValue != w {
				t.Errorf("theme.json preselects %q for %s but the generator ships %q — "+
					"edit assets/theme.json and internal/gen/main.go together", o.DefaultValue, o.ID, w)
			}
		}
	}

	for id := range want {
		if !seen[id] {
			t.Errorf("theme.json declares no %s option", id)
		}
	}
}
