package theme

import (
	"os"
	"path/filepath"
	"testing"
)

const validManifest = `{
  "name": "Vanilla Box",
  "id": "VanillaBox",
  "version": "0.1.0",
  "components": [
    { "id": "colors", "name": "Color scheme", "source": "color-schemes/VanillaBox.colors",
      "target": "color-schemes", "default": true },
    { "id": "icons", "name": "Icons", "source": "icons/VanillaBox", "target": "icons" }
  ]
}`

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ManifestName), validManifest)
	writeFile(t, filepath.Join(dir, "color-schemes", "VanillaBox.colors"), "[General]\n")

	theme, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if theme.Title() != "Vanilla Box v0.1.0" {
		t.Errorf("Title() = %q, want %q", theme.Title(), "Vanilla Box v0.1.0")
	}
	if len(theme.Components) != 2 {
		t.Fatalf("got %d components, want 2", len(theme.Components))
	}
	if !theme.Components[0].Default {
		t.Error("colors should default to selected")
	}

	// The color scheme file exists; the icons directory does not.
	if !theme.Components[0].Available {
		t.Error("colors should be available")
	}
	if theme.Components[1].Available {
		t.Error("icons should be unavailable when its source is missing")
	}
	if got := len(theme.Available()); got != 1 {
		t.Errorf("Available() returned %d components, want 1", got)
	}
}

func TestAvailabilityIgnoresEmptySources(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ManifestName), validManifest)

	// An empty file and an empty directory are both what a fresh checkout leaves
	// behind, and neither is installable.
	writeFile(t, filepath.Join(dir, "color-schemes", "VanillaBox.colors"), "")
	if err := os.MkdirAll(filepath.Join(dir, "icons", "VanillaBox"), 0o755); err != nil {
		t.Fatal(err)
	}

	theme, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	for _, c := range theme.Components {
		if c.Available {
			t.Errorf("component %q should be unavailable", c.ID)
		}
	}
}

func TestLoadManifestErrors(t *testing.T) {
	tests := []struct {
		name     string
		manifest string // empty means: write no manifest at all
	}{
		{name: "missing file"},
		{name: "malformed json", manifest: `{"name": "Vanilla Box",`},
		{name: "no name", manifest: `{"components": [{"id": "a", "name": "A", "source": "s", "target": "t"}]}`},
		{name: "no components", manifest: `{"name": "Vanilla Box", "components": []}`},
		{name: "component without source", manifest: `{"name": "V", "components": [{"id": "a", "name": "A", "target": "t"}]}`},
		{name: "duplicate ids", manifest: `{"name": "V", "components": [
			{"id": "a", "name": "A", "source": "s", "target": "t"},
			{"id": "a", "name": "B", "source": "s", "target": "t"}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.manifest != "" {
				writeFile(t, filepath.Join(dir, ManifestName), tt.manifest)
			}

			if _, err := LoadManifest(dir); err == nil {
				t.Fatal("LoadManifest succeeded, want an error")
			}
		})
	}
}

func TestShippedManifestIsValid(t *testing.T) {
	theme, err := LoadManifest("../../assets")
	if err != nil {
		t.Fatalf("the shipped assets/theme.json should load: %v", err)
	}

	if len(theme.Available()) != len(theme.Components) {
		t.Errorf("%d of %d shipped components are unavailable; the asset tree is incomplete",
			len(theme.Components)-len(theme.Available()), len(theme.Components))
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
