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

Four components ship:

All four are marked `required`, so all four install and there is nothing to pick between:

| Component | Installs to |
| --- | --- |
| Color scheme | `color-schemes/VanillaBoxDark.colors` |
| Plasma style | `plasma/desktoptheme/vanilla-box-dark/` |
| Window decoration | `aurorae/themes/VanillaBoxDark/` |
| Global theme | `plasma/look-and-feel/org.vanillabox.dark/` |

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
| Colour | Neutral, Ash, Slate, Moss, Rose, Plum | surfaces **and** the accent that goes with them |
| Corners | Square, Rounded | panels, popups, buttons, inputs, list items |
| Titlebar corners | Square, Rounded | the top corners of the window frame |
| Window buttons | Symbols, Traffic lights | close, minimise and maximise |
| Translucent panel | on/off | `widgets/panel-background.svg` |
| Translucent popups & menus | on/off | `dialogs/background.svg` |
| Translucent applets | on/off | `widgets/background.svg` |

Every default is the first value listed, so accepting each prompt installs Neutral, square corners
throughout and symbol buttons — a theme with no rounded corners anywhere.

A colour is a surface tint and an accent chosen together, not two separate questions. Accents match
their surfaces in temperature, so each variant reads as one decision. Neutral and Ash share the
same grey surfaces and differ only in whether anything on screen is coloured: Neutral has a warm
tan accent, Ash has none.

Everything the theme ships is installed. There is no component checklist: choosing a colour is
already choosing a colour scheme, and the rest of the theme is what makes that colour mean
anything. The review screen still lists every file destination before a byte is written.

Corners and titlebar corners are separate preferences because the two are independent: rounded
panels under a square titlebar is a reasonable thing to want. The titlebar's rounded variant cannot
round its *bottom* corners — see the comment in `decoration.svg` — so square is the only shape
there without a compromise.

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

Most of `assets/` is written from `spec/tokens.json`:

```sh
go generate ./...
```

Adding a colour is an edit to that file — a surface set if it needs a new one, a palette pairing it
with an accent, plus the matching value in `theme.json` so the installer offers it. The version and
the theme's identity live there too, and are written into the three KDE metadata files and the
installer's own const.

`go test ./internal/gen` fails if the committed assets and the tokens have drifted apart, and CI
runs `go generate` and fails on a dirty tree. Anything under `assets/variants/` the generator no
longer produces is deleted rather than left behind. See [DESIGN.md](DESIGN.md) for what is
generated and what is not.

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
spec/tokens.json            the only hand-edited input to the generated artwork
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
  identity.go               the metadata KDE wants in three formats
internal/ui/
  model.go                  root model, screen state machine, install plumbing
  keys.go                   bindings, enabled per screen so help stays honest
  styles.go                 lipgloss styles, adapting to light or dark terminals
  screen_*.go               the five screens plus the error view
assets/                     theme.json and the theme's files
  variants/                 the alternatives each preference picks from
```

## Tests

```sh
go test ./...
```

`internal/ui` drives the whole flow headlessly — running the commands the model returns and feeding
the messages back in, the way the runtime does. `internal/theme` installs the real shipped assets
into a temporary `$XDG_DATA_HOME`, both with transparency on and off, so the artwork and the
manifest are checked against each other rather than against a fixture.
