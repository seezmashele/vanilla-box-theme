// Package ui is the terminal interface for the Vanilla Box installer.
package ui

import (
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"vanillabox/internal/theme"
)

// screen is which step of the flow is on screen.
type screen int

const (
	screenSelect screen = iota
	screenOptions
	screenConfirm
	screenInstall
	screenDone
	screenError
)

// stepStatus is how far along one component's install is.
type stepStatus int

const (
	statusPending stepStatus = iota
	statusRunning
	statusOK
	statusFailed
)

// item is one component plus the UI state that hangs off it.
type item struct {
	component theme.Component
	selected  bool
	status    stepStatus
	err       error
}

// stepDoneMsg reports that one install step finished. Index is a position in
// the install queue, not in items.
type stepDoneMsg struct {
	index int
	err   error
}

// Model is the root model. Screens are methods on it rather than separate
// models, since they all read the same component list.
type Model struct {
	theme *theme.Theme
	items []item
	queue []int
	fatal error

	// choices is the state of every option in the theme, keyed by Option.ID.
	// It is keyed across all components rather than per component so it can be
	// handed to Install as-is.
	choices theme.Choices

	screen       screen
	cursor       int
	optionCursor int

	// finishing means the last step is done and we are letting the progress bar
	// animate up to 100% before moving on.
	finishing bool

	spinner  spinner.Model
	progress progress.Model
	help     help.Model
	keys     keyMap
	styles   styles

	width  int
	height int
}

// New builds the installer UI for a loaded theme.
func New(t *theme.Theme) Model {
	items := make([]item, len(t.Components))
	choices := t.DefaultChoices()

	for i, c := range t.Components {
		items[i] = item{
			component: c,
			selected:  c.Default && c.Available,
		}
	}

	s := newStyles(true)

	m := Model{
		theme:    t,
		items:    items,
		choices:  choices,
		screen:   screenSelect,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(s.accent)),
		progress: newProgress(s),
		help:     help.New(),
		keys:     newKeyMap(),
		styles:   s,
	}
	m.cursor = m.firstSelectable()
	m.updateBindings()

	return m
}

// NewError builds a UI that does nothing but explain why the theme could not be
// loaded.
func NewError(err error) Model {
	m := Model{
		fatal:  err,
		screen: screenError,
		help:   help.New(),
		keys:   newKeyMap(),
		styles: newStyles(true),
	}
	m.updateBindings()

	return m
}

// Init implements tea.Model. The background color request is what lets the
// styles adapt to a light or dark terminal.
func (m Model) Init() tea.Cmd {
	return tea.RequestBackgroundColor
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.SetWidth(m.contentWidth())
		m.progress.SetWidth(min(m.contentWidth(), 48))

		return m, nil

	case tea.BackgroundColorMsg:
		m.styles = newStyles(msg.IsDark())
		m.spinner.Style = m.styles.accent
		m.progress = newProgress(m.styles)
		m.progress.SetWidth(min(m.contentWidth(), 48))

		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)

		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel

		// Hold on the install screen until the bar has caught up with reality.
		if m.finishing && !m.progress.IsAnimating() {
			m.finishing = false
			m.screen = screenDone
			m.updateBindings()
		}

		return m, cmd

	case stepDoneMsg:
		return m.handleStepDone(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.Quit, m.keys.ForceQuit) {
		return m, tea.Quit
	}

	switch m.screen {
	case screenSelect:
		return m.handleSelectKey(msg)
	case screenOptions:
		return m.handleOptionsKey(msg)
	case screenConfirm:
		return m.handleConfirmKey(msg)
	case screenDone:
		if key.Matches(msg, m.keys.Restart) {
			return m.restart()
		}
	}

	return m, nil
}

func (m Model) handleSelectKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.cursor = m.moveCursor(-1)

	case key.Matches(msg, m.keys.Down):
		m.cursor = m.moveCursor(1)

	case key.Matches(msg, m.keys.Toggle):
		if m.cursor < len(m.items) && m.items[m.cursor].component.Available {
			m.items[m.cursor].selected = !m.items[m.cursor].selected
			m.updateBindings()
		}

	case key.Matches(msg, m.keys.All):
		for i := range m.items {
			m.items[i].selected = m.items[i].component.Available
		}
		m.updateBindings()

	case key.Matches(msg, m.keys.None):
		for i := range m.items {
			m.items[i].selected = false
		}
		m.updateBindings()

	case key.Matches(msg, m.keys.Confirm):
		if m.selectedCount() > 0 {
			m.screen = m.afterSelect()
			m.optionCursor = 0
			m.updateBindings()
		}
	}

	return m, nil
}

