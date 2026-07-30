package ui

import (
	"fmt"
	"strings"
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
			b.WriteString(m.styles.path.Render("      " + o.Name + ": " + onOff(m.choices[o.ID])))
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

// onOff renders an option's state the way the review screen lists it.
func onOff(on bool) string {
	if on {
		return "on"
	}

	return "off"
}
