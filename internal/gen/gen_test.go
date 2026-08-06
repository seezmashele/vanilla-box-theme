package main

import (
	"encoding/json"
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
// outline is the whole tile and the background is the same shape a pixel in, so
// the two curves stay parallel. Drawing the inner path at the outer radius
// would leave the border thicker at the corners than along the edges.
func TestOutlineIsConcentricWithTheSurface(t *testing.T) {
	svg := outlined().render()
	tile := element(svg, "topleft")

	// Radius 8 at the outer edge, 7 a pixel in — and cornerFactor puts the
	// inner control point at 1+7*0.4478.
	outer := `d="M0,10 L0,8 C0,3.582 3.582,0 8,0 L10,0 L10,10 Z"`
	inner := `d="M1,10 L1,8 C1,4.135 4.135,1 8,1 L10,1 L10,10 Z"`

	if !strings.Contains(tile, outer) {
		t.Errorf("outline should cover the whole corner, want %s in:\n%s", outer, tile)
	}
	if !strings.Contains(tile, inner) {
		t.Errorf("surface should sit a pixel inside it, want %s in:\n%s", inner, tile)
	}
	// The outline goes through the stylesheet so it follows the tint rather than
	// baking one palette's border into artwork every palette shares.
	if !strings.Contains(svg, `.ColorScheme-Text { color:#e8e4dd; }`) {
		t.Errorf("the outline's class should be declared in the stylesheet:\n%s", svg)
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
