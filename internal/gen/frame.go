package main

import (
	"fmt"
	"strconv"
	"strings"
)

// cornerFactor places a cubic corner's control points, at radius*cornerFactor in
// from the corner. It is the complement of the usual circle-to-cubic constant,
// written directly because the artwork's rounding pins it: only 0.4478
// reproduces both the 3.582 the frames use at radius 8 and the 3.135 the window
// decoration uses at radius 7 for its inset border.
const cornerFactor = 0.4478

// frame describes a nine-tile FrameSvg: four corners, four edges and a centre,
// laid out in a square of Size with tiles Tile across, followed by the margin
// hints Plasma reads the frame's insets from.
//
// Two idioms appear in the theme and they are not interchangeable. Dialog-like
// frames leave a straight run between the corner arc and the tile edge, because
// Tile exceeds Radius. The panel's tiles are exactly the radius, so its corners
// collapse to an arc and a single closing line. Emitting one from the other's
// template would change the path data, so each keeps its own builder.
type frame struct {
	Size   int     // the frame square: 44 for dialogs, 40 for the panel
	Canvas int     // SVG height, which leaves room for the hint row
	Tile   int     // corner and edge tile size
	Radius float64 // corner radius

	Fallback string // stylesheet colour, shown by editors and ignored at paint time
	Opacity  string // fill-opacity, or empty for fully opaque

	Mask     bool // emit the mask- copies used for blur regions
	HintSize int  // size of the four margin hints
	HintY    int  // baseline the hints sit on

	// Inline puts every tile on one line, as the panel artwork does. The
	// difference is cosmetic but it is load-bearing for byte-for-byte
	// regeneration, so it is described rather than normalised away.
	Inline bool
}