// afterSelect is the screen that follows the checklist: the preferences, or
// straight to the review when nothing selected has any.
func (m Model) afterSelect() screen {
	if len(m.visibleOptions()) == 0 {
		return screenConfirm
	}

	return screenOptions
}

// beforeConfirm is afterSelect in reverse — where esc from the review lands.
// The two cannot share a helper: skipping the preferences forwards means going
// to the review, and skipping them backwards means returning to the checklist.
func (m Model) beforeConfirm() screen {
	if len(m.visibleOptions()) == 0 {
		return screenSelect
	}

	return screenOptions
}

// visibleOptions are the preferences the current selection actually uses.
// Deselecting a component hides its preferences rather than leaving them on
// screen doing nothing.
//
// A component uses a preference either by declaring it or by naming it in a
// resolved path. The second case is what lets a theme-wide choice like the tint
// be declared once and still be offered whenever anything that reads it is
// being installed — without repeating its values on every component.
func (m Model) visibleOptions() []theme.Option {
	declared := map[string]theme.Option{}
	for _, it := range m.items {
		for _, o := range it.component.Options {
			declared[o.ID] = o
		}
	}

	var options []theme.Option
	seen := map[string]bool{}

	add := func(id string) {
		o, ok := declared[id]
		if !ok || seen[id] {
			return
		}

		seen[id] = true
		options = append(options, o)
	}

	for _, it := range m.items {
		if !it.selected {
			continue
		}

		for _, o := range it.component.Options {
			add(o.ID)
		}
		for _, r := range it.component.Resolved {
			for _, id := range theme.Placeholders(r.Source) {
				add(id)
			}
		}
	}

	return options
}

// hasSelect reports whether any option on the preferences screen is a choice
// among values rather than a switch.
func (m Model) hasSelect() bool {
	for _, o := range m.visibleOptions() {
		if o.Kind == theme.KindSelect {
			return true
		}
	}

	return false
}

func (m Model) handleOptionsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	options := m.visibleOptions()

	switch {
	case key.Matches(msg, m.keys.Up):
		if m.optionCursor > 0 {
			m.optionCursor--
		}

	case key.Matches(msg, m.keys.Down):
		if m.optionCursor < len(options)-1 {
			m.optionCursor++
		}

	case key.Matches(msg, m.keys.Toggle):
		m.changeOption(options, 1)

	case key.Matches(msg, m.keys.Next):
		m.changeOption(options, 1)

	case key.Matches(msg, m.keys.Prev):
		m.changeOption(options, -1)

	case key.Matches(msg, m.keys.Back):
		m.screen = screenSelect
		m.updateBindings()

	case key.Matches(msg, m.keys.Confirm):
		m.screen = screenConfirm
		m.updateBindings()
	}

	return m, nil
}

// changeOption moves the option under the cursor by delta. A toggle has two
// states and flips whichever way it is nudged; a select steps through its
// values and wraps, so the same keys work on both kinds of row.
func (m *Model) changeOption(options []theme.Option, delta int) {
	if m.optionCursor >= len(options) {
		return
	}

	o := options[m.optionCursor]
	if o.Kind != theme.KindSelect {
		m.choices.Toggles[o.ID] = !m.choices.Toggles[o.ID]

		return
	}

	at := 0
	for i, v := range o.Values {
		if v.ID == m.choices.Values[o.ID] {
			at = i
		}
	}

	next := (at + delta + len(o.Values)) % len(o.Values)
	m.choices.Values[o.ID] = o.Values[next].ID
}

func (m Model) handleConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.screen = m.beforeConfirm()
		m.updateBindings()

		return m, nil

	case key.Matches(msg, m.keys.Install):
		return m.beginInstall()
	}

	return m, nil
}

// beginInstall queues the selected components and starts the first one.
func (m Model) beginInstall() (tea.Model, tea.Cmd) {
	m.queue = m.queue[:0]
	for i := range m.items {
		m.items[i].status = statusPending
		m.items[i].err = nil

		if m.items[i].selected {
			m.queue = append(m.queue, i)
		}
	}

	m.screen = screenInstall
	m.finishing = false
	m.updateBindings()

	m.items[m.queue[0]].status = statusRunning

	return m, tea.Batch(
		m.spinner.Tick,
		m.progress.SetPercent(0),
		m.installStep(0),
	)
}

