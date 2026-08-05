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
	for _, tint := range []string{"neutral", "slate", "rose"} {
		writeFile(t, filepath.Join(assets, "variants", tint, "c.colors"), "["+tint+"]\n")
	}
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
				Target: "color-schemes", Required: true, Available: true,
				// Declares no options of its own, but reads the tint declared on
				// the Plasma style.
				Resolved: []theme.Resolved{{Source: "variants/{tint}/c.colors", Target: ""}},
			},
			{
				ID: "style", Name: "Plasma style", Source: "style",
				Target: "plasma/desktoptheme", Required: true, Available: true,
				Options: []theme.Option{
					{
						ID: "transparency", Name: "Transparency", Kind: theme.KindToggle,
						Default: true,
						OverlayWhenOff: theme.Overlay{
							From:  "style/opaque",
							Files: []string{"widgets/panel.svg"},
						},
					},
					{
						ID: "tint", Name: "Colour", Kind: theme.KindSelect,
						DefaultValue: "neutral",
						Values: []theme.OptionValue{
							{ID: "neutral", Name: "Neutral"},
							{ID: "slate", Name: "Slate"},
							{ID: "rose", Name: "Rose"},
						},
					},
				},
			},
			{
				ID: "cursors", Name: "Cursors", Source: "missing",
				Target: "icons", Required: true, Available: false,
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
	case "left":
		return tea.KeyPressMsg{Code: 'h', Text: "h"}
	case "right":
		return tea.KeyPressMsg{Code: 'l', Text: "l"}
	}

	panic("unhandled key " + k)
}

// optionAt puts the cursor on the named switch, so tests do not depend on the
// order the rows happen to come out in.
func optionAt(t *testing.T, m Model, id string) Model {
	t.Helper()

	for i, r := range m.optionRows() {
		if r.kind == rowToggle && r.option.ID == id {
			m.optionCursor = i

			return m
		}
	}

	t.Fatalf("no %q switch among the visible preferences", id)

	return m
}

// TestPreferencesIsTheFirstScreen covers the removal of the checklist: there is
// nothing to choose between components, so the run opens on the preferences.
func TestPreferencesIsTheFirstScreen(t *testing.T) {
	m := New(testTheme(t))

	if m.screen != screenOptions {
		t.Fatalf("screen = %v, want screenOptions", m.screen)
	}
	if got := m.installCount(); got != 2 {
		t.Errorf("installCount() = %d, want 2 (the two whose files are present)", got)
	}

	view := renderView(m)
	if strings.Contains(view, "Choose what to install") {
		t.Error("the component checklist should be gone")
	}
	if !strings.Contains(view, "Preferences") {
		t.Error("the first screen should be the preferences")
	}
}

// TestUnavailableComponentIsSkipped is what replaces the greyed-out checklist
// row: a gap in the asset tree installs the rest and says what it left out.
func TestUnavailableComponentIsSkipped(t *testing.T) {
	m := New(testTheme(t))

	if m.items[2].selected {
		t.Error("a component whose files are missing should not be queued")
	}

	m, _ = send(m, "enter")
	if view := renderView(m); !strings.Contains(view, "Cursors — unavailable, will be skipped") {
		t.Errorf("the review should name what it is skipping, got:\n%s", view)
	}
}

// TestEveryValueIsVisible is the point of the layout change: the alternatives
// are on screen rather than behind a control you have to operate to find them.
func TestEveryValueIsVisible(t *testing.T) {
	view := renderView(New(testTheme(t)))

	for _, name := range []string{"Neutral", "Slate", "Rose"} {
		if !strings.Contains(view, name) {
			t.Errorf("preferences should list %q without the user hunting for it", name)
		}
	}
}

