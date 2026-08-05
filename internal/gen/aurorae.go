package main

import (
	"fmt"
	"strings"
)

// Aurorae button glyphs. These are artwork rather than anything derived from a
// token, so they are held verbatim and only their size, colour and state
// opacities are decided here.
const (
	glyphClose    = "M205.66,194.34a8,8,0,0,1-11.32,11.32L128,139.31,61.66,205.66a8,8,0,0,1-11.32-11.32L116.69,128,50.34,61.66A8,8,0,0,1,61.66,50.34L128,116.69l66.34-66.35a8,8,0,0,1,11.32,11.32L139.31,128Z"
	glyphMinimize = "M224,128a8,8,0,0,1-8,8H40a8,8,0,0,1,0-16H216A8,8,0,0,1,224,128Z"
	glyphMaximize = "M208,32H48A16,16,0,0,0,32,48V208a16,16,0,0,0,16,16H208a16,16,0,0,0,16-16V48A16,16,0,0,0,208,32Zm0,176H48V48H208V208Z"
	glyphRestore  = "M216,32H88a8,8,0,0,0-8,8V80H40a8,8,0,0,0-8,8V216a8,8,0,0,0,8,8H168a8,8,0,0,0,8-8V176h40a8,8,0,0,0,8-8V40A8,8,0,0,0,216,32ZM160,208H48V96H160Zm48-48H176V88a8,8,0,0,0-8-8H96V48H208Z"
)

// decorationNote explains why the bottom corners are square. It is reproduced
// verbatim because the constraint it records is not obvious from the artwork,
// and someone will otherwise try to round them again.
const decorationNote = `<!-- Corner tiles are 8 wide so the top two corners carry an 8px radius. Each
     corner draws the border colour first and lays the background over it inset
     by 1px at radius 7, so the two antialiased edges stay concentric and no
     seam shows along the curve.

     The bottom corners are square, and deliberately so. Rounding them needs a
     bottom border to draw the curve in, and anything narrower than the radius
     lets the client window's square corner show through it. Breeze avoids that
     by calling KDecoration3::Decoration::setBorderRadius, which makes KWin clip
     the client itself — neither Aurorae plugin exposes that API, so an SVG
     theme cannot round the bottom without a visible bottom strip. -->`

// button renders one Aurorae titlebar button: a 24x24 canvas holding four
// states. The resting and deactivated states show only the glyph; hover and
// pressed lay a plate beneath it, rounded only if the style asks for it.
type button struct {
	Glyph string

	// GlyphSize is how big the symbol should be on screen, in pixels, and Box is
	// the square the tile is scaled into. Aurorae stretches the 24x24 tile to the
	// button box, so a size given in tile units would mean something different
	// for every button size; giving it in rendered pixels means the number in the
	// tokens is the number you measure.
	GlyphSize float64
	Box       float64

	PlateFill      string
	HoverOpacity   string
	PressedOpacity string
	Radius         float64

	GlyphFill  string
	RestOpen   string // glyph opacity at rest
	DimOpacity string // glyph opacity when the window is inactive
}

func (b button) render() string {
	// A fully transparent rect keeps the button's hit area at full tile size
	// whatever the glyph covers.
	hit := `<rect x="0" y="0" width="24" height="24" fill="#000" fill-opacity="0"/>`

	// The glyph paths are drawn on a 256-unit grid. Converting the wanted screen
	// size back into tile units undoes the stretch Aurorae will apply, and the
	// inset that centres it is half of whatever the tile has left over.
	glyph := func(opacity string) string {
		tile := b.GlyphSize * 24 / b.Box
		inset := (24 - tile) / 2

		return fmt.Sprintf(
			`<g transform="translate(%s,%s) scale(%s)"><path d="%s" fill="%s" opacity="%s"/></g>`,
			n(inset), n(inset), exact(tile/256), b.Glyph, b.GlyphFill, opacity)
	}
	plate := func(opacity string) string {
		corners := ""
		if b.Radius > 0 {
			corners = fmt.Sprintf(` rx="%s" ry="%s"`, n(b.Radius), n(b.Radius))
		}

		return fmt.Sprintf(`<rect x="0" y="0" width="24" height="24" fill="%s"%s opacity="%s"/>`,
			b.PlateFill, corners, opacity)
	}
	group := func(id string, body ...string) string {
		return fmt.Sprintf(`<g id="%s-center">%s</g>`, id, strings.Join(body, ""))
	}

	var s strings.Builder

	s.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24">` + "\n")
	s.WriteString(group("active", hit, glyph(b.RestOpen)))
	s.WriteString(group("hover", hit, plate(b.HoverOpacity), glyph("1")))
	s.WriteString(group("pressed", hit, plate(b.PressedOpacity), glyph("1")))
	s.WriteString(group("deactivated", hit, glyph(b.DimOpacity)))
	s.WriteString("\n</svg>\n")

	return s.String()
}

