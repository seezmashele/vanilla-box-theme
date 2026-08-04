package ui

import (
	"strings"

	"vanillabox/internal/theme"
)

// optionsView is the preferences step: the choices that change what gets
// written, for the components that are actually being installed.
func (m Model) optionsView() string {
	var b strings.Builder

	b.WriteString(m.heading("Preferences"))
	b.WriteString("\n")
	b.WriteString(m.styles.subtitle.Render("How the theme's files should be written"))
	b.WriteString("\n\n")

	for i, o := range m.visibleOptions() {
		cursor := "  "
		if i == m.optionCursor {
			cursor = m.styles.accent.Render("❯ ")
		}

		b.WriteString(cursor + m.optionRow(o))
		b.WriteString("\n")
	}

	return b.String()
}

// optionRow renders one preference. A switch reads as a checkbox; a choice
// reads as its current value between arrows, so the two are distinguishable at
// a glance and it is obvious which rows the sideways keys act on.
func (m Model) optionRow(o theme.Option) string {
	if o.Kind == theme.KindSelect {
		row := m.styles.selected.Render(o.Name) +
			" " + m.styles.accent.Render("‹ "+m.selectedValueName(o)+" ›")

		if o.Description != "" {
			row += " " + m.styles.dimmed.Render(o.Description)
		}

		return row
	}

	box, style := "[ ]", m.styles.item
	if m.choices.Toggles[o.ID] {
		box, style = "[x]", m.styles.selected
	}

	row := style.Render(box + " " + o.Name)
	if o.Description != "" {
		row += " " + m.styles.dimmed.Render(o.Description)
	}

	return row
}

// selectedValueName is the display name of a select's current value, falling
// back to the stored id if the manifest and the choice have drifted apart.
func (m Model) selectedValueName(o theme.Option) string {
	id := m.choices.Values[o.ID]

	for _, v := range o.Values {
		if v.ID == id {
			return v.Name
		}
	}

	return id
}
