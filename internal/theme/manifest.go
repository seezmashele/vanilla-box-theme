// Package theme describes the Vanilla Box theme and the components it is made
// of, and knows how to install them onto a KDE Plasma desktop.
package theme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ManifestName is the file, relative to the asset directory, that describes the
// theme.
const ManifestName = "theme.json"

// Component is one installable part of the theme, such as the color scheme or
// the icon set. Everything the installer needs in order to apply it lives here,
// so adding a component is a change to theme.json rather than to code.
type Component struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Source is the file or directory holding this component, relative to the
	// asset directory.
	Source string `json:"source"`

	// Target is the directory this component is copied into, relative to the
	// user's data directory (~/.local/share).
	Target string `json:"target"`

	// Options are the preferences that change what gets written for this
	// component. They hang off the component rather than the theme so the
	// preferences screen can show only what the current selection affects.
	Options []Option `json:"options"`

	// Default reports whether the component starts out selected.
	Default bool `json:"default"`

	// Available reports whether Source actually exists in the asset directory.
	// It is filled in by the availability check, not read from theme.json.
	Available bool `json:"-"`
}

// Option is a user preference that changes what is written for a component.
//
// The theme ships its own variants as directories inside the component, so an
// option does not edit any file: switching one off copies a directory over the
// files already installed. That keeps the artwork the only place the theme's
// looks are defined.
type Option struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// Default reports whether the option starts out on.
	Default bool `json:"default"`

	// OverlayWhenOff is a directory inside the component's source, copied over
	// the installed files when the option is switched off.
	OverlayWhenOff string `json:"overlayWhenOff"`
}

// Theme is a whole theme: its identity plus the components it ships.
type Theme struct {
	Name        string      `json:"name"`
	ID          string      `json:"id"`
	Version     string      `json:"version"`
	Author      string      `json:"author"`
	Description string      `json:"description"`
	Components  []Component `json:"components"`

	// AssetDir is the directory the manifest was loaded from.
	AssetDir string `json:"-"`

	// Stamp names this run's backup directory. It is fixed when the theme loads
	// so every component installed in one session backs up to the same place.
	Stamp string `json:"-"`
}

// LoadManifest reads theme.json from assetDir and records, for each component,
// whether its source files are actually present.
func LoadManifest(assetDir string) (*Theme, error) {
	path := filepath.Join(assetDir, ManifestName)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var t Theme
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := t.validate(); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", path, err)
	}

	t.AssetDir = assetDir
	t.Stamp = time.Now().Format("20060102-150405")
	t.refreshAvailability()

	return &t, nil
}

func (t *Theme) validate() error {
	if t.Name == "" {
		return fmt.Errorf("theme name is empty")
	}
	if len(t.Components) == 0 {
		return fmt.Errorf("theme declares no components")
	}

	seen := make(map[string]bool, len(t.Components))
	for i, c := range t.Components {
		switch {
		case c.ID == "":
			return fmt.Errorf("component %d has no id", i)
		case seen[c.ID]:
			return fmt.Errorf("duplicate component id %q", c.ID)
		case c.Name == "":
			return fmt.Errorf("component %q has no name", c.ID)
		case c.Source == "":
			return fmt.Errorf("component %q has no source", c.ID)
		case c.Target == "":
			return fmt.Errorf("component %q has no target", c.ID)
		}
		seen[c.ID] = true

		if err := validateOptions(c); err != nil {
			return err
		}
	}

	return nil
}

func validateOptions(c Component) error {
	seen := make(map[string]bool, len(c.Options))

	for i, o := range c.Options {
		switch {
		case o.ID == "":
			return fmt.Errorf("component %q: option %d has no id", c.ID, i)
		case seen[o.ID]:
			return fmt.Errorf("component %q: duplicate option id %q", c.ID, o.ID)
		case o.Name == "":
			return fmt.Errorf("component %q: option %q has no name", c.ID, o.ID)
		case o.OverlayWhenOff == "":
			return fmt.Errorf("component %q: option %q does nothing when off", c.ID, o.ID)
		}
		seen[o.ID] = true
	}

	return nil
}

// Title is the theme's display name with its version, for the UI header.
func (t *Theme) Title() string {
	if t.Version == "" {
		return t.Name
	}
	return t.Name + " v" + t.Version
}

// SourcePath is where the component's files are read from.
func (t *Theme) SourcePath(c Component) string {
	return filepath.Join(t.AssetDir, c.Source)
}

// TargetPath is where the component's files will be written. Errors resolving
// the home directory are folded into the path itself, since this value is only
// ever displayed or used as a destination that will fail loudly on write.
func (t *Theme) TargetPath(c Component) string {
	return filepath.Join(dataDir(), c.Target, filepath.Base(c.Source))
}

// DisplayTargetPath is TargetPath with the home directory shortened to "~", for
// the confirmation screen.
func (t *Theme) DisplayTargetPath(c Component) string {
	path := t.TargetPath(c)

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if rel, err := filepath.Rel(home, path); err == nil && !filepath.IsAbs(rel) {
		return filepath.Join("~", rel)
	}

	return path
}

// dataDir is the XDG data directory theme components are installed into.
func dataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ".local/share"
	}

	return filepath.Join(home, ".local", "share")
}
