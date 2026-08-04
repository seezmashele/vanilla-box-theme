# Variants

How Vanilla Box supports more than one look without turning into a directory of near-identical
copies. This document covers the architecture; [README.md](README.md) covers what the installer
does today.

The goal is that adding a variant is an edit to a token file, not a new tree of artwork, and that
the number of hand-maintained files stays flat as variants are added.

## The governing constraint

Two kinds of variation are possible, and they behave completely differently.

**Colour is a runtime property.** Plasma renders theme SVGs through `KSvg`, which substitutes the
`current-color-scheme` stylesheet at paint time from a `colors` file shipped inside the theme. The
proof ships with Plasma: `/usr/share/plasma/desktoptheme/breeze-dark/` contains exactly three
files — `colors`, `metadata.json`, `plasmarc` — and no artwork at all. It renders Breeze dark
purely by substitution.

**Geometry is not.** A corner radius lives in path data (`d="M0,8 C0,3.582 3.582,0 8,0"`) and in
the tile offsets around it. `KSvg` draws through `QSvgRenderer`, which implements roughly SVG 1.2
Tiny: no CSS custom properties, no stylable `rx`, no dependable `<use>`. There is no runtime knob
for shape, and no SVG trick will produce one. Square artwork means different bytes.

Treating both as one problem multiplies: 3 tints x 2 surface shapes x 2 decoration shapes x 2
button styles is 24 combinations of 25 artwork files. Separating them makes the cost additive
instead, because **each axis owns a disjoint set of files**.

## The axes

| Axis | Default | Mechanism | Files it owns |
| --- | --- | --- | --- |
| Tint | `neutral` | runtime | the two `colors` files |
| Surface shape | `rounded` | baked | `panel-background`, `dialogs/background`, `widgets/background`, `button`, `tooltip` |
| Decoration shape | `square` | baked | `aurorae/decoration.svg` |
| Button style | `windows` | baked | `aurorae/{close,minimize,maximize,restore}.svg` |
| Transparency (x4) | all on | per-file overlay | one file each, from `opaque/` |

Window decorations are square by default and panels and popups are rounded. These are separate
axes precisely so that default is expressible.

Square decorations also retire a compromise. The comment in `aurorae/themes/VanillaBoxDark/
decoration.svg` explains that Aurorae cannot round the bottom corners of a window — rounding needs
a bottom border to draw the curve into, and anything narrower than the radius lets the client's
square corner show through. That limitation now applies only to the non-default rounded variant.

