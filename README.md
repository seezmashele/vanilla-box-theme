# Vanilla Box

A terminal installer for the Vanilla Box Dark KDE Plasma theme, built with
[Bubble Tea](https://charm.land/bubbletea).

It copies the theme's files into `~/.local/share` so KDE can find them, and nothing else. It does
not apply the theme — once the files are in place, "Vanilla Box Dark" appears under **System
Settings → Colors & Themes** and you pick it there. What the installer *does* decide is how the
files are written: see [Preferences](#preferences).

## Build and run

```sh
go build -o vanillabox .
./vanillabox
```

The result is a single static executable with no runtime dependencies. The theme's files are read
from disk rather than embedded, so `assets/` ships alongside the binary.

```sh
CGO_ENABLED=0 go build -o vanillabox .   # explicitly static
```

## Usage

```
vanillabox [--assets DIR] [--version]
```

The asset directory is found from the first of these that contains a `theme.json`:

1. `--assets DIR`
2. `$VANILLABOX_ASSETS`
3. `./assets`
4. `assets/` next to the executable

If none has one, the UI opens on an error screen listing every path it tried.

To see what an install would do without touching your desktop, send it somewhere else:

```sh
XDG_DATA_HOME=/tmp/vbtest ./vanillabox
```

### Keys

| Key | Does |
| --- | --- |
| `↑`/`k`, `↓`/`j` | Move |
| `space` | Choose the value under the cursor, or flip a switch |
| `enter` | Continue, then install |
| `esc` | Back, from the review to the preferences |
| `r` | Start over, from the summary |
| `q` | Quit |

`ctrl+c` always quits, including mid-install.

## The theme

`assets/theme.json` describes the theme and the components it ships. Adding a component is an edit
to that file, not to the code:

```json
{
  "id": "colors",
  "name": "Color scheme",
  "description": "Window and widget colors",
  "source": "color-schemes/VanillaBoxDark.colors",
  "target": "color-schemes",
  "required": true
}
```

`source` is relative to the asset directory; `target` is relative to `~/.local/share`.

All five are marked `required`, so all five install and there is nothing to pick between:

| Component | Installs to |
| --- | --- |
| Color scheme | `color-schemes/VanillaBoxDark.colors` |
| Plasma style | `plasma/desktoptheme/vanilla-box-dark/` |
| Window decoration | `aurorae/themes/VanillaBoxDark/` |
| Global theme | `plasma/look-and-feel/org.vanillabox.dark/` |
| Icons | `icons/VanillaBoxIconsDark/` |

A component whose `source` is missing — or is an empty file or directory — is skipped rather than
failing the run, and the review screen lists it as `unavailable, will be skipped`. To see that,
point the installer at a partial tree:

```sh
./vanillabox --assets /path/to/incomplete/assets
```

## Preferences

The preferences are the only thing to decide, and they are asked over three pages:

| Page | Asks |
| --- | --- |
| Colour | the palette |
| Shape | corners, titlebar corners, window buttons |
| Transparency | the three switches |

`enter` moves to the next page and, from the last, to the review; `esc` steps back. Every value of
every choice is listed under it, so the alternatives are visible without operating anything. On a
short terminal a page scrolls, marking how much sits above and below.

Which page a preference appears on is `"group"` in `theme.json`, not something the UI decides, so
adding an option means saying where it belongs rather than editing a screen.

| Preference | Values | Changes |
| --- | --- | --- |
| Colour | Neutral, Slate, Plum, Rose, Forest | surfaces **and** the accent that goes with them |
| Panel & popup corners | Square, Rounded | the panel strip, menus, tooltips, applet backgrounds |
| Button & input corners | Square, Rounded | buttons, search bars, list items |
| Titlebar corners | Square, Rounded | the top corners of the window frame |
| Window buttons | Traffic lights, Symbols | close, minimise and maximise |
| Translucent panel | on/off | `widgets/panel-background.svg` |
| Translucent popups & menus | on/off | `dialogs/background.svg` |
| Translucent applets | on/off | `widgets/background.svg` |

Every default is the first value listed, so accepting each prompt installs Neutral, square corners
throughout and traffic-light buttons — a theme with no rounded corners anywhere.

A colour is a surface tint and an accent chosen together, not two separate questions. Accents match
their surfaces in temperature, so each variant reads as one decision. Neutral is the quietest of
them — grey surfaces under a warm tan accent — and the tinted four each carry the tint through the
surfaces, the selection colour and the titlebar alike.

Everything the theme ships is installed. There is no component checklist: choosing a colour is
already choosing a colour scheme, and the rest of the theme is what makes that colour mean
anything. The review screen still lists every file destination before a byte is written.

Corners are three preferences rather than one, because the three are independent in practice.
Rounded panels around square buttons is a real look; so is a rounded titlebar over square
everything else. The containers a thing sits in and the things sitting in them do not have to
agree, and they are given different radii when both are rounded — 8 for panels and popups, 6 for
buttons — because a button rounded as hard as the popup around it looks like it is trying to
escape.

The titlebar's rounded variant cannot round its *bottom* corners — see the comment in
`decoration.svg` — so square is the only shape there without a compromise.

Traffic lights are grey circles that take a muted colour on hover, and never show a symbol. They
stay on the right, where KDE puts window buttons; macOS would put them on the left, and you can
move them yourself in System Settings → Window Decorations without reinstalling. Aurorae gives each
button its own artwork, so hovering one lights only that one rather than all three.

An option never edits a file. It only decides which already-written bytes get copied, so the
artwork stays the only place the theme's looks are defined. A switch names an overlay to lay down
when it is off; a choice feeds a `{placeholder}` in a path under `assets/variants/`.

The launcher and the system tray popups cannot be separated: Plasma renders both from one
`dialogs/background`, so one switch covers them and is named for both. Tooltips have no switch
because they are opaque in the base artwork, and a switch that changes nothing is worse than none.

`widgets/tasks.svg` is deliberately outside the transparency switches. Its `0.3`/`0.4` values are
white `normal` and `hover` highlights drawn on top of the panel, not backgrounds — forcing them
opaque would give solid white task buttons.

`opaque/` and `solid/` are installed whichever way the switches go: Plasma falls back to those
prefixes on its own when compositing is off.

A preference is shown when the current selection actually uses it, whether the component declares
it or reads it through a variant path. The step is skipped entirely when nothing selected uses any.

## Generated files

Most of `assets/` is written from `spec/tokens.json` and `spec/icons.json`:

```sh
go generate ./...
```

Adding a colour is an edit to the tokens — a surface set if it needs a new one, a palette pairing
it with an accent, plus the matching value in `theme.json` so the installer offers it. The version
and the theme's identity live there too, and are written into the three KDE metadata files and the
installer's own const.

`go test ./internal/gen` fails if the committed assets and the spec have drifted apart, and CI
runs `go generate` and fails on a dirty tree. Anything under `assets/variants/` or `assets/icons/`
the generator no longer produces is deleted rather than left behind. See [DESIGN.md](DESIGN.md)
for what is generated and what is not.

## Icons

The icon theme is built from [Phosphor](https://phosphoricons.com) (MIT) in the **light** weight.
`spec/icons.json` maps a KDE icon name to the Phosphor icon that answers it, and the artwork
itself is vendored under `spec/phosphor/` at the commit that file pins.

Adding an icon is two steps:

```sh
$EDITOR spec/icons.json                 # "status/network-vpn": "shield"
go run ./internal/gen -fetch            # vendor anything newly named
go generate ./...
```

`-fetch` is the only part that touches the network, and it is never run by `go generate`: a build
has to work offline and produce the same bytes every time.

`color` is the colour they are painted, `#ebebeb`. It is the one thing in the theme that does not
follow the colour scheme: an icon that defers is repainted in the scheme's text colour, so having
a colour of its own means not deferring. Remove the key and the icons follow the scheme again.

`glyph` in the same file is the size in pixels, measured in the standard 22px box: the shipped
`16` gives a 16.0px icon with a 3px margin. That is Breeze's own metric, and Breeze is where every
unmapped icon still comes from — so the two sets are drawn to the same size on a panel showing
both. Phosphor unscaled would be 17.9px.

Only the icons the shell draws are mapped — tray, launcher, applet chrome, kickoff, session actions —
and `index.theme` inherits `breeze-dark` for everything else, which is most things. Application
icons in the menu come from each application's own `.desktop` file and are not ours to change.

KDE caches icon lookups, so a reinstall may keep showing the old set until the cache is rebuilt:

```sh
kbuildsycoca6 --noincremental
/usr/libexec/plasma-changeicons VanillaBoxIconsDark
```

## Applying a decoration change

KWin reads an Aurorae theme once per session. Reinstalling writes new files but the titlebar keeps
using what it already loaded, so a change to the window decoration — button sizes, positions, the
titlebar layout — will not appear until KWin reloads. A restart is known to do it.

If a decoration change seems not to have worked, check
`~/.local/share/aurorae/themes/VanillaBoxDark/VanillaBoxDarkrc` before doubting the change itself.

## Backups

Installing over an existing copy moves it to:

```
~/.local/share/vanillabox/backups/<timestamp>/<target>/<name>
```

Backups deliberately land outside the theme directories. Plasma scans those, so a backup left
beside the real thing would show up in System Settings as a second theme.

## Layout

```
main.go                     flags, asset-dir resolution, program startup
spec/tokens.json            the hand-edited input to the generated artwork
spec/icons.json             KDE icon name -> Phosphor icon, and the families
spec/phosphor/              vendored Phosphor sources, pinned by icons.json
internal/theme/
  manifest.go               Theme, Component, Option and Choices, LoadManifest
  availability.go           whether each component's files are actually present
  install.go                copying into place, backups, overlays, resolved files
  copy.go                   copyTree and move
  version.go                generated from spec/tokens.json
internal/gen/               go generate ./... -> assets/
  frame.go                  panel and popup frames
  control.go                buttons, inputs and list items
  aurorae.go                window decoration and titlebar buttons
  colors.go                 colour schemes
  icons.go                  the icon theme and its index
  fetch.go                  -fetch, which vendors the Phosphor sources
  identity.go               the metadata KDE wants in three formats
internal/ui/
  model.go                  root model, screen state machine, install plumbing
  keys.go                   bindings, enabled per screen so help stays honest
  styles.go                 lipgloss styles, adapting to light or dark terminals
  screen_*.go               the five screens plus the error view
assets/                     theme.json and the theme's files
  variants/                 the alternatives each preference picks from
  icons/                    the icon theme, one scalable dir per context
```

## Tests

```sh
go test ./...
```

`internal/ui` drives the whole flow headlessly — running the commands the model returns and feeding
the messages back in, the way the runtime does. `internal/theme` installs the real shipped assets
into a temporary `$XDG_DATA_HOME`, both with transparency on and off, so the artwork and the
manifest are checked against each other rather than against a fixture.
