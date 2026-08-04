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

		for _, o := range it.component.Options {
			b.WriteString(m.styles.path.Render("      " + o.Name + ": " + m.optionSummary(o)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.styles.subtitle.Render(
		"Files are copied only. Nothing is applied — you pick the theme in System Settings.",
	))
	b.WriteString("\n")

	return b.String()
}

// optionSummary renders an option's state the way the review screen lists it:
// a switch as on or off, a choice by the name of the value picked.
func (m Model) optionSummary(o theme.Option) string {
	if o.Kind == theme.KindSelect {
		return m.selectedValueName(o)
	}

	if m.choices.Toggles[o.ID] {
		return "on"
	}

	return "off"
}