// circleButton renders a macOS-style titlebar light: a filled circle, grey at
// rest and coloured on hover, with no glyph at any point.
//
// Hover is per-button and cannot be otherwise. On macOS, pointing at any of the
// three lights up all three; Aurorae renders each button from its own SVG with
// no knowledge of its neighbours, so here only the one under the pointer takes
// colour. That is a limit of the format, not a choice.
type circleButton struct {
	Radius  float64
	Rest    string // grey, window focused and pointer elsewhere
	Dim     string // window unfocused
	Hover   string // the light's own colour
	Pressed string // opacity applied to the hover colour
}

func (c circleButton) render() string {
	hit := `<rect x="0" y="0" width="24" height="24" fill="#000" fill-opacity="0"/>`

	circle := func(fill, opacity string) string {
		alpha := ""
		if opacity != "" {
			alpha = fmt.Sprintf(` opacity="%s"`, opacity)
		}

		return fmt.Sprintf(`<circle cx="12" cy="12" r="%s" fill="%s"%s/>`, n(c.Radius), fill, alpha)
	}
	group := func(id string, body ...string) string {
		return fmt.Sprintf(`<g id="%s-center">%s</g>`, id, strings.Join(body, ""))
	}

	var s strings.Builder

	s.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24">` + "\n")
	s.WriteString(group("active", hit, circle(c.Rest, "")))
	s.WriteString(group("hover", hit, circle(c.Hover, "")))
	s.WriteString(group("pressed", hit, circle(c.Hover, c.Pressed)))
	s.WriteString(group("deactivated", hit, circle(c.Dim, "")))
	s.WriteString("\n</svg>\n")

	return s.String()
}

// auroraeRC renders the decoration's layout file. Its metrics depend on both
// the decoration shape and the button style, which is why it is the one file
// resolved from a pair of axes rather than laid down by an overlay.
func auroraeRC(palette map[string]string, style buttonStyle, titleHeight int) string {
	// Padding is the frame Aurorae leaves around the decoration for shadows and
	// resize handles, and the title edges are the insets inside it.
	const (
		padding         = 1
		titleEdgeTop    = 0
		titleEdgeBottom = 0
		titleEdgeLeft   = 8
		titleEdgeRight  = 6
	)

	// Aurorae places a button at ButtonMarginTop from the top of the titlebar and
	// leaves the rest of the slack below it, so a zero margin sits every button
	// high. Deriving the margin centres whatever height a style asks for.
	marginTop := (titleHeight-style.Height)/2 + style.NudgeTop

	// A maximised window is laid out from an entirely separate set of keys, and
	// every one of them defaults to zero rather than to its ordinary
	// counterpart. AuroraeButtonGroup.qml computes the button offset as
	//
	//     maximised ? titleEdgeTopMaximized + buttonMarginTopMaximized
	//               : titleEdgeTop + padding.top + buttonMarginTop
	//
	// so leaving them unset moved every button on maximise.
	//
	// The maximised keys mirror the ordinary ones exactly, and deliberately do
	// not add the padding the other branch adds. Padding is the frame outside the
	// window, so the decoration's origin sits that far out from the window edge
	// when a window is not maximised and exactly on it when it is: the ordinary
	// branch adds padding to reach the same place the maximised branch already
	// starts from. Adding it to both put every button a pixel out.
	//
	// The edges also feed the titlebar height — borderTopMaximized is
	// titleEdgeTopMaximized + TitleHeight + titleEdgeBottomMaximized — so a
	// padded edge would additionally make a maximised titlebar 2px taller than a
	// restored one.
	maxTop := titleEdgeTop
	maxBottom := titleEdgeBottom
	maxLeft := titleEdgeLeft
	maxRight := titleEdgeRight

	// The application icon is the only button whose width is set separately, and
	// the left edge is wider than the right so it is not tucked into the corner.
	menuWidth := style.MenuWidth
	if menuWidth == 0 {
		menuWidth = style.Width
	}

	return fmt.Sprintf(`[General]
ActiveTextColor=%s
InactiveTextColor=%s
TitleAlignment=Center
TitleVerticalAlignment=Center
Animation=0

[Layout]
BorderLeft=0
BorderRight=0
BorderBottom=0
TitleEdgeTop=%d
TitleEdgeBottom=%d
TitleEdgeLeft=%d
TitleEdgeRight=%d
TitleEdgeTopMaximized=%d
TitleEdgeBottomMaximized=%d
TitleEdgeLeftMaximized=%d
TitleEdgeRightMaximized=%d
TitleBorderLeft=4
TitleBorderRight=4
TitleHeight=%d
ButtonWidth=%d
ButtonWidthMenu=%d
ButtonHeight=%d
ButtonSpacing=0
ButtonMarginTop=%d
ButtonMarginTopMaximized=%d
ExplicitButtonSpacer=6
PaddingTop=%d
PaddingBottom=%d
PaddingLeft=%d
PaddingRight=%d
`, rgb(palette["text"]), rgb(palette["textInactive"]),
		titleEdgeTop, titleEdgeBottom, titleEdgeLeft, titleEdgeRight,
		maxTop, maxBottom, maxLeft, maxRight,
		titleHeight, style.Width, menuWidth, style.Height,
		marginTop, marginTop,
		padding, padding, padding, padding)
}