// TestChoosingAValueReplacesTheCurrentOne covers the radio behaviour: picking
// one value of a choice unsets the other, rather than accumulating.
func TestChoosingAValueReplacesTheCurrentOne(t *testing.T) {
	m := New(testTheme(t))

	if got := m.choices.Values["tint"]; got != "neutral" {
		t.Fatalf("tint = %q, want the declared default", got)
	}

	m = rowFor(t, m, "tint", "rose")
	m, _ = send(m, " ")

	if got := m.choices.Values["tint"]; got != "rose" {
		t.Errorf("tint = %q, want rose", got)
	}

	m = rowFor(t, m, "tint", "slate")
	m, _ = send(m, " ")

	if got := m.choices.Values["tint"]; got != "slate" {
		t.Errorf("tint = %q, want slate — a choice holds one value", got)
	}
}

// TestCursorSkipsHeaders keeps the cursor on rows that do something. A header
// names a choice and cannot be selected.
func TestCursorSkipsHeaders(t *testing.T) {
	m := New(testTheme(t))
	rows := m.optionRows()

	if rows[m.optionCursor].kind == rowHeader {
		t.Fatal("the cursor should not start on a header")
	}

	for range len(rows) + 2 {
		m, _ = send(m, "down")
		if rows[m.optionCursor].kind == rowHeader {
			t.Fatalf("cursor landed on the header at row %d", m.optionCursor)
		}
	}

	for range len(rows) + 2 {
		m, _ = send(m, "up")
		if rows[m.optionCursor].kind == rowHeader {
			t.Fatalf("cursor landed on the header at row %d going back", m.optionCursor)
		}
	}
}

// TestShortTerminalScrolls checks the window follows the cursor rather than
// letting it walk off the bottom of a list taller than the screen.
func TestShortTerminalScrolls(t *testing.T) {
	m := New(testTheme(t))

	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 13})
	m = next.(Model)

	visible := m.visibleRows(len(m.optionRows()))
	if visible >= len(m.optionRows()) {
		t.Fatalf("test needs a window shorter than the list: %d of %d", visible, len(m.optionRows()))
	}

	for range len(m.optionRows()) {
		m, _ = send(m, "down")

		if m.optionCursor < m.optionScroll || m.optionCursor >= m.optionScroll+visible {
			t.Fatalf("cursor %d outside the window [%d,%d)",
				m.optionCursor, m.optionScroll, m.optionScroll+visible)
		}
	}

	if !strings.Contains(renderView(m), "below") && !strings.Contains(renderView(m), "above") {
		t.Error("a clipped list should say how much is off screen")
	}
}

// rowFor puts the cursor on a named value of a named option.
func rowFor(t *testing.T, m Model, option, value string) Model {
	t.Helper()

	for i, r := range m.optionRows() {
		if r.kind == rowValue && r.option.ID == option && r.value.ID == value {
			m.optionCursor = i

			return m
		}
	}

	t.Fatalf("no %q row for option %q", value, option)

	return m
}

func TestConfirmScreenListsTargetsAndOptions(t *testing.T) {
	m := New(testTheme(t))

	m = optionAt(t, m, "transparency")
	m, _ = send(m, " ", "enter")

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

// TestEscapeReturnsToPreferences checks the one step back that still exists.
// The preferences are the first screen now, so esc there has nowhere to go and
// the binding is disabled rather than silently doing nothing.
func TestEscapeReturnsToPreferences(t *testing.T) {
	m := New(testTheme(t))

	m, _ = send(m, "enter", "esc")
	if m.screen != screenOptions {
		t.Fatalf("screen = %v, want screenOptions after esc from the review", m.screen)
	}

	if m.keys.Back.Enabled() {
		t.Error("esc should be disabled on the first screen")
	}
}

// TestInstallWalksTheQueue drives the whole install by hand: it runs each
// command the model returns and feeds the resulting message back in, which is
// what the Bubble Tea runtime does.
func TestInstallWalksTheQueue(t *testing.T) {
	m := New(testTheme(t))

	m, _ = send(m, "enter") // through the preferences, to the review
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

	if m.screen != screenOptions {
		t.Errorf("screen = %v, want screenOptions", m.screen)
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
