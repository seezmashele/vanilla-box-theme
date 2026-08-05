package ui

import "vanillabox/internal/theme"

// rowKind is what one line of the preferences screen is.
type rowKind int

const (
	// rowHeader names a choice and cannot be acted on; its values follow it.
	rowHeader rowKind = iota
	// rowSpacer is blank. It is a row rather than padding inside another row so
	// that the scroll window counts lines and screen lines identically.
	rowSpacer
	rowValue
	rowToggle
)

// selectable reports whether the cursor may rest on this row.
func (r optionRow) selectable() bool {
	return r.kind == rowValue || r.kind == rowToggle
}

// optionRow is one line of the preferences screen. A choice becomes a header
// followed by one row per value, so every alternative is visible rather than
// hidden behind a control the user has to operate to discover.
type optionRow struct {
	kind   rowKind
	option theme.Option
	value  theme.OptionValue
}

// optionRows flattens the visible preferences into the lines that render.
func (m Model) optionRows() []optionRow {
	var rows []optionRow

	for _, o := range m.visibleOptions() {
		if o.Kind != theme.KindSelect {
			rows = append(rows, optionRow{kind: rowToggle, option: o})

			continue
		}

		if len(rows) > 0 {
			rows = append(rows, optionRow{kind: rowSpacer})
		}
		rows = append(rows, optionRow{kind: rowHeader, option: o})
		for _, v := range o.Values {
			rows = append(rows, optionRow{kind: rowValue, option: o, value: v})
		}
	}

	return rows
}

// firstOptionRow is where the cursor starts: the first line that can be acted
// on, skipping the header the list opens with.
func (m Model) firstOptionRow() int {
	for i, r := range m.optionRows() {
		if r.selectable() {
			return i
		}
	}

	return 0
}

// moveOptionCursor steps the cursor by delta, skipping headers and stopping at
// the ends, then brings the scroll window back over it.
func (m *Model) moveOptionCursor(delta int) {
	rows := m.optionRows()

	for next := m.optionCursor + delta; next >= 0 && next < len(rows); next += delta {
		if !rows[next].selectable() {
			continue
		}

		m.optionCursor = next

		break
	}

	m.scrollToCursor(len(rows))
}

// scrollToCursor moves the window the least amount that puts the cursor back
// inside it. A header immediately above the cursor is pulled in with it, since
// a value row is meaningless without the choice it belongs to.
func (m *Model) scrollToCursor(total int) {
	visible := m.visibleRows(total)
	if visible >= total {
		m.optionScroll = 0

		return
	}

	// Pull the header, and the blank line above it, in with the first value of a
	// group: a value row is meaningless without the choice it belongs to.
	top, rows := m.optionCursor, m.optionRows()
	for top > 0 && !rows[top-1].selectable() {
		top--
	}

	if top < m.optionScroll {
		m.optionScroll = top
	}
	if m.optionCursor >= m.optionScroll+visible {
		m.optionScroll = m.optionCursor - visible + 1
	}

	if max := total - visible; m.optionScroll > max {
		m.optionScroll = max
	}
	if m.optionScroll < 0 {
		m.optionScroll = 0
	}

	// Never begin part-way through a group: a column of unlabelled values is
	// worse than showing one group fewer. The cursor is always at or below the
	// window start, and it is only ever inside this group when the pull above
	// already moved the start to the header, so advancing past the group cannot
	// scroll off the cursor.
	for m.optionScroll > 0 && m.optionScroll < m.optionCursor && rows[m.optionScroll].kind == rowValue {
		m.optionScroll++
	}
}

// visibleRows is how many preference lines fit under the heading and above the
// help line. A terminal that has not reported its size yet — which is every
// terminal until the first resize, and every test — gets the whole list.
func (m Model) visibleRows(total int) int {
	// Heading, rule, subtitle, the blank lines around them, the help line, and
	// the two lines the scroll markers may take.
	const chrome = 11

	if m.height <= 0 {
		return total
	}

	if n := m.height - chrome; n > 0 {
		return n
	}

	return 1
}