// installStep runs one component's install off the UI goroutine. This is the
// seam: it is the only thing standing between the interface and a real install,
// and it does not care whether theme.Install is a stub or the real thing.
func (m Model) installStep(qi int) tea.Cmd {
	component := m.items[m.queue[qi]].component

	return func() tea.Msg {
		return stepDoneMsg{index: qi, err: m.theme.Install(component, m.choices)}
	}
}

func (m Model) handleStepDone(msg stepDoneMsg) (tea.Model, tea.Cmd) {
	if msg.index >= len(m.queue) {
		return m, nil
	}

	done := m.queue[msg.index]
	if msg.err != nil {
		m.items[done].status = statusFailed
		m.items[done].err = msg.err
	} else {
		m.items[done].status = statusOK
	}

	next := msg.index + 1
	progressCmd := m.progress.SetPercent(float64(next) / float64(len(m.queue)))

	if next >= len(m.queue) {
		m.finishing = true

		return m, progressCmd
	}

	m.items[m.queue[next]].status = statusRunning

	return m, tea.Batch(progressCmd, m.installStep(next))
}

func (m Model) restart() (tea.Model, tea.Cmd) {
	for i := range m.items {
		m.items[i].status = statusPending
		m.items[i].err = nil
	}

	m.queue = nil
	m.screen = screenSelect
	m.cursor = m.firstSelectable()
	m.updateBindings()

	return m, nil
}

// View implements tea.Model.
func (m Model) View() tea.View {
	var body string

	switch m.screen {
	case screenSelect:
		body = m.selectView()
	case screenOptions:
		body = m.optionsView()
	case screenConfirm:
		body = m.confirmView()
	case screenInstall:
		body = m.installView()
	case screenDone:
		body = m.doneView()
	case screenError:
		body = m.errorView()
	}

	return tea.NewView(m.styles.app.Render(body + "\n\n" + m.help.View(m.keys)))
}

// updateBindings enables only the keys the current screen responds to, which
// keeps the help line honest for free.
func (m *Model) updateBindings() {
	selecting := m.screen == screenSelect
	choosing := m.screen == screenOptions
	hasSelection := m.selectedCount() > 0

	m.keys.Up.SetEnabled(selecting || choosing)
	m.keys.Down.SetEnabled(selecting || choosing)
	m.keys.Toggle.SetEnabled(selecting || choosing)

	// The sideways keys only mean anything where a row has more than two
	// states, so they stay out of the help line unless a select is on screen.
	m.keys.Prev.SetEnabled(choosing && m.hasSelect())
	m.keys.Next.SetEnabled(choosing && m.hasSelect())

	m.keys.All.SetEnabled(selecting)
	m.keys.None.SetEnabled(selecting)
	m.keys.Confirm.SetEnabled((selecting && hasSelection) || choosing)

	m.keys.Install.SetEnabled(m.screen == screenConfirm)
	m.keys.Back.SetEnabled(m.screen == screenConfirm || choosing)
	m.keys.Restart.SetEnabled(m.screen == screenDone)
	m.keys.Quit.SetEnabled(m.screen != screenInstall)
}

// moveCursor steps the cursor by delta, stopping at the ends of the list.
func (m Model) moveCursor(delta int) int {
	next := m.cursor + delta
	if next < 0 || next >= len(m.items) {
		return m.cursor
	}

	return next
}

// firstSelectable is where the cursor starts: the first available component, or
// the top of the list if nothing is available.
func (m Model) firstSelectable() int {
	for i, it := range m.items {
		if it.component.Available {
			return i
		}
	}

	return 0
}

func (m Model) selectedCount() int {
	var n int
	for _, it := range m.items {
		if it.selected {
			n++
		}
	}

	return n
}

func (m Model) availableCount() int {
	var n int
	for _, it := range m.items {
		if it.component.Available {
			n++
		}
	}

	return n
}

// contentWidth is the width available inside the app padding.
func (m Model) contentWidth() int {
	const padding = 4

	if m.width <= padding {
		return 40
	}

	return m.width - padding
}

// heading renders a screen title with a rule under it.
func (m Model) heading(text string) string {
	rule := strings.Repeat("─", min(m.contentWidth(), max(lipgloss.Width(text), 24)))

	return m.styles.title.Render(text) + "\n" + m.styles.dimmed.Render(rule)
}