func (f frame) render() string {
	var b strings.Builder

	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`+"\n", f.Size, f.Canvas)
	b.WriteString(`<defs><style type="text/css" id="current-color-scheme">` + "\n")
	fmt.Fprintf(&b, ".ColorScheme-Background { color:%s; }\n", f.Fallback)
	b.WriteString("</style></defs>\n")

	fill := "fill:currentColor"
	if f.Opacity != "" {
		fill += ";fill-opacity:" + f.Opacity
	}
	paint := `class="ColorScheme-Background" style="` + fill + `"`

	sep := "\n"
	if f.Inline {
		sep = ""
	}

	for _, tile := range f.tiles(paint) {
		b.WriteString(tile + sep)
	}
	if f.Inline {
		b.WriteString("\n")
	}

	if f.Mask {
		for _, tile := range f.tiles(`style="fill:#ffffff"`) {
			b.WriteString(strings.Replace(tile, `id="`, `id="mask-`, 1) + "\n")
		}
	}

	for i, name := range []string{"top", "bottom", "left", "right"} {
		fmt.Fprintf(&b, `<rect id="hint-%s-margin" x="%d" y="%d" width="%d" height="%d" style="fill:none"/>`,
			name, i*10, f.HintY, f.HintSize, f.HintSize)
		if !f.Inline {
			b.WriteString("\n")
		}
	}
	if f.Inline {
		b.WriteString("\n")
	}

	b.WriteString("</svg>\n")

	return b.String()
}

// tiles returns the nine elements in the order the artwork uses: corners first,
// then edges, then the centre.
func (f frame) tiles(paint string) []string {
	t := f.Tile
	inner := f.Size - 2*t

	corner := f.dialogCorners
	switch {
	case f.Radius == 0:
		// A square corner is the same filled tile in either idiom, and emitting
		// it as a zero-length cubic would ship curves that render flat.
		corner = f.squareCorners
	case t == int(f.Radius):
		corner = f.panelCorners
	}
	tl, tr, bl, br := corner()

	group := func(id, d string) string {
		return fmt.Sprintf(`<g id="%s"><path d="%s" %s/></g>`, id, d, paint)
	}
	rect := func(id string, x, y, w, h int) string {
		return fmt.Sprintf(`<rect id="%s" x="%d" y="%d" width="%d" height="%d" %s/>`, id, x, y, w, h, paint)
	}

	return []string{
		group("topleft", tl),
		group("topright", tr),
		group("bottomleft", bl),
		group("bottomright", br),
		rect("top", t, 0, inner, t),
		rect("bottom", t, f.Size-t, inner, t),
		rect("left", 0, t, t, inner),
		rect("right", f.Size-t, t, t, inner),
		rect("center", t, t, inner, inner),
	}
}

// squareCorners builds corners with no radius at all: four plain tiles.
func (f frame) squareCorners() (tl, tr, bl, br string) {
	s := float64(f.Size)
	t := float64(f.Tile)

	tl = fmt.Sprintf("M0,%s L0,0 L%s,0 L%s,%s Z", n(t), n(t), n(t), n(t))
	tr = fmt.Sprintf("M%s,0 L%s,0 L%s,%s L%s,%s Z", n(s-t), n(s), n(s), n(t), n(s-t), n(t))
	bl = fmt.Sprintf("M0,%s L%s,%s L%s,%s L0,%s Z", n(s-t), n(t), n(s-t), n(t), n(s), n(s))
	br = fmt.Sprintf("M%s,%s L%s,%s L%s,%s L%s,%s Z", n(s-t), n(s-t), n(s), n(s-t), n(s), n(s), n(s-t), n(s))

	return tl, tr, bl, br
}

// dialogCorners builds corners for frames whose tile is larger than the radius,
// leaving a straight run between the end of the arc and the tile boundary.
func (f frame) dialogCorners() (tl, tr, bl, br string) {
	s := float64(f.Size)
	t := float64(f.Tile)
	r := f.Radius
	c := r * cornerFactor

	tl = fmt.Sprintf("M0,%s L0,%s C0,%s %s,0 %s,0 L%s,0 L%s,%s Z",
		n(t), n(r), n(c), n(c), n(r), n(t), n(t), n(t))
	tr = fmt.Sprintf("M%s,0 L%s,0 C%s,0 %s,%s %s,%s L%s,%s L%s,%s Z",
		n(s-t), n(s-r), n(s-c), n(s), n(c), n(s), n(r), n(s), n(t), n(s-t), n(t))
	bl = fmt.Sprintf("M0,%s L%s,%s L%s,%s L%s,%s C%s,%s 0,%s 0,%s Z",
		n(s-t), n(t), n(s-t), n(t), n(s), n(r), n(s), n(c), n(s), n(s-c), n(s-r))
	br = fmt.Sprintf("M%s,%s L%s,%s L%s,%s C%s,%s %s,%s %s,%s L%s,%s Z",
		n(s-t), n(s-t), n(s), n(s-t), n(s), n(s-r), n(s), n(s-c), n(s-c), n(s), n(s-r), n(s), n(s-t), n(s))

	return tl, tr, bl, br
}

// panelCorners builds corners for frames whose tile is exactly the radius, where
// the straight run vanishes and the path closes directly off the arc.
func (f frame) panelCorners() (tl, tr, bl, br string) {
	s := float64(f.Size)
	t := float64(f.Tile)
	r := f.Radius
	c := r * cornerFactor

	tl = fmt.Sprintf("M0,%s C0,%s %s,0 %s,0 L%s,%s Z",
		n(r), n(c), n(c), n(r), n(t), n(t))
	tr = fmt.Sprintf("M%s,0 C%s,0 %s,%s %s,%s L%s,%s Z",
		n(s-t), n(s-c), n(s), n(c), n(s), n(r), n(s-t), n(t))
	bl = fmt.Sprintf("M0,%s L%s,%s L%s,%s C%s,%s 0,%s 0,%s Z",
		n(s-t), n(t), n(s-t), n(t), n(s), n(c), n(s), n(s-c), n(s-t))
	br = fmt.Sprintf("M%s,%s L%s,%s C%s,%s %s,%s %s,%s Z",
		n(s-t), n(s-t), n(s), n(s-t), n(s), n(s-c), n(s-c), n(s), n(s-t), n(s))

	return tl, tr, bl, br
}

// n formats a coordinate the way the artwork writes them: three decimals at
// most, and no trailing zeros or bare point.
func n(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(s, "0")

	return strings.TrimRight(s, ".")
}