Only one file is claimed by two axes: the Aurorae `VanillaBoxDarkrc`, whose `[Layout]` metrics
depend on both decoration shape and button style. See [Resolved files](#resolved-files).

## Colour

### Prerequisite

The runtime mechanism does not work on the current artwork. The backgrounds carry the class but
hardcode the fill, so `currentColor` is never consulted and a `colors` file would be silently
ignored for exactly the surfaces a tint needs to reach:

```
widgets/background.svg        class="ColorScheme-Background" style="fill:#292929;fill-opacity:0.85"
widgets/panel-background.svg  (no class at all)              style="fill:#292929;fill-opacity:0.85"
```

`widgets/tooltip.svg` already does it correctly (`class="ColorScheme-Background"` with
`style="fill:currentColor"`) and is the model. Nine files need converting: `widgets/background.svg`,
`widgets/panel-background.svg` and `dialogs/background.svg`, plus their `opaque/` and `solid/`
copies. `fill-opacity` is a separate attribute and is unaffected, so the translucency design and
the overlay scheme survive untouched.

### Two consumers, two files

| File | Consumer | Controls |
| --- | --- | --- |
| `color-schemes/VanillaBoxDark.colors` | Qt/KDE apps | window and view backgrounds, menus, buttons, text |
| `plasma/desktoptheme/vanilla-box-dark/colors` | Plasma shell | panel, popups, dialogs, tooltips |

The second file does not exist yet and must be added.

### Rules for the generator

- `Window`, `Header` and `Complementary` backgrounds move together within a tint. Plasma resolves
  a surface against one of the three depending on the widget's colour set; keeping them identical
  means never having to work out which. The current scheme already satisfies this at `41,41,41`.
- `.ColorScheme-ButtonHover` reads `[Colors:Button] DecorationHover`. `button.svg` says `#9e9e9e`
  and the scheme says `158,158,158`; both must be emitted from one token so they cannot drift.
- The colours embedded in each SVG's `current-color-scheme` block are fallbacks — what an editor
  shows. They should stay accurate for the neutral tint but do not drive rendering.

Palettes are written as explicit values per tint, not derived by a hue or chroma transform. A
computed tint behaves badly at the lightness of `#141414`, and the near-blacks want hand-tuning.
Three tints of eight roles is twenty-four numbers in one file.

## Buttons

The button style axis is not only artwork.

**Order is a KWin setting.** Mac-style buttons sit on the left, which lives in `kwinrc
[org.kde.kdecoration2] ButtonsOnLeft` / `ButtonsOnRight` — that is, in the look-and-feel `defaults`
file, which therefore becomes generated. The shape of the change is `ButtonsOnLeft=XIA` with an
empty `ButtonsOnRight`, against a default of `ButtonsOnRight=IAX`. **The exact letter codes are
unverified** — confirm them once against System Settings -> Window Decorations before relying on
them.

**Metrics differ.** `ButtonWidth=28 ButtonHeight=26` suits glyph buttons; traffic lights want
roughly 16px circles and tighter spacing.

**The interaction model inverts.** The current Windows-style buttons are monochrome at rest and
reveal colour on hover. Traffic lights are coloured at rest and reveal a glyph on hover. This is a
per-style token pair, not a shared pattern with different values.

## Transparency

The four toggles are per-surface, and they are the four files already in `opaque/`:

| Toggle | File | Covers |
| --- | --- | --- |
| Panel | `widgets/panel-background.svg` | the panel strip |
| Popups & menus | `dialogs/background.svg` | the application launcher **and** system tray popups |
| Applets | `widgets/background.svg` | plasmoid content areas |
| Tooltips | `widgets/tooltip.svg` | hover tooltips |

The launcher and the tray popups cannot be separated. Both are `PlasmaCore.Dialog` instances and
Plasma ships exactly one `dialogs/background` for all of them; the surfaces are not distinguishable
at the artwork layer by any theme. The option is worded "Popups & menus" so the grouping is honest
in the UI.

`widgets/tasks.svg` remains excluded, for the reason given in the README: its `0.3`/`0.4` values
are white highlights drawn on the panel, not backgrounds.

Overlay granularity changes from directory to file. A whole-directory overlay would need one
directory per combination — sixteen for four independent toggles. Instead each toggle names the
single file it replaces, drawn from the same `opaque/` tree. `opaque/` and `solid/` continue to be
installed wholesale regardless of any toggle, because Plasma falls back to those prefixes itself
when compositing is off.

## Tokens

`spec/tokens.json` is the only file edited to add or adjust a variant.

```json
{
  "palettes": {
    "neutral": { "base":"#292929", "elevated":"#2f2f2f", "view":"#141414",
                 "text":"#e8e4dd", "highlight":"#8f8f8f", "border":"#383838" },
    "slate":   { "base":"#272a2e", "…": "…" },
    "rose":    { "base":"#2c2727", "…": "…" }
  },
  "surfaceShape": {
    "rounded": { "panel":8, "popup":8, "button":6 },
    "square":  { "panel":0, "popup":0, "button":0 }
  },
  "decorationShape": {
    "square":  { "titlebar":0 },
    "rounded": { "titlebar":8 }
  },
  "buttonStyle": {
    "windows": { "w":28, "h":26, "restColor":null,      "hoverColor":"#e0655f" },
    "mac":     { "w":16, "h":16, "restColor":"traffic", "glyphOnHover":true }
  },
  "opacity": { "panel":0.85, "popup":0.85, "applet":0.85, "tooltip":1.0 }
}
```

## Layout

Artwork becomes a build product. The SVGs are already mechanically regular — `button.svg` is nine
tiles across four states, `tasks.svg` is five edges by six states by nine tiles at computed offsets
— and nobody should be hand-editing a 210x150 coordinate grid, let alone two shape variants of one.

```
spec/
  tokens.json               the only hand-edited variant input
  templates/                one template per widget, radius and colour as parameters
internal/gen/               go:generate -> assets/
assets/                     generated; committed
  variants/
    tint-slate/             two colors files
    tint-rose/              two colors files
    surfaces-square/        the five surface SVGs
    deco-rounded/           decoration.svg
    buttons-mac/            the four button SVGs
    rc/<deco>-<buttons>/    VanillaBoxDarkrc
    defaults/<tint>-<buttons>/  look-and-feel defaults
```

Generated assets are committed. The README promises `assets/` ships beside the binary, the tests
install the real shipped artwork rather than a fixture, and a contributor should be able to run the
installer without a generate step. `.gitignore` therefore stays minimal and deliberately does not
list `assets/`.

## Manifest

`Option` becomes typed. A `select` chooses among named values; a `toggle` stays boolean.

```json
"options": [
  { "id":"tint", "name":"Colour", "kind":"select", "default":"neutral",
    "values":[ { "id":"neutral", "name":"Neutral" },
               { "id":"slate",   "name":"Slate",  "overlay":"variants/tint-slate" },
               { "id":"rose",    "name":"Rose",   "overlay":"variants/tint-rose" } ] },

  { "id":"transparency-panel", "name":"Translucent panel", "kind":"toggle", "default":true,
    "overlayWhenOff": { "from":"opaque", "files":["widgets/panel-background.svg"] } }
]
```

An option still never edits a file. It only chooses which pre-generated bytes get copied, which is
the same guarantee the transparency option makes today.

### Resolved files

Two files depend on a combination of axes rather than a single one. Rather than layer overlays and
depend on ordering, they are generated per combination and selected by substituting option ids into
a path:

```json
"resolved": [
  { "source":"variants/rc/{decorationShape}-{buttonStyle}/VanillaBoxDarkrc",
    "target":"aurorae/themes/VanillaBoxDark/VanillaBoxDarkrc" },
  { "source":"variants/defaults/{tint}-{buttonStyle}/defaults",
    "target":"plasma/look-and-feel/org.vanillabox.dark/contents/defaults" }
]
```

That is four rc files and six defaults files, all tiny ini.

## What gets installed

Unchanged: one colour scheme, one Plasma style, one Aurorae theme, one look-and-feel package.
Variants never multiply what lands in `~/.local/share`; they only decide which bytes are copied.
Backups continue to work as described in the README.

## Open questions

- **Does Aurorae substitute colours at runtime?** The five Aurorae SVGs have no
  `current-color-scheme` block, and no system Aurorae theme is installed to compare against. If it
  does not, tint multiplies the *generated* titlebar files by three; hand-maintained files stay
  flat either way, so the risk is contained. Worth testing early regardless.
- **Does the accent colour tint?** `AccentColor=174,142,108` is tan. Against a rose or slate
  background it may read as a clash rather than a tint. If the accent is constant across tints,
  `defaults` depends only on button style and drops from six files to two.
- **The KWin button letter codes**, as above.
