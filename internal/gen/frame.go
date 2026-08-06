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

	// Border is the opacity of a one-pixel outline drawn in the colour scheme's
	// text colour, written literally as the artwork writes its opacities, or
	// empty for a frame with no outline. The outline is painted first and the
	// background laid over it inset by a pixel, so the two antialiased curves
	// stay concentric — the same idiom the window decoration uses. A literal
	// colour would bake one palette's border into artwork every palette shares,
	// so the outline goes through the stylesheet like everything else here.
	Border         string
	BorderFallback string // stylesheet colour for the outline

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
	if f.Border != "" {
		fmt.Fprintf(&b, ".ColorScheme-Text { color:%s; }\n", f.BorderFallback)
	}
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

	for _, tile := range f.tiles(paint, f.Border) {
		b.WriteString(tile + sep)
	}
	if f.Inline {
		b.WriteString("\n")
	}

	// The mask is the blur region rather than artwork, so it stays the frame's
	// full outer shape: an outline drawn into it would punch a ring out of the
	// blur instead of showing up as a border.
	if f.Mask {
		for _, tile := range f.tiles(`style="fill:#ffffff"`, "") {
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
// then edges, then the centre. border is the outline's opacity, or empty for a
// frame that has none.
func (f frame) tiles(paint, border string) []string {
	t := f.Tile
	s := f.Size
	inner := s - 2*t

	tl, tr, bl, br := f.corners(0)

	rect := func(id string, x, y, w, h int) string {
		return fmt.Sprintf(`<rect id="%s" x="%d" y="%d" width="%d" height="%d" %s/>`, id, x, y, w, h, paint)
	}

	if border == "" {
		group := func(id, d string) string {
			return fmt.Sprintf(`<g id="%s"><path d="%s" %s/></g>`, id, d, paint)
		}

		return []string{
			group("topleft", tl),
			group("topright", tr),
			group("bottomleft", bl),
			group("bottomright", br),
			rect("top", t, 0, inner, t),
			rect("bottom", t, s-t, inner, t),
			rect("left", 0, t, t, inner),
			rect("right", s-t, t, t, inner),
			rect("center", t, t, inner, inner),
		}
	}

	// An outlined tile is two shapes under the one id Plasma looks up: the
	// outline covering the whole tile, and the background over it, pulled a
	// pixel off the frame's outer edge and left flush with the tile boundaries
	// it shares with its neighbours.
	itl, itr, ibl, ibr := f.corners(1)
	edge := fmt.Sprintf(`class="ColorScheme-Text" style="fill:currentColor" opacity="%s"`, border)

	corner := func(id, outline, fill string) string {
		return fmt.Sprintf(`<g id="%s"><path d="%s" %s/><path d="%s" %s/></g>`,
			id, outline, edge, fill, paint)
	}
	strip := func(id string, ox, oy, ow, oh, x, y, w, h int) string {
		return fmt.Sprintf(
			`<g id="%s"><rect x="%d" y="%d" width="%d" height="%d" %s/><rect x="%d" y="%d" width="%d" height="%d" %s/></g>`,
			id, ox, oy, ow, oh, edge, x, y, w, h, paint)
	}

	return []string{
		corner("topleft", tl, itl),
		corner("topright", tr, itr),
		corner("bottomleft", bl, ibl),
		corner("bottomright", br, ibr),
		strip("top", t, 0, inner, 1, t, 1, inner, t-1),
		strip("bottom", t, s-1, inner, 1, t, s-t, inner, t-1),
		strip("left", 0, t, 1, inner, 1, t, t-1, inner),
		strip("right", s-1, t, 1, inner, s-t, t, t-1, inner),
		rect("center", t, t, inner, inner),
	}
}

// corners returns the four corner paths, pulled in from the frame's outer edge
// by inset pixels.
//
// The idiom is picked from the frame's own radius rather than the inset one. An
// inset path is the same corner drawn a pixel in, and re-deciding on the inset
// radius would hand a bordered frame's inner path to a different template than
// its outer one.
func (f frame) corners(inset float64) (tl, tr, bl, br string) {
	switch {
	case f.Radius == 0:
		// A square corner is the same filled tile in either idiom, and emitting
		// it as a zero-length cubic would ship curves that render flat.
		return f.squareCorners(inset)
	case f.Tile == int(f.Radius):
		return f.panelCorners(inset)
	}

	return f.dialogCorners(inset)
}

// squareCorners builds corners with no radius at all: four plain tiles.
func (f frame) squareCorners(d float64) (tl, tr, bl, br string) {
	s := float64(f.Size)
	t := float64(f.Tile)
	far := s - d // the outer edge, pulled in by the inset

	tl = fmt.Sprintf("M%s,%s L%s,%s L%s,%s L%s,%s Z", n(d), n(t), n(d), n(d), n(t), n(d), n(t), n(t))
	tr = fmt.Sprintf("M%s,%s L%s,%s L%s,%s L%s,%s Z", n(s-t), n(d), n(far), n(d), n(far), n(t), n(s-t), n(t))
	bl = fmt.Sprintf("M%s,%s L%s,%s L%s,%s L%s,%s Z", n(d), n(s-t), n(t), n(s-t), n(t), n(far), n(d), n(far))
	br = fmt.Sprintf("M%s,%s L%s,%s L%s,%s L%s,%s Z",
		n(s-t), n(s-t), n(far), n(s-t), n(far), n(far), n(s-t), n(far))

	return tl, tr, bl, br
}

// dialogCorners builds corners for frames whose tile is larger than the radius,
// leaving a straight run between the end of the arc and the tile boundary.
func (f frame) dialogCorners(d float64) (tl, tr, bl, br string) {
	s := float64(f.Size)
	t := float64(f.Tile)
	r := f.Radius - d
	c := r * cornerFactor
	far := s - d // the outer edge, pulled in by the inset

	tl = fmt.Sprintf("M%s,%s L%s,%s C%s,%s %s,%s %s,%s L%s,%s L%s,%s Z",
		n(d), n(t), n(d), n(d+r), n(d), n(d+c), n(d+c), n(d), n(d+r), n(d), n(t), n(d), n(t), n(t))
	tr = fmt.Sprintf("M%s,%s L%s,%s C%s,%s %s,%s %s,%s L%s,%s L%s,%s Z",
		n(s-t), n(d), n(far-r), n(d), n(far-c), n(d), n(far), n(d+c), n(far), n(d+r), n(far), n(t), n(s-t), n(t))
	bl = fmt.Sprintf("M%s,%s L%s,%s L%s,%s L%s,%s C%s,%s %s,%s %s,%s Z",
		n(d), n(s-t), n(t), n(s-t), n(t), n(far), n(d+r), n(far), n(d+c), n(far), n(d), n(far-c), n(d), n(far-r))
	br = fmt.Sprintf("M%s,%s L%s,%s L%s,%s C%s,%s %s,%s %s,%s L%s,%s Z",
		n(s-t), n(s-t), n(far), n(s-t), n(far), n(far-r), n(far), n(far-c),
		n(far-c), n(far), n(far-r), n(far), n(s-t), n(far))

	return tl, tr, bl, br
}

// panelCorners builds corners for frames whose tile is exactly the radius, where
// the straight run vanishes and the path closes directly off the arc.
func (f frame) panelCorners(d float64) (tl, tr, bl, br string) {
	s := float64(f.Size)
	t := float64(f.Tile)
	r := f.Radius - d
	c := r * cornerFactor
	far := s - d // the outer edge, pulled in by the inset

	tl = fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s L%s,%s Z",
		n(d), n(d+r), n(d), n(d+c), n(d+c), n(d), n(d+r), n(d), n(t), n(t))
	tr = fmt.Sprintf("M%s,%s C%s,%s %s,%s %s,%s L%s,%s Z",
		n(s-t), n(d), n(far-c), n(d), n(far), n(d+c), n(far), n(d+r), n(s-t), n(t))
	bl = fmt.Sprintf("M%s,%s L%s,%s L%s,%s C%s,%s %s,%s %s,%s Z",
		n(d), n(s-t), n(t), n(s-t), n(t), n(far), n(d+c), n(far), n(d), n(far-c), n(d), n(s-t))
	br = fmt.Sprintf("M%s,%s L%s,%s C%s,%s %s,%s %s,%s Z",
		n(s-t), n(s-t), n(far), n(s-t), n(far), n(far-c), n(far-c), n(far), n(s-t), n(far))

	return tl, tr, bl, br
}

// exact formats a value at whatever precision it needs, for the handful of
// numbers that are scale factors rather than coordinates: rounding a scale to
// three decimals resizes what it is applied to.
func exact(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// n formats a coordinate the way the artwork writes them: three decimals at
// most, and no trailing zeros or bare point.
func n(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(s, "0")

	return strings.TrimRight(s, ".")
}
