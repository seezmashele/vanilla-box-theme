package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The icon theme is written into one scalable directory per context.
//
// Breeze keeps a directory per pixel size because its artwork is drawn again at
// each one. Phosphor is a single 256-unit geometry, so fixed sizes would be the
// same path data copied four times under four different Size= headings. Size
// below is what a lookup that asks for no particular size gets.
const (
	iconRoot    = "assets/icons"
	phosphorDir = "spec/phosphor"

	iconSizeDir = "scalable"
	iconSize    = 22
	iconMinSize = 8
	iconMaxSize = 512

	// iconBox is the viewBox every Phosphor asset is drawn in. Nothing is
	// resampled to another grid: the artwork keeps its own coordinates and the
	// renderer only ever wraps them.
	iconBox = 256.0

	// iconGrid is how much of that box Phosphor's own grid uses: its artwork is
	// inset by 24 units on every side, so an icon drawn out to the grid spans
	// 208 of the 256. Measured rather than documented upstream — power and
	// magnifying-glass both hit it exactly — and it is what turns a glyph size
	// in pixels into a scale.
	iconGrid = 208.0
)

// iconContexts maps a context directory to the Context= the icon theme spec
// wants written for it. A directory not named here is rejected by loadIcons
// rather than written with an empty context, which is a theme that installs and
// then quietly fails to answer.
var iconContexts = map[string]string{
	"actions":    "Actions",
	"apps":       "Applications",
	"categories": "Categories",
	"devices":    "Devices",
	"places":     "Places",
	"status":     "Status",
}

// iconSpec is spec/icons.json: which Phosphor icon answers which KDE icon name,
// plus the vendored artwork those names resolve to.
type iconSpec struct {
	Source iconSource `json:"source"`

	// Color paints the icons outright. Empty leaves them deferring to the
	// colour scheme, which is the KDE default and what every other piece of
	// artwork in this theme does.
	Color string `json:"color"`

	// Glyph is how many pixels an icon filling Phosphor's grid measures in the
	// iconSize box. Held in pixels because that is the decision being made;
	// glyphScale turns it into the transform that produces it.
	Glyph float64 `json:"glyph"`

	Icons    map[string]string `json:"icons"`
	Families []iconFamily      `json:"families"`

	// sources is the vendored artwork keyed by Phosphor name, read at load time
	// so that a mapping naming a source nobody vendored fails the build rather
	// than the install.
	sources map[string]string
}

// iconSource pins where the artwork came from. The ref is a commit rather than
// a branch so that re-vendoring years from now produces the same bytes.
type iconSource struct {
	Repo    string `json:"repo"`
	Ref     string `json:"ref"`
	Weight  string `json:"weight"`
	License string `json:"license"`
}

// iconFamily is the rule behind a numbered set. Breeze spells charge level and
// signal strength as a hundred-odd filenames that collapse to a handful of
// pictures, and a family multiplies them back out.
//
// Name is a template over {level} and one placeholder per slot. A slot
// alternative naming an icon replaces the level's — a charging battery is the
// same picture at every charge — and one naming null keeps it, which is how a
// power profile changes the filename without changing the artwork. Slots are
// applied in name order and the last override wins.
type iconFamily struct {
	Context string            `json:"context"`
	Name    string            `json:"name"`
	Levels  map[string]string `json:"levels"`

	// Slots is slot name -> the literal that replaces its placeholder -> the
	// icon that literal forces, or null to keep the level's.
	Slots map[string]map[string]*string `json:"slots"`
}

// readIconSpec reads the mapping alone. The fetcher wants it before the
// artwork it names exists, which is the one caller that cannot use loadIcons.
func readIconSpec(root string) (*iconSpec, error) {
	path := filepath.Join(root, "spec", "icons.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ic := &iconSpec{}
	if err := json.Unmarshal(data, ic); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return ic, nil
}

// loadIcons reads the mapping and the artwork it names.
func loadIcons(root string) (*iconSpec, error) {
	ic, err := readIconSpec(root)
	if err != nil {
		return nil, err
	}

	names, err := ic.names()
	if err != nil {
		return nil, err
	}

	ic.sources = map[string]string{}

	var missing []string

	for _, phosphor := range names {
		if _, ok := ic.sources[phosphor]; ok {
			continue
		}

		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ic.sourcePath(phosphor))))
		if err != nil {
			missing = append(missing, phosphor)

			continue
		}

		ic.sources[phosphor] = string(body)
	}

	if len(missing) > 0 {
		sort.Strings(missing)

		return nil, fmt.Errorf("%s has no %s — run `go run ./internal/gen -fetch`",
			phosphorDir, strings.Join(missing, ", "))
	}

	return ic, nil
}

