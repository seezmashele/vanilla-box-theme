package theme

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// installFixture builds a theme whose one component is a small package with an
// "opaque" overlay, mirroring how the real Plasma style is laid out, and points
// the data directory somewhere disposable.
func installFixture(t *testing.T) (*Theme, Component) {
	t.Helper()

	assets := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	writeFile(t, filepath.Join(assets, "style", "metadata.json"), "{}\n")
	writeFile(t, filepath.Join(assets, "style", "widgets", "panel.svg"), "translucent\n")
	writeFile(t, filepath.Join(assets, "style", "widgets", "tasks.svg"), "tasks\n")
	writeFile(t, filepath.Join(assets, "style", "opaque", "widgets", "panel.svg"), "opaque\n")

	component := Component{
		ID: "style", Name: "Plasma style", Source: "style",
		Target: "plasma/desktoptheme",
		Options: []Option{{
			ID: "transparency", Name: "Transparency", Kind: KindToggle,
			Default: true,
			OverlayWhenOff: Overlay{
				From:  "style/opaque",
				Files: []string{"widgets/panel.svg"},
			},
		}},
	}

	return &Theme{AssetDir: assets, Stamp: "20260730-120000"}, component
}

// on returns choices with every named toggle set.
func on(ids ...string) Choices {
	ch := NewChoices()
	for _, id := range ids {
		ch.Toggles[id] = true
	}

	return ch
}

