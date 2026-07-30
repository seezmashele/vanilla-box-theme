package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vanillabox/internal/theme"
)

// testTheme is a small stand-in with one optioned component and one unavailable
// one, so the tests cover both the preferences step and what a partial asset
// tree produces. Installs are real, so it ships real files and redirects the
// data directory into the test's temporary space.
func testTheme(t *testing.T) *theme.Theme {
	t.Helper()

	assets := t.TempDir()
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	writeFile(t, filepath.Join(assets, "colors.colors"), "[General]\n")
	writeFile(t, filepath.Join(assets, "style", "widgets", "panel.svg"), "translucent\n")
	writeFile(t, filepath.Join(assets, "style", "opaque", "widgets", "panel.svg"), "opaque\n")

	return &theme.Theme{
		Name:     "Vanilla Box",
		Version:  "0.1.0",
		AssetDir: assets,
		Stamp:    "test",
		Components: []theme.Component{
			{
				ID: "colors", Name: "Color scheme", Source: "colors.colors",
				Target: "color-schemes", Default: true, Available: true,
			},
			{
				ID: "style", Name: "Plasma style", Source: "style",
				Target: "plasma/desktoptheme", Default: true, Available: true,
				Options: []theme.Option{{
					ID: "transparency", Name: "Transparency",
					Default: true, OverlayWhenOff: "opaque",
				}},
			},
			{
				ID: "cursors", Name: "Cursors", Source: "missing",
				Target: "icons", Available: false,
			},
		},
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

// send pushes a key through the model the way the runtime would.
func send(m Model, keys ...string) (Model, tea.Cmd) {
	var cmd tea.Cmd

	for _, k := range keys {
		var next tea.Model
		next, cmd = m.Update(keyPress(k))
		m = next.(Model)
	}

	return m, cmd
}

func keyPress(k string) tea.KeyPressMsg {
	if len(k) == 1 {
		return tea.KeyPressMsg{Code: rune(k[0]), Text: k}
	}

	switch k {
	case "enter":
		return tea.KeyPressMsg{Code: '\r'}
	case "esc":
		return tea.KeyPressMsg{Code: 27}
	case "down":
		return tea.KeyPressMsg{Code: 'j', Text: "j"}
	case "up":
		return tea.KeyPressMsg{Code: 'k', Text: "k"}
	}

	panic("unhandled key " + k)
}

func TestSelectScreenStartsOnDefaults(t *testing.T) {
	m := New(testTheme(t))

	if m.screen != screenSelect {
		t.Fatalf("screen = %v, want screenSelect", m.screen)
	}
	if got := m.selectedCount(); got != 2 {
		t.Errorf("selectedCount() = %d, want 2 (defaults that are available)", got)
	}

	view := renderView(m)
	if !strings.Contains(view, "Vanilla Box v0.1.0") {
		t.Error("select view should show the theme title")
	}
	if !strings.Contains(view, "unavailable") {
		t.Error("select view should mark the unavailable component")
	}
}

func TestUnavailableComponentCannotBeSelected(t *testing.T) {
	m := New(testTheme(t))
	m.cursor = 2 // the unavailable one

	m, _ = send(m, " ")

	if m.items[2].selected {
		t.Error("toggling an unavailable component should do nothing")
	}
}

func TestSelectAllSkipsUnavailable(t *testing.T) {
	m := New(testTheme(t))

	m, _ = send(m, "a")

	if got := m.selectedCount(); got != 2 {
		t.Errorf("selectedCount() after 'a' = %d, want 2", got)
	}
}

func TestCannotContinueWithNothingSelected(t *testing.T) {
	m := New(testTheme(t))

	m, _ = send(m, "n", "enter")

	if m.screen != screenSelect {
		t.Error("enter with nothing selected should stay on the select screen")
	}
	if m.keys.Confirm.Enabled() {
		t.Error("the continue binding should be disabled with nothing selected")
	}
}

func TestOptionsScreenStartsOnDefaults(t *testing.T) {
	m := New(testTheme(t))

	m, _ = send(m, "enter")

	if m.screen != screenOptions {
		t.Fatalf("screen = %v, want screenOptions", m.screen)
	}
	if !m.choices["transparency"] {
		t.Error("transparency should start on, from the option's default")
	}

	if view := renderView(m); !strings.Contains(view, "Transparency") {
		t.Error("options view should list the option")
	}

	m, _ = send(m, " ")
	if m.choices["transparency"] {
		t.Error("space should turn the option off")
	}
}

// TestOptionsScreenSkippedWithoutOptions covers deselecting the only component
// that has preferences: there is nothing to ask about, so the step is skipped
// rather than shown empty.
func TestOptionsScreenSkippedWithoutOptions(t *testing.T) {
	m := New(testTheme(t))

	m.cursor = 1 // the optioned component
	m, _ = send(m, " ", "enter")

	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want screenConfirm", m.screen)
	}

	// And esc goes back to the checklist, not to a screen that was never shown.
	m, _ = send(m, "esc")
	if m.screen != screenSelect {
		t.Errorf("screen = %v, want screenSelect after esc", m.screen)
	}
}

func TestConfirmScreenListsTargetsAndOptions(t *testing.T) {
	m := New(testTheme(t))

	m, _ = send(m, "enter", " ", "enter")

	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want screenConfirm", m.screen)
	}

	view := renderView(m)
	if !strings.Contains(view, "Color scheme") {
		t.Error("confirm view should list the selected components")
	}
	if !strings.Contains(view, "color-schemes") {
		t.Error("confirm view should show where files go")
	}
	if !strings.Contains(view, "Transparency: off") {
		t.Error("confirm view should report the chosen options")
	}
	if !strings.Contains(view, "System Settings") {
		t.Error("confirm view should say the theme is not applied")
	}
}

