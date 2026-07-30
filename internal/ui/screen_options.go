package ui

import (
	"strings"
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

		box := "[ ]"
		style := m.styles.item
		if m.choices[o.ID] {
			box = "[x]"
			style = m.styles.selected
		}

		row := style.Render(box + " " + o.Name)
		if o.Description != "" {
			row += " " + m.styles.dimmed.Render(o.Description)
		}

		b.WriteString(cursor + row)
		b.WriteString("\n")
	}

	return b.String()
}
