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
| Accent | `sand` | runtime | `AccentColor` in the look-and-feel `defaults` |
| Surface shape | `rounded` | baked | `panel-background`, `dialogs/background`, `widgets/background`, `button`, `tooltip` |
| Decoration shape | `square` | baked | `aurorae/decoration.svg` |
| Button style | `windows` | baked | `aurorae/{close,minimize,maximize,restore}.svg` |
| Transparency (x4) | all on | per-file overlay | one file each, from `opaque/` |

Window decorations are square by default and panels and popups are rounded. These are separate
axes precisely so that default is expressible.

Tint and accent are chosen independently — the surface colour and the highlight colour are two
questions, and pairing them would multiply the menu without adding expressiveness. Both are data,
so neither generates a single SVG.

Square decorations also retire a compromise. The comment in `aurorae/themes/VanillaBoxDark/
decoration.svg` explains that Aurorae cannot round the bottom corners of a window — rounding needs
a bottom border to draw the curve into, and anything narrower than the radius lets the client's
square corner show through. That limitation now applies only to the non-default rounded variant.

Only one file is claimed by two axes: the Aurorae `VanillaBoxDarkrc`, whose `[Layout]` metrics
depend on both decoration shape and button style. See [Resolved files](#resolved-files).

## Colour

### Prerequisite — done

The runtime mechanism did not work on the original artwork. The backgrounds carried the class but
hardcoded the fill, so `currentColor` was never consulted and a `colors` file would have been
silently ignored for exactly the surfaces a tint needs to reach:

```
widgets/background.svg        class="ColorScheme-Background" style="fill:#292929;fill-opacity:0.85"
widgets/panel-background.svg  (no class at all)              style="fill:#292929;fill-opacity:0.85"
```

Nine files were converted to `fill:currentColor` — `widgets/background.svg`,
`widgets/panel-background.svg` and `dialogs/background.svg`, plus their `opaque/` and `solid/`
copies — with `widgets/tooltip.svg` as the model. The panel files needed the class adding as well;
they had none. `fill-opacity` is a separate attribute and was unaffected, so the translucency
design and the overlay scheme survived untouched.

Thirteen surfaces now defer to the scheme. `widgets/scrollbar.svg` is among them and is easy to
overlook, which is why `TestShippedStyleFollowsTheColorScheme` asserts the expected set by path
rather than by count.

### Two consumers, two files

| File | Consumer | Controls |
| --- | --- | --- |
| `color-schemes/VanillaBoxDark.colors` | Qt/KDE apps | window and view backgrounds, menus, buttons, text |
| `plasma/desktoptheme/vanilla-box-dark/colors` | Plasma shell | panel, popups, dialogs, tooltips |

### Rules for the generator

- `Window`, `Header` and `Complementary` backgrounds move together within a tint. Plasma resolves
  a surface against one of the three depending on the widget's colour set; keeping them identical
  means never having to work out which. The current scheme already satisfies this at `41,41,41`.
- `.ColorScheme-ButtonHover` reads `[Colors:Button] DecorationHover`. `button.svg` says `#9e9e9e`
  and the scheme says `158,158,158`; both must be emitted from one token so they cannot drift.
- `Colors:Selection` comes from the accent, not from the tint palette. It is the one role the two
  colour axes share, and the reason they must be separate parameters rather than one lookup.
- The colours embedded in each SVG's `current-color-scheme` block are fallbacks — what an editor
  shows. They should stay accurate for the neutral tint and the default accent, but do not drive
  rendering. `TestShippedStyleFollowsTheColorScheme` guards the deferral itself.

Palettes are written as explicit values per tint, not derived by a hue or chroma transform. A
computed tint behaves badly at the lightness of `#141414`, and the near-blacks want hand-tuning.
Five surface roles across three tints is fifteen numbers, plus one per accent.

## Accent

Accent is not part of the colour scheme format. None of the schemes shipped with Plasma carries an
`AccentColor` key; it lives in `kdeglobals [General] AccentColor`, which the look-and-feel
`defaults` file already writes, alongside `accentColorFromWallpaper=false` so KDE cannot override
the choice from the desktop picture.

For applications that is the whole mechanism — one line, and KDE tints selection, focus rings and
checkboxes from it at runtime.

The Plasma shell is the open part. Because the theme ships its own `colors` file, the shell reads
that file directly rather than resolving through `kdeglobals`, so the accent may not reach panel
artwork on its own. This decides the file count and nothing else:

| If the shell follows `kdeglobals` | If it does not |
| --- | --- |
| accent lives only in `defaults` | accent is also baked into the desktoptheme `colors` |
| tint and accent stay independent: 3 + N files | the two become a product: 3 x N files |

Either way every file involved is a few kilobytes of ini and every one is generated. Hand-maintained
input stays a single token file, and no artwork is produced in either case. Design the generator to
take tint and accent as separate parameters and the outcome is a loop, not a rewrite.

### What the accent should reach

The theme's greyscale is deliberate: `Colors:Selection` is `143,143,143`, and the active task
underline draws from `.ColorScheme-Highlight` rather than from a colour. An accent that only
reaches application focus rings would be close to invisible in a desktop this neutral.

So the accent should drive `Colors:Selection` in both `colors` files as well as `kdeglobals`. That
knowingly reverses the original decision for every non-neutral accent, which is the point of
offering the axis.

Today's two halves disagree: applications get `AccentColor=174,142,108` while the shell's highlight
is `143,143,143` grey. An accent token therefore carries both a `highlight` and a `kde` value, and
the shipped `sand` sets them to today's mismatched pair so the generator can reproduce the current
theme exactly.

That split is a transitional measure, not the design. Adopting the axis properly means collapsing
the two to one colour per accent, which turns the active task underline from grey to tan under
`sand`. Doing it now would have changed the shipped look in the middle of a phase whose whole
point was changing nothing, so it belongs with the other accent work.

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

The toggles are per-surface:

| Toggle | File | Covers |
| --- | --- | --- |
| Panel | `widgets/panel-background.svg` | the panel strip |
| Popups & menus | `dialogs/background.svg` | the application launcher **and** system tray popups |
| Applets | `widgets/background.svg` | plasmoid content areas |

There are three, not the four the `opaque/` tree would suggest. `widgets/tooltip.svg` is opaque in
the base artwork — `opacity.tooltip` is zero, so the root file and its `opaque/` copy are byte for
byte the same — and a switch that cannot change anything is worse than no switch. A fourth toggle
becomes real the moment tooltips are given an opacity, and not before.

The launcher and the tray popups cannot be separated. Both are `PlasmaCore.Dialog` instances and
Plasma ships exactly one `dialogs/background` for all of them; the surfaces are not distinguishable
at the artwork layer by any theme. The option is worded "Popups & menus" so the grouping is honest
in the UI.

`widgets/tasks.svg` remains excluded, for the reason given in the README: its `0.3`/`0.4` values
are white highlights drawn on the panel, not backgrounds.

Overlay granularity changes from directory to file. A whole-directory overlay would need one
directory per combination — eight for three independent toggles. Instead each toggle names the
single file it replaces, drawn from the same `opaque/` tree. `opaque/` and `solid/` continue to be
installed wholesale regardless of any toggle, because Plasma falls back to those prefixes itself
when compositing is off.

An overlay's `from` is relative to the **asset directory**, not to the component's source. The
original transparency option read its overlay out of the tree it had just installed, which was neat
while every overlay lived inside one component. It does not survive a tint, whose two `colors`
files belong to two different components, so overlays now name an asset-relative directory and the
two cases work the same way.

## Tokens

`spec/tokens.json` is the only file edited to add or adjust a variant.

```json
{
  "palettes": {
    "neutral": { "base":"#292929", "elevated":"#2f2f2f", "view":"#141414",
                 "text":"#e8e4dd", "border":"#383838" },
    "slate":   { "base":"#272a2e", "…": "…" },
    "rose":    { "base":"#2c2727", "…": "…" }
  },
  "accents": {
    "sand": { "highlight":"#8f8f8f", "kde":"#ae8e6c" },
    "…":    "…"
  },
  "surfaceShape": {
    "rounded": { "panel":8, "popup":8, "button":6 },
    "square":  { "panel":0, "popup":0, "button":0 }
  },
  "decorationShape": {
    "square":  { "titlebar":0 },
    "rounded": { "titlebar":8 }
  },
  "buttonStyles": {
    "windows": { "plateRadius":3, "closePlate":"#e0655f", "width":28, "height":26,
                 "closeHover":"0.75", "plainHover":"0.18", "rest":"0.85", "…":"…" }
  },
  "opacity": { "panel":0.85, "popup":0.85, "tooltip":0 }
}
```

## Layout

Artwork becomes a build product. The SVGs are already mechanically regular — `button.svg` is nine
tiles across four states, `tasks.svg` is five edges by six states by nine tiles at computed offsets
— and nobody should be hand-editing a 210x150 coordinate grid, let alone two shape variants of one.

```
spec/
  tokens.json               the only hand-edited variant input
internal/gen/
  main.go                   which files exist at a point in the variant space
  frame.go                  panel and popup frames: nine tiles, cubic corners
  control.go                buttons, inputs and list items: stacked states, arc corners
  aurorae.go                window decoration, titlebar buttons, the two ini files
  colors.go                 KColorScheme ini from a palette and an accent
assets/                     generated; committed
  variants/
    colors/<tint>[-<accent>]/   the two colors files
    surfaces-square/            the five surface SVGs
    deco-rounded/               decoration.svg
    buttons-mac/                the four button SVGs
    rc/<deco>-<buttons>/        VanillaBoxDarkrc
    defaults/<accent>-<buttons>/    look-and-feel defaults
```

The emitters are Go rather than text templates. Regeneration has to be byte-for-byte against
artwork that was originally written by hand, and the existing files are not uniformly formatted —
the panel puts its nine tiles on one line where the dialog frames use one per line. Reproducing
that from a template means encoding whitespace in the template, which is worse to read than the
code that decides it.

Twenty-four files are generated and nine are not. Artwork no axis touches — `line.svg`,
`plasmoidheading.svg`, `tabbar.svg`, `tasks.svg`, the two `metadata.json`, `metadata.desktop` and
`theme.json` — stays hand-maintained, because generating a file that never varies costs a builder
and buys nothing.

`scrollbar.svg` is the deliberate exception. It is an Inkscape document: 987 lines and 32KB of
editor metadata to express 22 rectangles, unlike every other file in the theme. Its whole
contribution to the shape axis is one `rx` on the slider, so it stays hand-maintained and a square
variant would override it with a single-file overlay rather than a generator. Rewriting it as
clean SVG is worthwhile cleanup, but it is not variant work.

### Three corner idioms

The artwork does not round corners one way, and the differences are in the path data rather than
in appearance, so they cannot be normalised without changing the committed bytes:

| Where | Construction |
| --- | --- |
| Popup and dialog frames | cubic, with a straight run between arc and tile edge (tile 10 > radius 8) |
| Panel frame | cubic, closing directly off the arc (tile 8 == radius 8) |
| Buttons, inputs, list items | `A` arc commands, radius 6 |

The cubic control points sit at `radius * 0.4478`. That constant is pinned by the artwork's own
rounding rather than chosen: it is the only three-decimal value that yields both the `3.582` the
frames use at radius 8 and the `3.135` the window decoration uses at radius 7 for its inset
border. The usual `1 - 0.5523` gives `3.134` and fails to reproduce the decoration.

Generated assets are committed. The README promises `assets/` ships beside the binary, the tests
install the real shipped artwork rather than a fixture, and a contributor should be able to run the
installer without a generate step. `.gitignore` therefore stays minimal and deliberately does not
list `assets/`.

## Manifest

`Option` becomes typed. A `select` chooses among named values; a `toggle` stays boolean.

```json
"options": [
  { "id":"tint", "name":"Colour", "kind":"select", "defaultValue":"neutral",
    "values":[ { "id":"neutral", "name":"Neutral" },
               { "id":"slate",   "name":"Slate",
                 "overlay":{ "from":"variants/colors/slate" } } ] },

  { "id":"transparency-panel", "name":"Translucent panel", "kind":"toggle", "default":true,
    "overlayWhenOff": { "from":"plasma/desktoptheme/vanilla-box-dark/opaque",
                        "files":["widgets/panel-background.svg"] } }
]
```

An omitted `kind` means a toggle, so an option written before selects existed still loads. A select
value with no overlay is the one that matches the artwork as generated — `neutral` copies nothing.

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
  { "source":"variants/defaults/{accent}-{buttonStyle}/defaults",
    "target":"plasma/look-and-feel/org.vanillabox.dark/contents/defaults" },
  { "source":"variants/colors/{tint}/colors",
    "target":"plasma/desktoptheme/vanilla-box-dark/colors" }
]
```

Four rc files, and two defaults files per accent. The `colors` path grows a second key —
`{tint}-{accent}` — only if the shell turns out not to follow `kdeglobals`; see
[Accent](#accent). All are tiny ini, and all are generated.

The `defaults` file is the one place several axes meet, because it is a single KDE-defined file
that happens to carry an accent, a colour scheme name, a Plasma theme name and a button order.
Generating it per combination is preferred over teaching the installer to merge ini fragments,
which would break the guarantee that an option only ever chooses bytes.

## What gets installed

Unchanged: one colour scheme, one Plasma style, one Aurorae theme, one look-and-feel package.
Variants never multiply what lands in `~/.local/share`; they only decide which bytes are copied.
Backups continue to work as described in the README.

## Open questions

- **Does Aurorae substitute colours at runtime?** The five Aurorae SVGs have no
  `current-color-scheme` block, and no system Aurorae theme is installed to compare against. If it
  does not, tint multiplies the *generated* titlebar files by three; hand-maintained files stay
  flat either way, so the risk is contained. Worth testing early regardless.
- **Does the Plasma shell resolve the accent through `kdeglobals`,** or only through the theme's own
  `colors` file? This decides whether tint and accent stay independent or become a product. See
  [Accent](#accent). Testable in the same session as the question above: set `AccentColor`, restart
  the shell, watch the active task underline.
- **The KWin button letter codes**, as above.
