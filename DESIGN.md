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

Treating both as one problem multiplies: 6 palettes x 2 surface shapes x 2 decoration shapes x 2
button styles is 24 combinations of 25 artwork files. Separating them makes the cost additive
instead, because **each axis owns a disjoint set of files**.

## The axes

| Axis | Default | Mechanism | Files it owns |
| --- | --- | --- | --- |
| Palette | `neutral` | runtime, plus one SVG | the two `colors` files, the look-and-feel `defaults`, `decoration.svg` |
| Sidebar | `window` | runtime, as a product with the palette | the two `colors` files |
| Container shape | `rounded` | baked | the four frames across three prefixes |
| Element shape | `rounded` | baked | `button`, `lineedit`, `viewitem` |
| Titlebar shape | `rounded` | baked | `aurorae/decoration.svg`, as a product with the palette |
| Button style | `mac` | baked | the four button SVGs and `VanillaBoxDarkrc` |
| Transparency (x4) | all on | per-file overlay | one file each, from `opaque/` |
| Icons | `off` | whole component, plus the `defaults` as a product | `icons/VanillaBoxIconsDark/`, `[kdeglobals][Icons]` |

Rounded is the default on all three shape axes, and each lists `rounded` before `square` so the
shipped answer is the one the cursor starts on. The theme shipped square containers and titlebars
first, on the reasoning that straight edges where the desktop ends read as deliberate; softened
corners throughout turned out to read as the more finished default, and a square desktop is still
three keystrokes away.

They stay separate axes because the questions are genuinely independent — rounded panels under a
square titlebar is a combination someone will want, and so is a square popup around rounded
buttons — and because only the titlebar carries the Aurorae limitation below. Sharing a default is
not the same as being one question.

Containers and elements were one axis at first, on the reasoning that a corner radius is a corner
radius. They are not: the surfaces a thing sits in and the things sitting in it are separately
convincing, and holding them together made the one combination people actually reach for —
rounded panels, square controls — unreachable. Splitting them cost nothing structurally, because
the two own disjoint sets of files: containers own the backgrounds, elements own the widget
artwork. That is what lets them be two overlays laid down in either order rather than a product of
four combinations, which is what a shared file would have forced.

Both values of every shape axis carry an overlay, including the default ones. A value
that copies nothing would be correct only while the base artwork happens to be what it describes,
and the base has changed more than once — surfaces from rounded to square, buttons from symbols to
traffic lights and back again. Making every value carry its own overlay keeps the option
independent of that.

A palette is a surface set and the accent that goes with it, chosen together. Surfaces and
highlights were two independent axes at first, on the reasoning that they are two questions. They
are — but a curated pair is a defensible answer to both, and eighteen combinations is a great deal
of menu for something most people set once and never revisit. Five named variants say more about
what the theme is for than eighteen coordinates do.

The cost is real: rose surfaces with a steel accent is no longer reachable. If that turns out to
matter the axes split again — the generator already writes a product elsewhere and would do it here
without ceremony.

Accents match their surfaces in temperature, so a variant reads as one decision rather than two.
`neutral` is the exception and takes the grey surfaces, which is what makes it the quiet one rather
than a colour with the volume turned down. Surfaces are named separately from palettes and
referenced by name — a set can then be shared, and renaming a variant does not mean renaming the
colours it points at.

A palette moves surfaces and its accent. Text, inactive text and the colour that sits on the
highlight are held still across all five, because warm text on a blue surface reads as a mistake
rather than as a variant.

`onHighlight` is the one that does not survive the rule, and `forest` is why. An accent is the
selection background — the only colour a palette moves that text sits *on* rather than beside — and
at `#4a6d41` the shared dark on-highlight colour reads at 2.8:1, which is not a foreground. The
light `text` colour reads at 4.7:1 on the same green. The crossover is around `#5c8452`: above it
the dark colour wins, below it the light one does.

So a palette may override `onHighlight`, and only `forest` does. The alternative was moving the
foregrounds for everyone, which is the question holding them still exists to avoid — and picking a
green by what the selection text needed rather than by what the palette is called.
`TestForestInvertsItsSelectionText` pins both halves: forest takes the light colour, and the other
four are checked for still taking the dark one, because an override that leaked would look like a
theme-wide change nobody asked for.

The accent is also `ForegroundLink`, drawn *on* the background rather than under text, and that one
has no override to hide behind. Forest links sit at 2.5:1 against its surfaces where the other four
palettes are near 4.6:1. It is the price of the green: the link colour would have to stop being the
accent to fix it, which would cost the theme the thing that makes an accent read as one decision.

That also keeps a palette almost free. The only artwork it repaints is `decoration.svg`, which
paints the titlebar directly instead of deferring to the scheme; everything else either resolves
its colour at paint time or carries the palette as nothing more than an editor fallback.

Square decorations also retire a compromise. The comment in `aurorae/themes/VanillaBoxDark/
decoration.svg` explains that Aurorae cannot round the bottom corners of a window — rounding needs
a bottom border to draw the curve into, and anything narrower than the radius lets the client's
square corner show through. That limitation now applies only to the non-default rounded variant.