// lookAndFeelDefaults renders the settings KDE applies when the global theme is
// chosen. The accent lives here, and so would a button order: Mac-style buttons
// sit on the left, which is a KWin setting rather than anything in the Aurorae
// theme. An empty order leaves KWin's own default in place.
func lookAndFeelDefaults(accent, buttonsOnLeft, buttonsOnRight string) string {
	var b strings.Builder

	fmt.Fprintf(&b, `[kdeglobals][General]
ColorScheme=VanillaBoxDark
accentColorFromWallpaper=false
AccentColor=%s

[plasmarc][Theme]
name=vanilla-box-dark

[kwinrc][org.kde.kdecoration2]
library=org.kde.kwin.aurorae.v2
theme=__aurorae__svg__VanillaBoxDark
BorderSize=None
BorderSizeAuto=false
`, rgb(accent))

	if buttonsOnLeft != "" || buttonsOnRight != "" {
		fmt.Fprintf(&b, "ButtonsOnLeft=%s\nButtonsOnRight=%s\n", buttonsOnLeft, buttonsOnRight)
	}

	b.WriteString(`
[kcminputrc][Mouse]
cursorTheme=McMojave-cursors
cursorSize=24

[ksplashrc][KSplash]
Theme=None
Engine=none
`)

	return b.String()
}

// decoration renders the window frame: a titlebar row over left, centre and
// right body tiles, drawn once for the focused window and once for an unfocused
// one. The border is painted first and the background laid over it inset by a
// pixel, so the two antialiased curves stay concentric.
type decoration struct {
	Width   int
	TitleH  int // titlebar row including its top border
	BodyH   int // height of the left/centre/right tiles
	Step    int // distance from the active state to the inactive one
	Radius  float64
	Border  string
	Backgnd string
}

