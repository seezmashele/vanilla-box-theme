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
	start := strings.Index(svg, `<g id="topleft"`)
	if start < 0 {
		return svg
	}

	return svg[start : start+strings.Index(svg[start:], "\n")]
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