One file is claimed by two axes: `decoration.svg`, which the palette paints and the titlebar shape
gives corners to. It is written as a product, `variants/decoration/<palette>-<shape>/`. See
[Resolved files](#resolved-files).

`VanillaBoxDarkrc` turned out to belong to one axis after all. Its `[Layout]` metrics are the
button sizes, and the titlebar height does not change with the corner radius — so it travels with
the button style as part of that overlay rather than needing a combination of its own. Shipping the
metrics beside the artwork they describe also makes it impossible to swap one without the other.

## Colour

### Prerequisite — done

The runtime mechanism did not work on the original artwork. The backgrounds carried the class but
hardcoded the fill, so `currentColor` was never consulted and a `colors` file would have been
silently ignored for exactly the surfaces a palette needs to reach:

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

- `Window`, `Header` and `Complementary` backgrounds move together within a surface set. Plasma
  resolves a surface against one of the three depending on the widget's colour set; keeping them
  means never having to work out which. The current scheme already satisfies this at `41,41,41`.
  The sidebar option is the one sanctioned exception, and it breaks the rule in the narrowest way
  it can — see below.
- `.ColorScheme-ButtonHover` reads `[Colors:Button] DecorationHover`. `button.svg` says `#9e9e9e`
  and the scheme says `158,158,158`; both must be emitted from one token so they cannot drift.
- `Colors:Selection` comes from the palette's accent, not from its surfaces. It is the one role
  that reads from the other half of the pair.
- `Colors:Tooltip` takes `view` rather than a chrome surface. A tooltip is the one popup that
  appears over arbitrary content, so it reads as a dark card rather than as one more shade of the
  window it covers — the same reason it is the only container that carries a border. Its
  `DecorationFocus` follows its background, as it does in every set that does not want a visible
  focus ring; only `Complementary` and `View` deliberately differ.
- The colours embedded in each SVG's `current-color-scheme` block are fallbacks — what an editor
  shows. They should stay accurate for the default palette, but do not drive rendering.
  `TestShippedStyleFollowsTheColorScheme` guards the deferral itself.

### The sidebar option

KColorScheme has no sidebar role. A places panel — Dolphin's, Kate's, anything in a `QDockWidget` —
paints with the window background, which is why it matches the toolbar rather than the list beside
it. So "make the sidebar the view colour" can only be spelled as "move `[Colors:Window]
BackgroundNormal` to the view colour", and that reaches every window background in every Qt app,
not just panels. Dialogs, settings pages and message boxes go dark with it. This is why it is an
option and not the default.

`Header` deliberately does not follow. It is the one place the move-together rule above is broken,
and breaking it is the point: a sidebar merged into the file list still wants a toolbar above it
that is not, and `Header` is the role that draws it. `Complementary` does follow `Window`, so of
the two roles a widget might resolve a plain window surface against, neither can disagree with the
other.

The choice is a product with the palette rather than an overlay, because both `colors` files carry
the window background and there is no file to swap that does not also carry the tint —
`variants/colors/{palette}-{sidebar}/`, ten directories for five palettes.
`TestSidebarMovesOnlyTheWindowBackground` pins which sections are allowed to move.

Surfaces are written as explicit values per set, not derived by a hue or chroma transform. A
computed shift behaves badly at the lightness of `#141414`, and the near-blacks want hand-tuning.
Six surface roles across five sets is thirty numbers, plus one accent per palette.

## Accent

Accent is not part of the colour scheme format. None of the schemes shipped with Plasma carries an
`AccentColor` key; it lives in `kdeglobals [General] AccentColor`, which the look-and-feel
`defaults` file writes, alongside `accentColorFromWallpaper=false` so KDE cannot override the
choice from the desktop picture.

For applications that is the whole mechanism — one line, and KDE tints selection, focus rings and
checkboxes from it at runtime.

The Plasma shell is the part that was never settled. Because the theme ships its own `colors` file,
the shell reads that file directly rather than resolving through `kdeglobals`, so the accent may
not reach panel artwork on its own. Rather than answer the question, the accent is written into
both: if the shell does resolve `kdeglobals` the second copy changes nothing, and if it does not,
the second copy is the only thing that works.

### What the accent reaches

`Colors:Selection` in both `colors` files, and `AccentColor` in `kdeglobals`. The theme's greyscale
was once deliberate — selection was `143,143,143` and the active task underline drew from
`.ColorScheme-Highlight` rather than from a colour — and an accent that reached only application
focus rings would be close to invisible in a desktop this neutral.

Under the default `neutral` palette that makes the active task underline tan rather than grey. An
`ash` palette once held the grey `143,143,143` selection as a way of keeping the original
no-colour-anywhere look reachable; it was dropped, because a variant whose only content is the
absence of the accent is a menu entry explaining a decision rather than offering one.

## Buttons

The button style axis is not only artwork.

**Order is a KWin setting.** Mac-style buttons sit on the left, which lives in `kwinrc
[org.kde.kdecoration2] ButtonsOnLeft` / `ButtonsOnRight` — that is, in the look-and-feel `defaults`
file, which therefore becomes generated. The shape of the change is `ButtonsOnLeft=XIA` with an
empty `ButtonsOnRight`, against a default of `ButtonsOnRight=IAX`. **The exact letter codes are
unverified** — confirm them once against System Settings -> Window Decorations before relying on
them.

### Metrics as shipped

These were tuned by eye against a real titlebar rather than derived from anything, so they are
worth writing down. `TestTitlebarButtonMetrics` pins them.

| | Symbols | Traffic lights |
| --- | --- | --- |
| Button box | 28 x 28 | 22 x 22 |
| `glyphSize` / `circleRadius` | 13 | 6 |
| **Rendered mark** | 13 x 13 px | 11 px across |
| `nudgeTop` | -1 | -1 |
| `ButtonMarginTop` | 0 | 3 |
| `ButtonWidthMenu` | 20 | 16 |
| Plate | square, no radius | n/a |

Both boxes are square on purpose. Aurorae scales the 24x24 tile to `ButtonWidth x ButtonHeight`, so
a box of 28x26 stretched every symbol 7.7% wider than tall — a circle in a glyph stopped being a
circle. Keeping the box square makes the scale uniform.

**The two size tokens are measured differently, which is a trap.** `glyphSize` is in rendered
pixels: the generator converts it back into tile units so the number in the tokens is the number
you measure on screen. `circleRadius` is still in tile units, so it scales with the button box —
growing the box from 20 to 22 took the circles from 10px to 11px without the token changing.
Worth unifying if the circles are ever tuned as carefully as the glyphs were.

**The application icon is the one button sized on its own.** Aurorae gives every button the same
`ButtonHeight` and only the width is per-type, through `ButtonWidthMenu` — so the icon can be made
smaller sideways and the leftover height is what gives it room above and below. Raising
`ButtonMarginTop` instead would have pushed the close, minimise and maximise buttons down with it.

`TitleEdgeLeft` is 8 against a `TitleEdgeRight` of 6 for the same reason: the icon sits in the
corner of the window and wants more of a margin there than the buttons at the other end do.

**`ButtonMarginTop` is derived**, as `(TitleHeight - ButtonHeight) / 2 + nudgeTop`. Aurorae places a
button that far from the top of the titlebar and leaves the remaining slack below it, so a zero
margin sits every button high. `nudgeTop` is the optical correction on top of that arithmetic, and
it is relative: changing a button height moves the centre, so the nudge has to be revisited to hold
the same edge alignment. The symbols' -1 is what makes their hover plate sit flush against the
titlebar's top border.

The circles stay centred in their own tile so the hit area still matches what is drawn; it is the
button box that moves.

### Maximised windows lay out from different keys

Aurorae positions a maximised titlebar from an entirely separate set of `*Maximized` keys, and
every one of them defaults to zero rather than to its ordinary counterpart. `AuroraeButtonGroup.qml`
computes the button offset as

```qml
maximised ? titleEdgeTopMaximized + buttonMarginTopMaximized
          : titleEdgeTop + padding.top + buttonMarginTop
```

Leaving them unset moved every button the moment a window was maximised: 1px up for the symbols,
4px up for the traffic lights, and 7px outward for both. Nothing in the theme was wrong — the
second branch simply read values nobody had written.

Each maximised key now mirrors its ordinary counterpart, and deliberately does **not** add the
padding the other branch adds. Padding is the frame outside the window: the decoration's origin
sits that far out from the window edge when a window is restored, and exactly on it when maximised.
The ordinary branch adds padding to reach the place the maximised branch already starts from, so
adding it to both puts every button a pixel out — which is what a first attempt at this did.

The edges also feed the titlebar height. `borderTopMaximized` is
`titleEdgeTopMaximized + TitleHeight + titleEdgeBottomMaximized`, so padded edges additionally made
a maximised titlebar 2px taller than a restored one — a second, larger error hiding behind the
first.

These values are **confirmed on a real desktop**, not only reasoned about: the buttons hold their
position across restore and maximise. `TestTitlebarButtonMetrics` asserts them alongside the
ordinary metrics, because this is precisely the kind of thing that is invisible until someone
maximises a window.

The working set, for reference:

```ini
TitleEdgeTop=0                 TitleEdgeTopMaximized=0
TitleEdgeBottom=0              TitleEdgeBottomMaximized=0
TitleEdgeLeft=6                TitleEdgeLeftMaximized=6
TitleEdgeRight=6               TitleEdgeRightMaximized=6
ButtonMarginTop=3              ButtonMarginTopMaximized=3     ; traffic lights; 0 for symbols
PaddingTop=1  PaddingBottom=1  PaddingLeft=1  PaddingRight=1
```

### Applying a decoration change

KWin reads an Aurorae theme **once**, and caches it for the life of the session. Reinstalling puts
new files on disk and changes nothing on screen.

What was confirmed to work is a full restart. Switching to another window decoration in System
Settings and back is the obvious lighter alternative and may be enough, but it has not been shown to
be — a change that appeared not to work here turned out to have been correct on disk the whole time,
and the only thing that had failed was getting KWin to read it.

That is worth remembering before concluding that a decoration edit did not work: check the
installed `VanillaBoxDarkrc` first, and only then doubt the values.

**The interaction model differs.** The Windows-style buttons are monochrome glyphs at rest that
gain a square coloured plate on hover. The Mac style has no glyphs at all: three grey circles at rest,
which take a muted traffic-light colour on hover. This is a per-style treatment, not a shared
pattern with different values.

The traffic-light colours are muted rather than the authentic `#ff5f57`/`#febc2e`/`#28c840`. In a
desktop whose selection is grey and whose accents top out around `#b8776a`, authentic values would
be the most saturated pixels on screen by a wide margin.

**Hover is per-button, and cannot be otherwise.** On macOS, hovering any of the three lights up all
three. Aurorae renders each button from its own SVG with no knowledge of its neighbours, so here
hovering close colours only close. This is a limitation of the format, not a choice.

**The buttons stay on the right,** whichever shape is chosen. Left is the macOS convention and the
only place the traffic-light shape normally appears, so choosing traffic lights here gives the
shape without the placement. It is deliberate:
button order is a `kwinrc` setting rather than an Aurorae file, so writing it would overwrite a
preference the user may have set for reasons of their own — and they can move the buttons in System
Settings at any time without reinstalling. Left is what a macOS switcher, an RTL locale (where KWin
mirrors the layout regardless) or an elementary-style desktop would expect.

Leaving the order alone also means the KWin button letter codes never have to be verified, which is
why that open question is now closed rather than answered.

## Icons

The icon theme is [Phosphor](https://phosphoricons.com)'s **light** weight, mapped onto KDE icon
names by `spec/icons.json`. One weight throughout: mixing them is what makes an icon set look
assembled rather than designed.

The set shipped as `fill` first. Light is the outline of the same drawings, which reads quieter
beside Breeze's own outline icons but has less to hold onto at 14 pixels — a hairline that a panel
at fractional scaling can thin further. Changing weight is one key in `spec/icons.json` and a
re-vendor, so this is a decision that stays cheap to revisit rather than one to get right once.

The weights are not the same drawing at different thicknesses, and a few compositions change
outright: `power` is a solid disc with a knocked-out symbol in `fill` and a bare arc in `light`, so
it comes out a pixel and a half under the icons it sits beside. The glyph size below is a grid
reference, not a promise about any individual icon.

**Inheritance is what makes the set possible at all.** Breeze ships nineteen thousand SVGs. Any
attempt to replace that is a project, not a component. `index.theme` names `Inherits=breeze-dark`,
so the names mapped here are the only ones this theme owes; everything else is answered by Breeze,
including names KDE has not invented yet. The scope that follows is "icons the shell itself draws"
— tray, launcher, applet chrome, kickoff, session actions — because those are the ones seen
constantly and against our own panel.

The boundary worth stating: application entries in the menu draw from each application's own
`.desktop` file. Firefox stays Firefox. This theme owns the launcher button, the category rail and
the session actions, not the app list.

### Finding what is missing

Guessing at icon names does not work; a missing one is invisible, because inheritance quietly
supplies a Breeze icon in its place and the result looks intentional. So the list is derived rather
than imagined:

```sh
# the icon names the shell actually references
strings /usr/bin/plasmashell /usr/lib64/qt6/qml/org/kde/plasma/private/*/*.so |
  grep -oE '[a-z][a-z0-9]+(-[a-z0-9]+)+' | sort -u > referenced
# intersect with names Breeze actually ships, then subtract spec/icons.json
```

Restricting to hyphenated names is what makes it usable: binaries are full of single words that
collide with icon names — `class`, `enum`, `sqrt`, `formula` are all real Breeze icons and none of
them is being asked for by the panel. Hyphenated matches are almost always genuine.

That found the applet header's `configure` and `window-pin`, kickoff's `favorites` and
`help-contents`, and about fifty more. What is left unmapped after it is deliberate: app-specific
names the shell merely mentions (`kdeconnect-tray`, `games-highscores`, `irc-voice`,
`view-barcode-qr`) and the mobile broadband set.

### The transform is a wrapper

A Phosphor fill asset is one path in a 256-unit box already set to `fill="currentColor"`. A KDE
monochrome icon is the same thing wrapped in a `current-color-scheme` stylesheet — the idiom the
Plasma style's artwork already uses. So an icon is generated, not drawn: the source's paths lifted
out with an XML parser and wrapped by the renderer.

### The icons are painted, not recoloured

This is the one place the theme opts out of the mechanism everything else depends on.

An icon carrying `ColorScheme-Text` is repainted at load time in the colour scheme's text colour,
`#e8e4dd`. `color` in `spec/icons.json` is `#ebebeb`, a colour the scheme does not contain — and
there is no way to have both. The stylesheet *is* the request to be recoloured, so an icon with a
colour of its own goes without one, and the generator emits `<g fill="#ebebeb">` with no stylesheet
at all. A test checks both directions: an icon that carried the colour and the class would have the
colour silently replaced and look like it had never been set.

What that costs:

- **Selected rows.** KDE paints a deferring icon in the inverted foreground when it lands on a
  selection, the way the text beside it inverts. A painted icon stays `#ebebeb` there. In the
  kickoff category rail — which is exactly where this theme's icons are, and which highlights on
  hover — a light icon then sits on the accent while the label beside it goes dark.
- **The palette.** Nothing follows a palette change any more; the colour is in the artwork.

What it does not cost is the variant axis. The icons were already the same bytes for all five
palettes, for the opposite reason: they used to resolve their colour late, and now they have none
to resolve.

Deleting `color` puts the icons back on the colour scheme, and is the whole of the way back.

### Glyph size

`glyph` in `spec/icons.json` is how many pixels an icon filling Phosphor's grid measures in the
22 pixel box the tray and menus ask for. It is held in pixels rather than as a scale because the
pixels are the decision; the generator derives the transform, and a test checks the round trip so
nobody has to rasterise an icon to find out what they got.

Two reference points sit either side of the shipped value. Phosphor unscaled measures **17.9px**:
its artwork is inset by 24 units of 256, so it draws to 81% of its box where Breeze draws to 73%.
Breeze itself measures **16.0px**, and Breeze is where every unmapped icon still comes from, so it
is the size ours are seen next to.

The theme ships **16px**, a 3 pixel margin, which is Breeze's own metric — so a mapped icon and the
inherited one beside it are drawn to the same size. It arrived there by being asked for twice
rather than by aiming at it, which is worth recording: 14 and 15 both read as small on a panel
mixing the two sets, and the size Breeze picked turned out to be the size that looks right next to
Breeze.

In the light weight the square icons spread around it: the grid constant is 208 units and the
weight's own drawings sit between 172 and 212, so a magnifying glass measures 15.7px, a lock 16.3
and the launcher's four squares 13.2. That spread is the artwork's, not the scale's.

The scale is uniform rather than per-icon, and applied as a transform about the centre of the box
rather than by rewriting path data — what is committed stays recognisably the vendored source with
a wrapper around it. Normalising each icon to its own bounding box would make every glyph the same
size, which is not what a designed set does: Phosphor's relative sizes — a magnifying glass smaller
than a monitor — are part of why it reads as one set. So 14px is what a full-grid icon measures,
and the ones drawn smaller stay proportionally smaller.

The stylesheet is written directly under `<svg>` rather than inside a `<defs>` as the frames do.
Both work for the Plasma style; the icon loader is a different consumer, and the format Breeze's
own icons use is the one worth copying rather than the one that happens to match the frames.

### One geometry, one directory

Breeze keeps a directory per pixel size because its artwork is drawn again at each one. Phosphor is
a single geometry, so fixed sizes would be the same path data under four different `Size=`
headings. The theme writes one `scalable` directory per context instead, which is what the icon
theme spec is for. If a lookup ever misses, adding fixed sizes is a loop in the generator.

Every name is also written as its `-symbolic` twin. Plasma 6 asks for the symbolic name in most
shell contexts, and a monochrome set has no second drawing to offer anyway — so the alternative to
duplicating the file is falling through to Breeze for half the tray.

### Families

Breeze spells charge level and signal strength as a hundred and two battery filenames and a hundred
and sixty-three network ones. They collapse to about a dozen pictures, and writing the collapse out
by hand is how a mapping file goes stale. A family is a name template with a level and any number
of slots; the generator multiplies it out. A slot alternative naming an icon overrides the level's
— a charging battery is the same picture at every charge — and one naming null keeps it, which is
how a power profile changes the filename without changing the artwork.

Mobile broadband is deliberately unmapped, even though the scan shows the shell referencing it:
another hundred and sixty names, drawn only by a machine with a modem in it, and Breeze still
answers them. It is the one place where "referenced by the shell" and "worth mapping" come apart.

### The sources are vendored

`spec/phosphor/` holds the artwork the mapping names, committed, at the commit `icons.json` pins.
Generation has to work offline and produce the same bytes every time, and a build that reaches the
network does neither. `go run ./internal/gen -fetch` is the one thing in the repository that opens
a socket, and it is run by hand when a mapping is added. A test fails if a mapping names artwork
nobody vendored, because that failure would otherwise wait for the next person to run the
generator.

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
while every overlay lived inside one component. It does not survive a palette, whose two `colors`
files belong to two different components, so overlays now name an asset-relative directory and the
two cases work the same way.

`from` also takes `{option-id}` placeholders, and the transparency switches need them:
`variants/containers/{containers}/opaque`. Two overlays land on the same file here — the square
variant replaces every container, and then a switch replaces one of them again with its opaque
copy. Drawing that copy from a fixed path would put rounded corners back on exactly the surfaces
the user made opaque, and only on those. Options are applied in the order the manifest declares
them, so the shape select is written before the switches that draw from it.

The switches follow `containers` and not `elements`, which is the split doing its job: only a
background has an opaque copy to fall back to, so the element axis has nothing to offer them.

## Tokens

`spec/tokens.json` is the only file edited to add or adjust a variant.

```json
{
  "theme":      { "name":"Vanilla Box Dark", "version":"0.2.0",
                  "iconsId":"VanillaBoxIconsDark", "…":"…" },
  "foreground": { "text":"#e8e4dd", "textInactive":"#8a8782", "onHighlight":"#1f1f1f" },

  "surfaces": {
    "grey":   { "background":"#292929", "elevated":"#2f2f2f", "view":"#141414", "…":"…" },
    "slate":  { "background":"#272a2f", "…":"…" },
    "forest": { "background":"#252b25", "…":"…" },
    "…":      "…"
  },
  "palettes": {
    "neutral": { "surfaces":"grey",   "accent":"#ae8e6c" },
    "slate":   { "surfaces":"slate",  "accent":"#7d93ad" },
    "…":       "…",
    "forest":  { "surfaces":"forest", "accent":"#4a6d41", "onHighlight":"#e8e4dd" }
  },
  "containerShape": {
    "rounded": { "panel":8, "popup":8 },
    "square":  { "panel":0, "popup":0 }
  },
  "elementShape": {
    "rounded": { "button":6 },
    "square":  { "button":0 }
  },
  "decorationShape": {
    "square":  { "titlebar":0 },
    "rounded": { "titlebar":8 }
  },
  "buttonStyles": {
    "windows": { "plateRadius":0, "closePlate":"#e0655f", "width":28, "height":26,
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
  tokens.json               the hand-edited variant input
  icons.json                KDE icon name -> Phosphor icon, and the families
  phosphor/                 vendored artwork, pinned by icons.json
internal/gen/
  main.go                   which files exist at a point in the variant space
  frame.go                  panel and popup frames: nine tiles, cubic corners
  control.go                buttons, inputs and list items: stacked states, arc corners
  aurorae.go                window decoration, titlebar buttons, the two ini files
  colors.go                 KColorScheme ini from a palette and an accent
  icons.go                  the icon theme and its index
  fetch.go                  -fetch, the one thing here that uses the network
assets/                     generated; committed
  variants/
    colors/<palette>/               the two colors files      10 files
    decoration/<palette>-<shape>/   decoration.svg            10 files
    defaults/<palette>-<icons>/     look-and-feel defaults     10 files
    containers/<shape>/             the four frames x3        24 files
    elements/<shape>/               the three controls         6 files
    buttons/<style>/                titlebar set              10 files
  icons/VanillaBoxIconsDark/
    <context>/scalable/             the icon theme           585 files
```

The emitters are Go rather than text templates. Regeneration has to be byte-for-byte against
artwork that was originally written by hand, and the existing files are not uniformly formatted —
the panel puts its nine tiles on one line where the dialog frames use one per line. Reproducing
that from a template means encoding whitespace in the template, which is worse to read than the
code that decides it.

Artwork no axis touches — `line.svg`, `plasmoidheading.svg`, `tabbar.svg`, `tasks.svg`,
`scrollwidget.svg` — stays hand-maintained, because generating a file that never varies costs a
builder and buys nothing.

The identity files are the exception to that rule, and are generated despite never varying: KDE
wants the same handful of facts in three formats — two `metadata.json` and a `metadata.desktop` —
and the installer wants the version in a fourth. Four hand-maintained copies of one version number
is four chances to disagree, so all four are written from `spec/tokens.json`. `internal/theme/
version.go` is generated Go rather than something read at runtime, so `vanillabox --version` still
answers when the asset directory cannot be found at all.

`assets/theme.json` keeps its own copy, because the manifest stays hand-written. A test asserts the
two agree.

`scrollbar.svg` is the deliberate exception. It is an Inkscape document: 987 lines and 32KB of
editor metadata to express 22 rectangles, unlike every other file in the theme. Its whole
contribution to the shape axis is one `rx` on the slider, so it stays hand-maintained and takes no
part in any variant. Rewriting it as clean SVG is worthwhile cleanup, but it is not variant work.

### Widgets that exist only to paint nothing

A theme that does not ship a widget falls back to the default theme's copy, so an omission is not
neutral — it inherits Breeze. `widgets/scrollwidget.svg` is here for that reason alone: Breeze's
version paints `border-top`, `border-left`, `border-right` and `border-bottom` in `currentColor` at
full opacity, which is a 1px box drawn around every scroll area in a Plasma popup. Nothing outside
that file can switch it off, so the only way not to have it is to ship a `scrollwidget` whose tiles
are all `fill:none`.

Its 1px edges are kept rather than collapsed to zero, so the insets Plasma reads match what Breeze
reported and nothing reflows. The scrollbar itself is untouched.

Thirty-one other widgets still fall back — `frame`, `listitem`, `slider`, `switch` and the rest —
and each is a place Breeze can show through. They are only worth shipping when one of them is
actually seen to be wrong.

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

### The tooltip's outline

`widgets/tooltip.svg` is the only container that carries a border. It is also the only one that
appears over arbitrary content rather than over the desktop or a panel it already contrasts with,
so it is the only one that has to define its own edge.

The background is painted first, over the tile's whole shape, and the outline laid over it as a
ring between that shape and the same shape a pixel in, so the two antialiased curves stay
concentric. The corner builders therefore take an inset, and the idiom is chosen from the frame's
own radius rather than the inset one — an inner path is the same corner drawn a pixel in, and
re-deciding on the reduced radius would hand a frame's inner path to a different template than its
outer one.

A corner's ring is both paths in one `d`, under `fill-rule="evenodd"`, so the inner shape is cut
out rather than drawn. They meet flush along the boundaries a tile shares with its neighbours, and
there the two coincident edges cancel and leave the ring open — the same three sides the edge
strips leave bare, because an outline on a shared seam would draw a line across the middle of the
tooltip.

Three things about it are easy to get wrong:

- **The outline goes over the surface, not under it.** The window decoration stacks these the other
  way round — border first, background inset over it — and this file was first written from that
  template. It does not carry across. The decoration's border is an opaque literal, so what sits
  underneath it never comes up; this one is a tenth of the text colour, and a tenth of something
  needs the surface underneath it to be a tenth *of*. Painted first, the frame's outermost pixel is
  the text colour at `0.098` alpha over whatever is behind the tooltip — ninety percent
  transparent, a gap rather than an edge. Over the surface it resolves to an opaque `#292828`
  against the default palette's `#141414`.

- **It goes through the stylesheet, not a literal colour.** The container artwork is generated once
  per shape, not per palette — `containers/` has a `rounded` and a `square` tree and no tint axis —
  because Plasma resolves `ColorScheme-*` classes at paint time. A literal border colour would bake
  the default palette's edge into a file all five palettes share. The outline is
  `ColorScheme-Text` at `0.1`, so it follows the tint like everything else.
- **The mask copies stay whole.** `mask-*` is the blur region rather than artwork. An outline drawn
  into it would punch a ring out of the blur instead of showing up as a border, so the mask keeps
  the frame's full outer shape.

The border does not change what the transparency toggles cover. `opacity.tooltip` is still zero, so
the root file and its `opaque/` copy remain byte for byte the same and there is still no fourth
toggle to offer.

#### The empty shadow prefix

`widgets/tooltip.svg` also ships a `shadow-` prefix whose nine tiles are all `fill:none`. It exists
for the same reason `widgets/scrollwidget.svg` does — to stop a fallback — and without it the
tooltip draws two borders.

`org.kde.plasma.components.ToolTip` builds its background from two `FrameSvgItem`s over the same
sheet: one with `prefix: "shadow"`, anchored with *negative* margins so it sits outside, and one
plain. KSvg decides a prefix exists by looking for `<prefix>-center`, and clears the prefix when it
finds none — so a sheet with no shadow tiles renders its own frame a second time, inflated by the
frame's 4px margins. Two flat fills stacked that way are invisible; two outlined ones are a pair of
concentric borders 4px apart.

The tiles are bare rects on the painted tiles' own coordinates. An element is fetched by id, so
what it overlaps in the sheet never comes up — the `mask-` copies already work this way. The
`shadow-hint-*-margin` rects repeat the frame's own, so the prefix reports the insets KSvg was
already deriving from the fallback and the tooltip's geometry does not move.

Only the tooltip needs this. `widgets/button.svg` is the other file a `shadow` prefix is asked for
(`ButtonShadow.qml`), and it has no unprefixed tiles at all, so its fallback finds nothing to draw.
Every other container is a flat fill, where a doubled draw has nothing to show. A border is what
makes the fallback visible, and the tooltip is the only container that carries one.

Generated assets are committed. The README promises `assets/` ships beside the binary, the tests
install the real shipped artwork rather than a fixture, and a contributor should be able to run the
installer without a generate step. `.gitignore` therefore stays minimal and deliberately does not
list `assets/`.

## Manifest

`Option` becomes typed. A `select` chooses among named values; a `toggle` stays boolean.

```json
"options": [
  { "id":"palette", "name":"Colour", "kind":"select", "defaultValue":"neutral",
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

`group` names the page an option is asked on and `order` places it within that page, low to high,
with the manifest's own sequence breaking ties. A page is gathered by group name rather than as a
run, because a group can span components — Shape is asked partly by the Plasma style and partly by
the decoration — and without `order` the only way to move one preference within it is to reorder
the components, which moves every other page too.

The order values are listed in is presentational. The cursor opens on the value already selected
rather than on the first line, so `defaultValue` and the listing are free to disagree: Button &
input corners lists Square first and installs Rounded. Tying the cursor to the first row instead
would make every listing order a silent second declaration of the default, and one stray enter on
an unread page would change the answer.

An option still never edits a file. It only chooses which pre-generated bytes get copied, which is
the same guarantee the transparency option makes today.

### There is no component checklist

The four core components are marked `required`, so they install without being asked about and the
checklist screen is gone. The run opens on the preferences.

The icon theme is the exception, and it is the case that shows why the flag was worth keeping past
the screen it was invented for. Icons are a real choice — they replace every icon on the desktop,
which is the largest thing this installer can do to a machine — and they ship turned off, so
accepting every prompt leaves the user's icons alone.

They are not offered as a checklist row, though. A component carries `installedWhen`, naming a
preference and the value it has to hold:

```json
{ "id": "icons", "installedWhen": { "option": "icons", "value": "on" } }
```

The question is then asked once, in its own preferences page, and two things follow the one answer:
the files, and the `[kdeglobals][Icons]` line in the look-and-feel `defaults`. Those had to move
together — switching to an icon theme that was never installed points kdeglobals at a directory
that is not there. KDE recovers by falling back, but silently writing a broken setting is not the
same as leaving the user's icons alone, which is what turning the option down asked for. So the
`defaults` file became a product with the icon choice, `defaults/<palette>-<on|off>/`, and the
answer reaches both halves or neither.

Selection is recomputed whenever an answer changes rather than decided once when the model is
built, since the preference that decides it can be revisited at any point before the review.

What this gives up is real: the Global Theme package cannot be declined, and its `defaults` also
sets a cursor theme and a splash screen. Those apply only once the Global Theme is picked in System
Settings, but they are no longer avoidable at install time.

A component that is not installed is named on the review screen, and the two reasons are told
apart: `unavailable, will be skipped` is a gap in the asset tree, `not selected` is the user's
answer. That was the greyed-out checklist row's job, and the review is now the only place left to
say it.

### One question at a time

Preferences are asked over pages rather than on one screen: surface colour, shape, transparency,
icons.
Each option declares its `group` in the manifest, and the pages are the distinct groups in the
order they first become visible.

That order is worth knowing: it follows `visibleOptions`, which walks components, not the order
options sit inside one of them. The palette is declared on the Plasma style but reached first
through the colour scheme's resolved path, so Colour leads. A page whose group appears in two
components — Shape, which spans the Plasma style and the decoration — still renders as one page,
because grouping is by name rather than by run.

A page asking a single choice drops that choice's header row, since the page heading already names
it, and moves its description into the subtitle beside the step counter. Otherwise the screen says
"Colour" twice.

### Choices are lists, not controls

A choice renders as a header with one row per value, `(•)` on the chosen one. The alternative — a
single row cycled with arrow keys — hides every option the user has not already found.

The cost is height: seven preferences with their values is more than a 24-line terminal holds, so
the screen scrolls. Blank lines between groups are rows in the model rather than padding inside
another row, which keeps the scroll window counting screen lines and list rows identically.

The window never begins part-way through a group or ends on a group's header. Unlabelled values and
a header naming nothing are the same mistake, and both are worse than showing one group fewer.

`←`/`→` went with the change. Once values have their own rows, up/down and space do everything, and
a binding that duplicates another is noise in a help line whose whole purpose is to be honest.

### Where a preference is offered

A preference is shown when the current selection actually uses it — either because a selected
component declares it, or because a selected component names it in a resolved path. The palette is
declared once, on the Plasma style, and the colour scheme reads it through
`variants/colors/{palette}/…` — which matters more now that the colour scheme is required and no
longer appears on the checklist at all.

The alternative was repeating every value list on every component that consumes it, which is the
same data in three places and three places for it to drift.

### Resolved files

Two files depend on a combination of axes rather than a single one. Rather than layer overlays and
depend on ordering, they are generated per combination and selected by substituting option ids into
a path:

`source` is relative to the asset directory; `target` is relative to the component's installed
directory, and an empty target means the component's own path — which is what a component that
installs a single file needs.

```json
"resolved": [
  { "source":"variants/colors/{palette}/colors",       "target":"colors" },
  { "source":"variants/defaults/{accent}/defaults",    "target":"contents/defaults" },
  { "source":"variants/decoration/{palette}-{titlebar}/decoration.svg",
    "target":"decoration.svg" }
]
```

Resolved files are written on every install, the default combination included. There is no special
case for "this is the one already in the tree", so the path that runs for `neutral`/`sand` is the
same path that runs for everything else.

The `defaults` file is the one place several axes meet, because it is a single KDE-defined file
that happens to carry an accent, a colour scheme name, a Plasma theme name and a button order.
Generating it per combination is preferred over teaching the installer to merge ini fragments,
which would break the guarantee that an option only ever chooses bytes.

## What gets installed

One colour scheme, one Plasma style, one Aurorae theme, one look-and-feel package, and the icon
theme if it was asked for.
Variants never multiply what lands in `~/.local/share`; they only decide which bytes are copied.
Backups continue to work as described in the README.

## Settled

- **Whether Aurorae substitutes colours at runtime no longer matters.** It was open for a while:
  the five Aurorae SVGs have no `current-color-scheme` block, so if Aurorae did substitute, the
  theme was not taking advantage of it. Nothing now depends on the answer — `decoration.svg` is
  generated per palette with its colours already baked in, and the buttons are painted in
  foregrounds that are held constant across every palette.
- **The maximised layout keys work as described**, verified on a real desktop after a restart.

## Open questions

- **Adding an accent is two edits, not one:** the colour in `spec/tokens.json`, and the value in
  `assets/theme.json` so the installer offers it — now carrying the colour a second time, as the
  swatch the preferences screen draws. The manifest stays hand-written by decision — the README
  promises that adding a component is an edit to it rather than to the code, and generating it
  would buy consistency at the cost of that promise. `TestSwatchesMatchTheAccents` closes the gap
  the copy opens, failing with both values and the instruction to edit them together. If the two
  drift often enough to matter anyway, generating the value lists is the fix.
- **`scrollbar.svg` keeps its rounded slider in the square variant.** Its `rx` is 2px on a 6px
  slider, which is close to invisible, and patching it would mean the generator reading and
  rewriting the one Inkscape document in the tree. Worth doing alongside the rewrite, not before.
