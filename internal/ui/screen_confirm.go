package ui

import (
	"fmt"
	"strings"

	"vanillabox/internal/theme"
)

// confirmView is the last stop before installing: exactly what will be written,
// and where.
func (m Model) confirmView() string {
	var b strings.Builder

	b.WriteString(m.heading("Review"))
	b.WriteString("\n")
	b.WriteString(m.styles.subtitle.Render(fmt.Sprintf(
		"%d component(s) will be installed:", m.selectedCount(),
	)))
	b.WriteString("\n\n")

	for _, it := range m.items {
		if !it.selected {
			continue
		}

		b.WriteString(m.styles.item.Render("  • " + it.component.Name))
		b.WriteString("\n")
		b.WriteString(m.styles.path.Render("      → " + m.theme.DisplayTargetPath(it.component)))
		b.WriteString("\n")

		if cmd := applyCommand(it.component); cmd != "" {
			b.WriteString(m.styles.path.Render("      $ " + cmd))
			b.WriteString("\n")
		}
	}

	if theme.Simulated {
		b.WriteString("\n")
		b.WriteString(m.styles.warning.Render(
			"Simulated run — no files will be written and no commands will run.",
		))
		b.WriteString("\n")
	}

	return b.String()
}

// applyCommand is the KDE command that would be run for a component, rendered
// the way it would be typed.
func applyCommand(c theme.Component) string {
	if c.ApplyCmd == "" {
		return ""
	}

	return strings.TrimSpace(c.ApplyCmd + " " + strings.Join(c.ApplyArgs, " "))
}
