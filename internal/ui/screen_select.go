package ui

import (
	"fmt"
	"strings"
)

// selectView is the component checklist: the first thing the user sees.
func (m Model) selectView() string {
	var b strings.Builder

	b.WriteString(m.heading(m.theme.Title()))
	b.WriteString("\n")

	if m.theme.Description != "" {
		b.WriteString(m.styles.subtitle.Render(m.theme.Description))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.heading.Render("Choose what to install"))
	b.WriteString("\n\n")

	for i, it := range m.items {
		b.WriteString(m.itemRow(i, it))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.selectFooter())

	return b.String()
}

// itemRow renders one line of the checklist: cursor, checkbox, name, and either
// a description or an unavailable marker.
func (m Model) itemRow(i int, it item) string {
	cursor := "  "
	if i == m.cursor {
		cursor = m.styles.accent.Render("❯ ")
	}

	if !it.component.Available {
		return cursor + m.styles.dimmed.Render(fmt.Sprintf(
			"[-] %-20s %s",
			it.component.Name,
			"unavailable — missing "+it.component.Source,
		))
	}

	box := "[ ]"
	style := m.styles.item
	if it.selected {
		box = "[x]"
		style = m.styles.selected
	}

	row := style.Render(fmt.Sprintf("%s %-20s", box, it.component.Name))
	if it.component.Description != "" {
		row += " " + m.styles.dimmed.Render(it.component.Description)
	}

	return cursor + row
}

// selectFooter explains the current selection, and says so plainly when there is
// nothing to install rather than leaving enter looking broken.
func (m Model) selectFooter() string {
	switch {
	case m.availableCount() == 0:
		return m.styles.warning.Render(
			"No components found in " + m.theme.AssetDir + " — nothing can be installed.",
		)

	case m.selectedCount() == 0:
		return m.styles.dimmed.Render("Nothing selected. Pick at least one component to continue.")

	default:
		return m.styles.subtitle.Render(fmt.Sprintf(
			"%d of %d components selected",
			m.selectedCount(), m.availableCount(),
		))
	}
}
