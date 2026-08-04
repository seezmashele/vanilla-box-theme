package theme

import (
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
			ID: "transparency", Name: "Transparency",
			Default: true, OverlayWhenOff: "opaque",
		}},
	}

	return &Theme{AssetDir: assets, Stamp: "20260730-120000"}, component
}

func TestInstallCopiesTheComponent(t *testing.T) {
	theme, component := installFixture(t)

	if err := theme.Install(component, map[string]bool{"transparency": true}); err != nil {
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

	if err := theme.Install(component, map[string]bool{"transparency": false}); err != nil {
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

	if err := theme.Install(component, map[string]bool{"transparency": true}); err != nil {
		t.Fatalf("first Install: %v", err)
	}

	// Something the user left behind, which must survive in the backup rather
	// than be silently overwritten.
	dst := theme.TargetPath(component)
	writeFile(t, filepath.Join(dst, "widgets", "custom.svg"), "mine\n")

	if err := theme.Install(component, map[string]bool{"transparency": true}); err != nil {
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

func TestInstallReportsAMissingSource(t *testing.T) {
	theme, _ := installFixture(t)

	err := theme.Install(Component{ID: "gone", Name: "Gone", Source: "nowhere", Target: "x"}, nil)
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
	const panel = "widgets/panel-background.svg"

	for _, transparency := range []bool{true, false} {
		t.Run("transparency="+onOff(transparency), func(t *testing.T) {
			t.Setenv("XDG_DATA_HOME", t.TempDir())

			theme, err := LoadManifest("../../assets")
			if err != nil {
				t.Fatalf("LoadManifest: %v", err)
			}

			choices := map[string]bool{"transparency": transparency}
			for _, c := range theme.Components {
				if err := theme.Install(c, choices); err != nil {
					t.Fatalf("install %s: %v", c.ID, err)
				}
			}

			style := theme.TargetPath(theme.Components[1])
			if theme.Components[1].ID != "plasma-style" {
				t.Fatalf("expected plasma-style second, got %q", theme.Components[1].ID)
			}

			// The panel follows the option; the task manager never does, because
			// its translucency is white highlight artwork rather than background.
			translucent := strings.Contains(
				readFile(t, filepath.Join(style, filepath.FromSlash(panel))),
				"fill-opacity:0.85",
			)
			if translucent != transparency {
				t.Errorf("panel translucent = %v, want %v", translucent, transparency)
			}

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

// TestShippedStyleFollowsTheColorScheme guards the mechanism a colour variant
// rides on. Plasma recolours a theme by substituting the current-color-scheme
// stylesheet at paint time, which only reaches elements that actually defer to
// it: they must carry a ColorScheme class and fill with currentColor. A
// hardcoded hex renders identically today and silently ignores the colors file
// tomorrow, so the drift is invisible until a variant is added.
func TestShippedStyleFollowsTheColorScheme(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	theme, err := LoadManifest("../../assets")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	style := theme.Components[1]
	if style.ID != "plasma-style" {
		t.Fatalf("expected plasma-style second, got %q", style.ID)
	}
	if err := theme.Install(style, map[string]bool{"transparency": true}); err != nil {
		t.Fatalf("install %s: %v", style.ID, err)
	}

	dst := theme.TargetPath(style)

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
	err = filepath.WalkDir(dst, func(path string, d fs.DirEntry, err error) error {
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