func (d decoration) render() string {
	var b strings.Builder

	// One state occupies the titlebar, the body and the bottom border; the
	// canvas holds the inactive copy a full step below the active one.
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`+"\n",
		d.Width, d.Step+d.TitleH+d.BodyH+1)
	b.WriteString(decorationNote + "\n")

	for _, state := range []struct {
		Prefix string
		Y      int
	}{{"", 0}, {"inactive-", d.Step}} {
		for _, el := range d.state(state.Prefix, state.Y) {
			b.WriteString(el + "\n")
		}
	}

	b.WriteString("</svg>\n")

	return b.String()
}

func (d decoration) state(prefix string, y int) []string {
	w := d.Width
	title := y + d.TitleH
	bottom := title + d.BodyH

	id := func(name string) string { return "decoration-" + prefix + name }

	border := fmt.Sprintf(`fill="%s"`, d.Border)
	backgnd := fmt.Sprintf(`class="ColorScheme-Background" fill="%s"`, d.Backgnd)

	outerL, outerR, innerL, innerR := d.corners(y)

	return []string{
		fmt.Sprintf(`<g id="%s"><path d="%s" %s/><path d="%s" %s/></g>`,
			id("topleft"), outerL, border, innerL, backgnd),
		fmt.Sprintf(`<g id="%s"><rect x="8" y="%d" width="24" height="1" %s/><rect x="8" y="%d" width="24" height="%d" %s/></g>`,
			id("top"), y, border, y+1, d.TitleH-1, backgnd),
		fmt.Sprintf(`<g id="%s"><path d="%s" %s/><path d="%s" %s/></g>`,
			id("topright"), outerR, border, innerR, backgnd),
		fmt.Sprintf(`<g id="%s"><rect x="0" y="%d" width="1" height="%d" %s/><rect x="1" y="%d" width="7" height="%d" %s/></g>`,
			id("left"), title, d.BodyH, border, title, d.BodyH, backgnd),
		fmt.Sprintf(`<rect id="%s" x="8" y="%d" width="24" height="%d" %s/>`,
			id("center"), title, d.BodyH, backgnd),
		fmt.Sprintf(`<g id="%s"><rect x="32" y="%d" width="7" height="%d" %s/><rect x="%d" y="%d" width="1" height="%d" %s/></g>`,
			id("right"), title, d.BodyH, backgnd, w-1, title, d.BodyH, border),
		fmt.Sprintf(`<rect id="%s" x="0" y="%d" width="8" height="1" %s/>`, id("bottomleft"), bottom, border),
		fmt.Sprintf(`<rect id="%s" x="8" y="%d" width="24" height="1" %s/>`, id("bottom"), bottom, border),
		fmt.Sprintf(`<rect id="%s" x="32" y="%d" width="8" height="1" %s/>`, id("bottomright"), bottom, border),
	}
}

// corners returns the outer border path and the inset background path for both
// top corners. At radius zero they degenerate to straight lines, which is what
// the square decoration variant wants.
func (d decoration) corners(y int) (outerL, outerR, innerL, innerR string) {
	w := float64(d.Width)
	title := float64(y + d.TitleH)
	top := float64(y)
	r := d.Radius

	if r == 0 {
		outerL = fmt.Sprintf("M0,%s L0,%s L8,%s L8,%s Z", n(title), n(top), n(top), n(title))
		outerR = fmt.Sprintf("M%s,%s L%s,%s L32,%s L32,%s Z", n(w), n(title), n(w), n(top), n(top), n(title))
		innerL = fmt.Sprintf("M1,%s L1,%s L8,%s L8,%s Z", n(title), n(top+1), n(top+1), n(title))
		innerR = fmt.Sprintf("M%s,%s L%s,%s L32,%s L32,%s Z", n(w-1), n(title), n(w-1), n(top+1), n(top+1), n(title))

		return outerL, outerR, innerL, innerR
	}

	c := r * cornerFactor
	ri := r - 1
	ci := ri * cornerFactor

	outerL = fmt.Sprintf("M0,%s L0,%s C0,%s %s,%s %s,%s L%s,%s Z",
		n(title), n(top+r), n(top+c), n(c), n(top), n(r), n(top), n(r), n(title))
	outerR = fmt.Sprintf("M%s,%s L%s,%s C%s,%s %s,%s %s,%s L%s,%s Z",
		n(w), n(title), n(w), n(top+r), n(w), n(top+c), n(w-c), n(top), n(w-r), n(top), n(w-r), n(title))
	innerL = fmt.Sprintf("M1,%s L1,%s C1,%s %s,%s %s,%s L%s,%s Z",
		n(title), n(top+1+ri), n(top+1+ci), n(1+ci), n(top+1), n(1+ri), n(top+1), n(1+ri), n(title))
	innerR = fmt.Sprintf("M%s,%s L%s,%s C%s,%s %s,%s %s,%s L%s,%s Z",
		n(w-1), n(title), n(w-1), n(top+1+ri), n(w-1), n(top+1+ci), n(w-1-ci), n(top+1), n(w-1-ri), n(top+1), n(w-1-ri), n(title))

	return outerL, outerR, innerL, innerR
}