// glyphScale is the factor that makes an icon filling Phosphor's grid measure
// Glyph pixels in the iconSize box. Zero leaves the artwork at its own size,
// which is what an icons.json that says nothing about glyphs should mean.
func (ic *iconSpec) glyphScale() float64 {
	if ic.Glyph == 0 {
		return 1
	}

	scale := ic.Glyph / (iconSize * iconGrid / iconBox)

	// Rounded here rather than on the way out, so that the offset centring the
	// artwork is derived from the same number the transform prints. Deriving it
	// from the full-precision scale leaves a transform whose two halves disagree
	// about how much was taken off — by a fraction of a pixel, but visibly so to
	// anyone reading the file and checking the arithmetic.
	return math.Round(scale*1000) / 1000
}

// sourcePath is where one Phosphor icon is vendored, relative to the repository
// root.
func (ic *iconSpec) sourcePath(phosphor string) string {
	return fmt.Sprintf("%s/%s-%s.svg", phosphorDir, phosphor, ic.Source.Weight)
}

// names is every icon the theme answers, keyed "context/name", after the
// families have been multiplied out.
func (ic *iconSpec) names() (map[string]string, error) {
	out := make(map[string]string, len(ic.Icons))

	for name, phosphor := range ic.Icons {
		context, _, ok := strings.Cut(name, "/")
		if !ok {
			return nil, fmt.Errorf("icon %q is not qualified by a context", name)
		}
		if _, ok := iconContexts[context]; !ok {
			return nil, fmt.Errorf("icon %q names unknown context %q", name, context)
		}

		out[name] = phosphor
	}

	for _, f := range ic.Families {
		expanded, err := f.expand()
		if err != nil {
			return nil, err
		}

		for name, phosphor := range expanded {
			out[name] = phosphor
		}
	}

	return out, nil
}

// expand multiplies one family out into the names it stands for.
func (f iconFamily) expand() (map[string]string, error) {
	if _, ok := iconContexts[f.Context]; !ok {
		return nil, fmt.Errorf("family %q names unknown context %q", f.Name, f.Context)
	}

	slots := make([]string, 0, len(f.Slots))
	for slot := range f.Slots {
		slots = append(slots, slot)
	}
	sort.Strings(slots)

	out := map[string]string{}

	for level, levelIcon := range f.Levels {
		// walk fills one slot per step, so that every combination of
		// alternatives is visited exactly once.
		var walk func(depth int, name, icon string) error

		walk = func(depth int, name, icon string) error {
			if depth == len(slots) {
				if open := strings.IndexByte(name, '{'); open >= 0 {
					return fmt.Errorf("family %q has no value for %q", f.Name, name[open:])
				}

				out[f.Context+"/"+name] = icon

				return nil
			}

			slot := slots[depth]

			for literal, override := range f.Slots[slot] {
				next := icon
				if override != nil {
					next = *override
				}

				if err := walk(depth+1, strings.ReplaceAll(name, "{"+slot+"}", literal), next); err != nil {
					return err
				}
			}

			return nil
		}

		if err := walk(0, strings.ReplaceAll(f.Name, "{level}", level), levelIcon); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// icons renders the whole icon theme: every mapped name, its symbolic twin, and
// the index that tells KDE what is in here and what answers everything else.
func icons(tk *tokens, ic *iconSpec) (map[string]string, error) {
	names, err := ic.names()
	if err != nil {
		return nil, err
	}

	root := iconRoot + "/" + tk.Theme.IconsID
	out := map[string]string{}
	contexts := map[string]bool{}

	for name, phosphor := range names {
		art, err := phosphorPaths(ic.sources[phosphor])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ic.sourcePath(phosphor), err)
		}

		context, base, _ := strings.Cut(name, "/")
		contexts[context] = true

		body := icon{
			Paths: art, Fallback: tk.Foreground["text"],
			Color: ic.Color, Scale: ic.glyphScale(),
		}.render()
		dir := root + "/" + context + "/" + iconSizeDir

		// Plasma 6 asks for the symbolic name in most shell contexts and falls
		// through to the inherited theme when it misses. The two are the same
		// picture here: every icon in this set is already monochrome, so a
		// separate symbolic drawing would be the same drawing.
		out[dir+"/"+base+".svg"] = body
		out[dir+"/"+base+"-symbolic.svg"] = body
	}

	dirs := make([]string, 0, len(contexts))
	for context := range contexts {
		dirs = append(dirs, context+"/"+iconSizeDir)
	}
	sort.Strings(dirs)

	out[root+"/index.theme"] = iconIndex(tk.Theme, dirs)

	return out, nil
}

