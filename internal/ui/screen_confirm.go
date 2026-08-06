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
		"%d component(s) will be installed:", m.installCount(),
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

	// A component missing from the list above was either turned down or is not
	// in the asset tree, and the two need telling apart: one is the user's doing
	// and the other is a gap in the build. This is the only place either is said.
	for _, it := range m.items {
		if it.selected {
			continue
		}

		reason := "unavailable, will be skipped"
		if it.component.Available {
			reason = "not selected"
		}

		b.WriteString(m.styles.dimmed.Render(
			"  • " + it.component.Name + " — " + reason,
		))
		b.WriteString("\n")
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
