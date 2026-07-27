package theme

import (
	"math/rand/v2"
	"time"
)

// Simulated reports whether installs are faked. The UI reads this to warn the
// user that nothing is actually being written.
const Simulated = true

// Install applies a single component.
//
// It is currently a stub: it sleeps for a plausible amount of time and reports
// success without touching the filesystem. This is the only place in the
// program that pretends. Making installs real means replacing the body below
// with, roughly:
//
//	copy t.SourcePath(c) into t.TargetPath(c)
//	run c.ApplyCmd with c.ApplyArgs
//
// and flipping Simulated to false. Nothing in internal/ui needs to change: the
// UI already runs this one component at a time and renders whatever error comes
// back.
func (t *Theme) Install(c Component) error {
	time.Sleep(simulatedWork())

	return nil
}

// simulatedWork is how long a fake install step takes. The jitter keeps the
// progress bar from looking mechanical.
func simulatedWork() time.Duration {
	const (
		min = 500 * time.Millisecond
		max = 900 * time.Millisecond
	)

	return min + time.Duration(rand.Int64N(int64(max-min)))
}