// icon is one KDE icon: Phosphor's path data, either painted a fixed colour or
// wrapped in the stylesheet Plasma recolours through.
//
// The two are exclusive, and that is the whole of it: the stylesheet is how an
// icon asks to be recoloured, so an icon that wants a colour of its own has to
// go without one. Where the stylesheet is used it sits directly under <svg>
// rather than in a <defs> as the rest of the theme's artwork does — that is the
// shape Breeze's icons use, and the icon loader is a different consumer from
// the Plasma style's frame renderer.
type icon struct {
	Paths []string

	// Color paints the artwork outright and leaves the stylesheet out. Empty
	// falls back to the colour-scheme idiom, where Fallback is only what an
	// editor shows and Plasma replaces it at paint time.
	Color    string
	Fallback string

	// Scale shrinks the artwork about the centre of the box. It is applied as a
	// transform rather than by rewriting the path data, so what is committed
	// stays recognisably the vendored source with a wrapper around it.
	Scale float64
}

func (i icon) render() string {
	var b strings.Builder

	fmt.Fprintf(&b, `<svg width="%d" height="%d" viewBox="0 0 256 256" xmlns="http://www.w3.org/2000/svg">`+"\n",
		iconSize, iconSize)

	group := fmt.Sprintf(`    <g fill="%s"`, i.Color)

	if i.Color == "" {
		fmt.Fprintf(&b, `    <style id="current-color-scheme" type="text/css">.ColorScheme-Text { color:%s; }</style>`+"\n",
			i.Fallback)

		group = `    <g class="ColorScheme-Text" fill="currentColor"`
	}

	if i.Scale != 0 && i.Scale != 1 {
		// Half the space the shrink frees goes to each side, which is what keeps
		// the artwork centred instead of pinned to the top left.
		offset := (iconBox / 2) * (1 - i.Scale)
		group += fmt.Sprintf(` transform="translate(%s,%s) scale(%s)"`, n(offset), n(offset), n(i.Scale))
	}

	b.WriteString(group + ">\n")

	for _, p := range i.Paths {
		fmt.Fprintf(&b, "        %s\n", p)
	}

	b.WriteString("    </g>\n</svg>\n")

	return b.String()
}

// phosphorPaths pulls the drawing out of a vendored source.
//
// The artwork is parsed rather than pattern-matched because a filled icon is
// not always one path: a few carry a second with its own fill-rule, and a
// regular expression that dropped it would produce an icon missing a hole.
func phosphorPaths(source string) ([]string, error) {
	var art struct {
		Paths []struct {
			D        string `xml:"d,attr"`
			FillRule string `xml:"fill-rule,attr"`
		} `xml:"path"`
	}

	if err := xml.Unmarshal([]byte(source), &art); err != nil {
		return nil, err
	}
	if len(art.Paths) == 0 {
		return nil, fmt.Errorf("no path in the source")
	}

	out := make([]string, 0, len(art.Paths))

	for _, p := range art.Paths {
		if p.FillRule != "" {
			out = append(out, fmt.Sprintf(`<path fill-rule="%s" d="%s"/>`, p.FillRule, p.D))

			continue
		}

		out = append(out, fmt.Sprintf(`<path d="%s"/>`, p.D))
	}

	return out, nil
}

// iconIndex renders index.theme. Inherits is the load-bearing line: it is what
// makes a set this size usable at all, by handing everything unmapped to Breeze
// rather than leaving a missing-image square.
func iconIndex(id identity, dirs []string) string {
	var b strings.Builder

	fmt.Fprintf(&b, `[Icon Theme]
Name=%s
Comment=%s
Inherits=breeze-dark,breeze,hicolor
Example=start-here-kde
Directories=%s
`, id.IconsName, id.Description, strings.Join(dirs, ","))

	for _, dir := range dirs {
		context, _, _ := strings.Cut(dir, "/")

		fmt.Fprintf(&b, `
[%s]
Size=%d
MinSize=%d
MaxSize=%d
Context=%s
Type=Scalable
`, dir, iconSize, iconMinSize, iconMaxSize, iconContexts[context])
	}

	return b.String()
}