func TestEscapeGoesBackScreenByScreen(t *testing.T) {
	m := New(testTheme(t))

	m, _ = send(m, "enter", "enter", "esc")
	if m.screen != screenOptions {
		t.Fatalf("screen = %v, want screenOptions after esc from confirm", m.screen)
	}

	m, _ = send(m, "esc")
	if m.screen != screenSelect {
		t.Errorf("screen = %v, want screenSelect after esc from options", m.screen)
	}
}

// TestInstallWalksTheQueue drives the whole install by hand: it runs each
// command the model returns and feeds the resulting message back in, which is
// what the Bubble Tea runtime does.
func TestInstallWalksTheQueue(t *testing.T) {
	m := New(testTheme(t))

	m, _ = send(m, "enter", "enter") // through the preferences, to the review
	next, cmd := m.Update(keyPress("enter"))
	m = next.(Model)

	if m.screen != screenInstall {
		t.Fatalf("screen = %v, want screenInstall", m.screen)
	}
	if len(m.queue) != 2 {
		t.Fatalf("queue has %d steps, want 2", len(m.queue))
	}
	if m.items[m.queue[0]].status != statusRunning {
		t.Error("the first queued component should be running")
	}
	if m.keys.Quit.Enabled() {
		t.Error("quit should be disabled while installing")
	}

	// Run the install to completion.
	for step := range m.queue {
		msg := runInstallStep(t, cmd)
		if msg == nil {
			t.Fatalf("step %d produced no stepDoneMsg", step)
		}

		next, cmd = m.Update(msg)
		m = next.(Model)
	}

	succeeded, failed := m.results()
	if succeeded != 2 || failed != 0 {
		t.Errorf("results() = (%d, %d), want (2, 0)", succeeded, failed)
	}
	if !m.finishing {
		t.Error("the model should be waiting for the progress bar to catch up")
	}

	// The progress bar finishing is what moves us to the done screen. The last
	// command the model handed back animates it; keep pumping frames until it
	// settles.
	for range 500 {
		if cmd == nil || m.screen == screenDone {
			break
		}

		next, cmd = m.Update(cmd())
		m = next.(Model)
	}

	if m.screen != screenDone {
		t.Fatal("never reached the done screen")
	}
	if view := renderView(m); !strings.Contains(view, "2 installed") {
		t.Errorf("done view should report 2 installed, got:\n%s", view)
	}
}

func TestRestartFromDone(t *testing.T) {
	m := New(testTheme(t))
	m.screen = screenDone
	m.queue = []int{0}
	m.items[0].status = statusOK
	m.updateBindings()

	m, _ = send(m, "r")

	if m.screen != screenSelect {
		t.Errorf("screen = %v, want screenSelect", m.screen)
	}
	if m.items[0].status != statusPending {
		t.Error("restart should clear step statuses")
	}
}

func TestErrorModelShowsTheReason(t *testing.T) {
	m := NewError(errMissingAssets{})

	view := renderView(m)
	if !strings.Contains(view, "Could not load the theme") {
		t.Error("error view should say the theme could not be loaded")
	}
	if !strings.Contains(view, "no theme.json") {
		t.Error("error view should include the underlying error")
	}
}

type errMissingAssets struct{}

func (errMissingAssets) Error() string { return "no theme.json found" }

// runInstallStep executes a command chain until it yields a stepDoneMsg.
func runInstallStep(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()

	if cmd == nil {
		return nil
	}

	deadline := time.Now().Add(10 * time.Second)

	pending := []tea.Cmd{cmd}
	for len(pending) > 0 && time.Now().Before(deadline) {
		next := pending[0]
		pending = pending[1:]

		if next == nil {
			continue
		}

		switch msg := next().(type) {
		case stepDoneMsg:
			return msg
		case tea.BatchMsg:
			pending = append(pending, msg...)
		}
	}

	return nil
}

func renderView(m Model) string {
	return m.View().Content
}
