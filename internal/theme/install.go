package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// BackupDir is where a replaced install is kept, relative to the user's data
// directory. It sits outside the theme directories on purpose: Plasma scans
// those, so a backup left beside the real thing would show up in System
// Settings as a second theme.
const BackupDir = "vanillabox/backups"

// Install places one component's files where KDE will find them.
//
// It does not apply anything. Once the files are in place the theme appears in
// System Settings, and it is the user who chooses it there.
//
// choices holds the state of every option, keyed by Option.ID. An option that
// is off has its overlay copied over the files already written.
func (t *Theme) Install(c Component, choices map[string]bool) error {
	src := t.SourcePath(c)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("read %s: %w", c.Source, err)
	}

	dst := t.TargetPath(c)

	if err := t.backup(c, dst); err != nil {
		return fmt.Errorf("back up the existing %s: %w", c.Name, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := copyTree(src, dst); err != nil {
		return fmt.Errorf("copy %s: %w", c.Name, err)
	}

	return overlayOptions(c, dst, choices)
}

// overlayOptions applies the overlay of every option that is switched off. An
// option that is on needs nothing done: the files as shipped are already what
// it describes.
func overlayOptions(c Component, dst string, choices map[string]bool) error {
	for _, o := range c.Options {
		if choices[o.ID] {
			continue
		}

		// The overlay lives inside what was just installed, and holds no
		// directory of its own name, so copying it over its own parent cannot
		// recurse.
		overlay := filepath.Join(dst, o.OverlayWhenOff)
		if _, err := os.Stat(overlay); err != nil {
			return fmt.Errorf("%s: no %q overlay in %s: %w", o.Name, o.OverlayWhenOff, c.Source, err)
		}
		if err := copyTree(overlay, dst); err != nil {
			return fmt.Errorf("%s: %w", o.Name, err)
		}
	}

	return nil
}

// backup moves an existing install out of the way. Nothing there is not an
// error: most components will not be installed yet.
func (t *Theme) backup(c Component, dst string) error {
	if _, err := os.Lstat(dst); err != nil {
		return nil
	}

	dir := filepath.Join(dataDir(), BackupDir, t.Stamp, c.Target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	return move(dst, freePath(filepath.Join(dir, filepath.Base(dst))))
}

// freePath returns path, or the first "path-2", "path-3"... that is free.
// Installing twice in one session shares a Stamp, and the second run must not
// land on top of the first run's backup.
func freePath(path string) string {
	if _, err := os.Lstat(path); err != nil {
		return path
	}

	for n := 2; ; n++ {
		candidate := path + "-" + strconv.Itoa(n)
		if _, err := os.Lstat(candidate); err != nil {
			return candidate
		}
	}
}