func TestInstallCopiesTheComponent(t *testing.T) {
	theme, component := installFixture(t)

	if err := theme.Install(component, on("transparency")); err != nil {
		t.Fatalf("Install: %v", err)
	}

	dst := theme.TargetPath(component)
	for path, want := range map[string]string{
		"metadata.json":            "{}\n",
		"widgets/panel.svg":        "translucent\n",
		"widgets/tasks.svg":        "tasks\n",
		"opaque/widgets/panel.svg": "opaque\n",
	} {
		if got := readFile(t, filepath.Join(dst, filepath.FromSlash(path))); got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
}

// TestInstallOverlaysWhenOptionIsOff is the whole point of the option: the
// theme's own opaque variant is laid over the translucent one.
func TestInstallOverlaysWhenOptionIsOff(t *testing.T) {
	theme, component := installFixture(t)

	if err := theme.Install(component, NewChoices()); err != nil {
		t.Fatalf("Install: %v", err)
	}

	dst := theme.TargetPath(component)

	if got := readFile(t, filepath.Join(dst, "widgets", "panel.svg")); got != "opaque\n" {
		t.Errorf("panel.svg = %q, want the opaque version", got)
	}

	// Files the overlay does not mention are left alone. tasks.svg is the real
	// case: its translucency is deliberate and opaque/ leaves it out.
	if got := readFile(t, filepath.Join(dst, "widgets", "tasks.svg")); got != "tasks\n" {
		t.Errorf("tasks.svg = %q, want it untouched by the overlay", got)
	}
}

func TestInstallBacksUpAnExistingInstall(t *testing.T) {
	theme, component := installFixture(t)

	if err := theme.Install(component, on("transparency")); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	// Something the user left behind, which must survive in the backup rather
	// than be silently overwritten.
	dst := theme.TargetPath(component)
	writeFile(t, filepath.Join(dst, "widgets", "custom.svg"), "mine\n")

	if err := theme.Install(component, on("transparency")); err != nil {
		t.Fatalf("second Install: %v", err)
	}

	backup := filepath.Join(
		os.Getenv("XDG_DATA_HOME"), filepath.FromSlash(BackupDir),
		theme.Stamp, filepath.FromSlash(component.Target), "style",
	)
	if got := readFile(t, filepath.Join(backup, "widgets", "custom.svg")); got != "mine\n" {
		t.Errorf("the previous install should be under %s, got %q", backup, got)
	}

	// And the replacement is clean: the stray file is gone from the live copy.
	if _, err := os.Stat(filepath.Join(dst, "widgets", "custom.svg")); !os.IsNotExist(err) {
		t.Error("a replaced install should not keep files the theme no longer ships")
	}
}

// TestInstallWritesResolvedFiles covers the files that depend on more than one
// option. Layering them as overlays would make the order they are applied in
// decide the result; naming the combination in the path does not.
func TestInstallWritesResolvedFiles(t *testing.T) {
	theme, component := installFixture(t)

	writeFile(t, filepath.Join(theme.AssetDir, "variants", "rc", "square-mac", "rc"), "square+mac\n")
	writeFile(t, filepath.Join(theme.AssetDir, "variants", "rc", "rounded-mac", "rc"), "rounded+mac\n")

	component.Resolved = []Resolved{{
		Source: "variants/rc/{decoration}-{buttons}/rc",
		Target: "themerc",
	}}

	choices := on("transparency")
	choices.Values["decoration"] = "square"
	choices.Values["buttons"] = "mac"

	if err := theme.Install(component, choices); err != nil {
		t.Fatalf("Install: %v", err)
	}

	got := readFile(t, filepath.Join(theme.TargetPath(component), "themerc"))
	if got != "square+mac\n" {
		t.Errorf("themerc = %q, want the square+mac combination", got)
	}
}

// TestInstallReportsAnUnfillablePlaceholder checks the failure names the option
// rather than the path it produced, which is a step too late to be useful.
func TestInstallReportsAnUnfillablePlaceholder(t *testing.T) {
	theme, component := installFixture(t)

	component.Resolved = []Resolved{{Source: "variants/{nosuch}/rc", Target: "themerc"}}

	err := theme.Install(component, on("transparency"))
	if err == nil {
		t.Fatal("Install succeeded with an unfillable placeholder, want an error")
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error should name the missing option, got %v", err)
	}
}

func TestInstallReportsAMissingSource(t *testing.T) {
	theme, _ := installFixture(t)

	err := theme.Install(Component{ID: "gone", Name: "Gone", Source: "nowhere", Target: "x"}, NewChoices())
	if err == nil {
		t.Fatal("Install succeeded with a missing source, want an error")
	}
	if !strings.Contains(err.Error(), "nowhere") {
		t.Errorf("error should name the missing source, got %v", err)
	}
}

// TestShippedThemeInstalls drives the real asset tree through a real install,
// into a temporary data directory. It is the end-to-end check that the artwork
// and the manifest agree with each other.
func TestShippedThemeInstalls(t *testing.T) {
	for _, transparency := range []bool{true, false} {
		t.Run("transparency="+onOff(transparency), func(t *testing.T) {
			theme, style := installShipped(t, func(ch Choices) {
				for id := range ch.Toggles {
					ch.Toggles[id] = transparency
				}
			})

			translucent := strings.Contains(
				readFile(t, filepath.Join(style, filepath.FromSlash("widgets/panel-background.svg"))),
				"fill-opacity:0.85",
			)
			if translucent != transparency {
				t.Errorf("panel translucent = %v, want %v", translucent, transparency)
			}

			// The task manager never follows the option, because its translucency
			// is white highlight artwork rather than a background.
			tasks := readFile(t, filepath.Join(style, "widgets", "tasks.svg"))
			if !strings.Contains(tasks, "fill-opacity:0.3") {
				t.Error("tasks.svg should keep its highlight opacity whatever the option")
			}

			// The look-and-feel previews are the largest files in the theme, and
			// the easiest thing for a copy to quietly truncate.
			preview := filepath.Join(
				theme.TargetPath(theme.Components[3]), "contents", "previews", "preview.png",
			)
			if info, err := os.Stat(preview); err != nil || info.Size() < 100_000 {
				t.Errorf("preview.png: stat %v, size %v", err, info)
			}
		})
	}
}

// TestTransparencyTogglesActIndependently is what the split bought: turning one
// surface opaque must leave the other two alone. A whole-directory overlay
// could not do this without a directory per combination.
func TestTransparencyTogglesActIndependently(t *testing.T) {
	surfaces := map[string]string{
		"transparency-panel":   "widgets/panel-background.svg",
		"transparency-popups":  "dialogs/background.svg",
		"transparency-applets": "widgets/background.svg",
	}

	for off := range surfaces {
		t.Run(off, func(t *testing.T) {
			_, style := installShipped(t, func(ch Choices) {
				for id := range ch.Toggles {
					ch.Toggles[id] = true
				}
				ch.Toggles[off] = false
			})

			for id, file := range surfaces {
				body := readFile(t, filepath.Join(style, filepath.FromSlash(file)))
				translucent := strings.Contains(body, "fill-opacity:0.85")

				if want := id != off; translucent != want {
					t.Errorf("with %s off, %s translucent = %v, want %v", off, file, translucent, want)
				}
			}
		})
	}
}

// TestPaletteCarriesItsAccent checks the merged colour axis: one choice has to
// reach the surfaces, the shell's highlight, the KDE accent and the titlebar
// paint together. They were separate options once, and the risk in joining them
// is that one of the four quietly keeps the default.
func TestPaletteCarriesItsAccent(t *testing.T) {
	palettes := map[string]struct{ surface, accent string }{
		"neutral": {"41,41,41", "174,142,108"},
		"ash":     {"41,41,41", "143,143,143"},
		"slate":   {"39,42,47", "125,147,173"},
		"moss":    {"39,42,39", "140,168,130"},
		"rose":    {"43,39,39", "184,119,106"},
		"plum":    {"43,39,45", "162,136,176"},
	}

	for name, want := range palettes {
		t.Run(name, func(t *testing.T) {
			theme, style := installShipped(t, func(ch Choices) {
				ch.Values["palette"] = name
			})

			shell := readFile(t, filepath.Join(style, "colors"))
			if !strings.Contains(shell, "BackgroundNormal="+want.surface) {
				t.Errorf("shell colors missing the %s surface %s", name, want.surface)
			}
			if !strings.Contains(shell, "[Colors:Selection]\nBackgroundAlternate="+want.accent) {
				t.Errorf("shell colors missing the %s accent %s", name, want.accent)
			}

			app := readFile(t, theme.TargetPath(theme.Components[0]))
			if !strings.Contains(app, "BackgroundNormal="+want.surface) ||
				!strings.Contains(app, "[Colors:Selection]\nBackgroundAlternate="+want.accent) {
				t.Error("the application scheme disagrees with the shell's copy")
			}

			defaults := readFile(t, filepath.Join(
				theme.TargetPath(theme.Components[3]), "contents", "defaults"))
			if !strings.Contains(defaults, "AccentColor="+want.accent) {
				t.Errorf("look-and-feel defaults missing AccentColor=%s", want.accent)
			}

			deco := readFile(t, filepath.Join(
				theme.TargetPath(theme.Components[2]), "decoration.svg"))
			if !strings.Contains(deco, hexOf(want.surface)) {
				t.Errorf("decoration.svg is not painted in the %s surface", name)
			}
		})
	}
}

// TestNeutralAndAshDifferOnlyInTheirAccent is the pair that makes the merged
// axis worth having: identical surfaces, and the whole difference is whether
// anything on screen is coloured. If the accent stopped reaching the scheme,
// the two would become the same theme and nothing else would notice.
func TestNeutralAndAshDifferOnlyInTheirAccent(t *testing.T) {
	read := func(name string) string {
		_, style := installShipped(t, func(ch Choices) { ch.Values["palette"] = name })

		return readFile(t, filepath.Join(style, "colors"))
	}

	neutral, ash := read("neutral"), read("ash")

	if neutral == ash {
		t.Fatal("neutral and ash produced the same colour scheme")
	}
	if !strings.Contains(neutral, "BackgroundNormal=41,41,41") ||
		!strings.Contains(ash, "BackgroundNormal=41,41,41") {
		t.Error("neutral and ash should share the grey surfaces")
	}
	if !strings.Contains(ash, "[Colors:Selection]\nBackgroundAlternate=143,143,143") {
		t.Error("ash should have no colour anywhere, including its selection")
	}
}

// TestSquareSurfacesSurviveTheTransparencySwitches is the interaction the
// placeholder in an overlay path exists for. The square variant is laid down
// first; turning a surface opaque then copies that one file again, and it has
// to come from the square tree. Drawing it from a fixed path would quietly put
// rounded corners back on exactly the surfaces the user made opaque.
func TestSquareSurfacesSurviveTheTransparencySwitches(t *testing.T) {
	rounded := map[string]string{
		"widgets/panel-background.svg": "transparency-panel",
		"dialogs/background.svg":       "transparency-popups",
		"widgets/background.svg":       "transparency-applets",
	}

	for file, toggle := range rounded {
		t.Run(toggle, func(t *testing.T) {
			_, style := installShipped(t, func(ch Choices) {
				ch.Values["surfaces"] = "square"
				ch.Toggles[toggle] = false
			})

			body := readFile(t, filepath.Join(style, filepath.FromSlash(file)))

			if strings.Contains(body, " C") {
				t.Errorf("%s kept a curve command after being made opaque", file)
			}
			if strings.Contains(body, "fill-opacity") {
				t.Errorf("%s should be opaque with %s off", file, toggle)
			}
		})
	}
}

// TestSurfaceShapeReachesEverySurface checks the axis covers what the answer to
// "what goes square" said it should: not only the panel and popups, but the
// controls too.
func TestSurfaceShapeReachesEverySurface(t *testing.T) {
	for _, shape := range []string{"rounded", "square"} {
		t.Run(shape, func(t *testing.T) {
			_, style := installShipped(t, func(ch Choices) {
				ch.Values["surfaces"] = shape
			})

			for _, file := range []string{
				"widgets/panel-background.svg", "dialogs/background.svg",
				"widgets/background.svg", "widgets/tooltip.svg",
				"widgets/button.svg", "widgets/lineedit.svg", "widgets/viewitem.svg",
				"opaque/widgets/panel-background.svg", "solid/dialogs/background.svg",
			} {
				body := readFile(t, filepath.Join(style, filepath.FromSlash(file)))

				// Rounded corners are arcs in the controls and cubics in the
				// frames; square ones are neither.
				curved := strings.Contains(body, " C") || strings.Contains(body, " A")
				if want := shape == "rounded"; curved != want {
					t.Errorf("%s curved = %v, want %v for %s", file, curved, want, shape)
				}
			}
		})
	}
}

// TestTitlebarShapeIsAProductOfTheTint covers the decoration, which is the one
// piece of artwork two axes reach at once: the tint paints it and the titlebar
// shape decides its corners.
func TestTitlebarShapeIsAProductOfThePalette(t *testing.T) {
	for _, tint := range []string{"neutral", "slate"} {
		for _, shape := range []string{"square", "rounded"} {
			t.Run(tint+"-"+shape, func(t *testing.T) {
				theme, _ := installShipped(t, func(ch Choices) {
					ch.Values["palette"] = tint
					ch.Values["titlebar"] = shape
				})

				deco := readFile(t, filepath.Join(
					theme.TargetPath(theme.Components[2]), "decoration.svg"))

				// The file opens with a comment explaining the bottom corners, and
				// its prose contains capital letters that look like path commands.
				comment := regexp.MustCompile(`(?s)<!--.*?-->`)
				curved := strings.Contains(comment.ReplaceAllString(deco, ""), " C")
				if want := shape == "rounded"; curved != want {
					t.Errorf("titlebar curved = %v, want %v for %s", curved, want, shape)
				}

				surface := map[string]string{"neutral": "#292929", "slate": "#272a2f"}[tint]
				if !strings.Contains(deco, surface) {
					t.Errorf("titlebar is not painted in the %s surface %s", tint, surface)
				}
			})
		}
	}
}

// TestButtonStyleSwapsTheWholeTitlebarSet checks that choosing traffic lights
// replaces every button and the layout file together. A style that changed the
// artwork but left the old metrics behind would draw circles into boxes sized
// for glyphs.
func TestButtonStyleSwapsTheWholeTitlebarSet(t *testing.T) {
	for _, style := range []string{"windows", "mac"} {
		t.Run(style, func(t *testing.T) {
			theme, _ := installShipped(t, func(ch Choices) {
				ch.Values["buttons"] = style
			})
			dst := theme.TargetPath(theme.Components[2])

			circles := style == "mac"
			for _, file := range []string{"close.svg", "minimize.svg", "maximize.svg", "restore.svg"} {
				body := readFile(t, filepath.Join(dst, file))

				if got := strings.Contains(body, "<circle"); got != circles {
					t.Errorf("%s has circles = %v, want %v", file, got, circles)
				}
				// Traffic lights carry no symbol at any state, ever.
				if got := strings.Contains(body, "<path"); got == circles && circles {
					t.Errorf("%s should draw no glyph in the traffic-light style", file)
				}
			}

			rc := readFile(t, filepath.Join(dst, "VanillaBoxDarkrc"))
			// The margin centres the button in the 30px titlebar — (30-26)/2 and
			// (30-20)/2 — plus whatever optical nudge the style declares. Traffic
			// traffic lights sit a pixel above centre; symbols sit flush with the top of
			// the titlebar, so the hover plate has no gap above it.
			width, margin := "ButtonWidth=28", "ButtonMarginTop=0"
			if circles {
				width, margin = "ButtonWidth=22", "ButtonMarginTop=3"
			}
			if !strings.Contains(rc, width) {
				t.Errorf("the layout file does not carry the %s metrics (%s)", style, width)
			}

			// Aurorae leaves the slack below a button, so the margin has to centre
			// it in the titlebar or a short button rides high.
			if !strings.Contains(rc, margin) {
				t.Errorf("%s buttons are not centred in the titlebar (want %s)", style, margin)
			}
		})
	}
}

// TestTitlebarButtonMetrics pins the numbers that were tuned by eye against a
// real titlebar. Nothing else in the suite would notice them drifting: the
// artwork would still be valid, the theme would still install, and the buttons
// would just be the wrong size or sitting a pixel off. See DESIGN.md.
func TestTitlebarButtonMetrics(t *testing.T) {
	metrics := map[string]struct {
		box, margin int
		mark        string // what the artwork must draw at that size
	}{
		// 13x13px symbol on a 28x28 box, flush with the top of the titlebar so
		// the hover plate has no gap above it.
		"windows": {box: 28, margin: 0, mark: `scale(0.04352678571428571)`},
		// 11px circle on a 22x22 box, a pixel above centre.
		"mac": {box: 22, margin: 3, mark: `<circle cx="12" cy="12" r="6"`},
	}

	for style, want := range metrics {
		t.Run(style, func(t *testing.T) {
			theme, _ := installShipped(t, func(ch Choices) {
				ch.Values["buttons"] = style
			})
			dst := theme.TargetPath(theme.Components[2])

			rc := readFile(t, filepath.Join(dst, "VanillaBoxDarkrc"))
			for _, line := range []string{
				fmt.Sprintf("ButtonWidth=%d", want.box),
				fmt.Sprintf("ButtonHeight=%d", want.box),
				fmt.Sprintf("ButtonMarginTop=%d", want.margin),
				// Maximising a window lays the titlebar out from a separate set of
				// keys that default to zero, which moved every button. These have
				// to mirror the ordinary ones or the buttons jump.
				fmt.Sprintf("ButtonMarginTopMaximized=%d", want.margin),
				// Mirrors of the ordinary edges, without the padding the other
				// branch adds: padding is outside the window, and a maximised
				// window has none.
				"TitleEdgeTopMaximized=0",
				"TitleEdgeLeftMaximized=6",
				"TitleEdgeRightMaximized=6",
			} {
				if !strings.Contains(rc, line) {
					t.Errorf("layout file is missing %s", line)
				}
			}

			// A non-square box would stretch the mark, which is the reason both
			// dimensions are asserted rather than just the width.
			if !strings.Contains(readFile(t, filepath.Join(dst, "close.svg")), want.mark) {
				t.Errorf("close.svg does not draw its mark at the tuned size (%s)", want.mark)
			}
		})
	}
}

// hexOf turns "41,41,41" into "#292929", the form the artwork writes.
func hexOf(triple string) string {
	var r, g, b int
	if _, err := fmt.Sscanf(triple, "%d,%d,%d", &r, &g, &b); err != nil {
		panic(err)
	}

	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// installShipped installs the real theme into a temporary data directory, with
// choices starting at the manifest's defaults and adjusted by set. It returns
// the theme and the installed Plasma style's directory.
func installShipped(t *testing.T, set func(Choices)) (*Theme, string) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	theme, err := LoadManifest("../../assets")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	choices := theme.DefaultChoices()
	if set != nil {
		set(choices)
	}

	for _, c := range theme.Components {
		if err := theme.Install(c, choices); err != nil {
			t.Fatalf("install %s: %v", c.ID, err)
		}
	}

	if theme.Components[1].ID != "plasma-style" {
		t.Fatalf("expected plasma-style second, got %q", theme.Components[1].ID)
	}

	return theme, theme.TargetPath(theme.Components[1])
}

// TestShippedStyleFollowsTheColorScheme guards the mechanism a colour variant
// rides on. Plasma recolours a theme by substituting the current-color-scheme
// stylesheet at paint time, which only reaches elements that actually defer to
// it: they must carry a ColorScheme class and fill with currentColor. A
// hardcoded hex renders identically today and silently ignores the colors file
// tomorrow, so the drift is invisible until a variant is added.
func TestShippedStyleFollowsTheColorScheme(t *testing.T) {
	_, dst := installShipped(t, nil)

	// Without this file the Plasma shell falls back to the system scheme, and a
	// tint would reach applications but not the panel.
	colors := readFile(t, filepath.Join(dst, "colors"))
	for _, section := range []string{"[Colors:Window]", "[Colors:Tooltip]", "[Colors:Selection]"} {
		if !strings.Contains(colors, section) {
			t.Errorf("colors is missing %s", section)
		}
	}

	// The stylesheet block legitimately names the colour as an editor fallback;
	// everywhere else a background hex means the element opted out.
	defs := regexp.MustCompile(`(?s)<defs>.*?</defs>`)

	found := map[string]bool{}
	err := filepath.WalkDir(dst, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".svg" {
			return err
		}

		body := defs.ReplaceAllString(readFile(t, path), "")
		rel := filepath.ToSlash(mustRel(t, dst, path))

		if strings.Contains(body, "fill:#292929") {
			t.Errorf("%s: background is hardcoded, so a colors file cannot reach it", rel)
		}
		if strings.Contains(body, "ColorScheme-Background") {
			found[rel] = true
			if !strings.Contains(body, "fill:currentColor") {
				t.Errorf("%s: claims ColorScheme-Background but never fills with currentColor", rel)
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dst, err)
	}

	// The four background surfaces, each present in the theme root and again
	// under the opaque/ and solid/ prefixes Plasma falls back to; scrollbar draws
	// its trough from the same colour. Listing them rather than counting keeps a
	// surface that quietly stops following the scheme from passing unnoticed.
	want := []string{"widgets/scrollbar.svg"}
	for _, prefix := range []string{"", "opaque/", "solid/"} {
		want = append(want,
			prefix+"widgets/panel-background.svg",
			prefix+"widgets/background.svg",
			prefix+"widgets/tooltip.svg",
			prefix+"dialogs/background.svg",
		)
	}

	for _, path := range want {
		if !found[path] {
			t.Errorf("%s does not follow the colour scheme", path)
		}
		delete(found, path)
	}
	for path := range found {
		t.Errorf("%s follows the colour scheme but is not in the expected set", path)
	}
}

func onOff(on bool) string {
	if on {
		return "on"
	}

	return "off"
}

func mustRel(t *testing.T, base, path string) string {
	t.Helper()

	rel, err := filepath.Rel(base, path)
	if err != nil {
		t.Fatalf("rel %s %s: %v", base, path, err)
	}

	return rel
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return string(data)
}
