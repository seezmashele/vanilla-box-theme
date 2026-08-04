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
| `space` | Toggle a component or preference |
| `←`/`h`, `→`/`l` | Change a preference that has more than two values |
| `a` / `n` | Select all / none |
| `enter` | Continue, then install |
| `esc` | Back |
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
  "default": true
}
```

`source` is relative to the asset directory; `target` is relative to `~/.local/share`.

Four components ship:

| Component | Installs to |
| --- | --- |
| Color scheme | `color-schemes/VanillaBoxDark.colors` |
| Plasma style | `plasma/desktoptheme/vanilla-box-dark/` |
| Window decoration | `aurorae/themes/VanillaBoxDark/` |
| Global theme | `plasma/look-and-feel/org.vanillabox.dark/` |

Components whose `source` is missing — or is an empty file or directory — are shown greyed out and
labelled `unavailable`, and cannot be selected. To see that state, point the installer at a partial
tree:

```sh
./vanillabox --assets /path/to/incomplete/assets
```

## Preferences

Five preferences ship. Two are choices; three are switches.

| Preference | Values | Changes |
| --- | --- | --- |
| Surface colour | Neutral, Slate, Rose | panels, popups and window backgrounds |
| Accent | Sand, Ash, Moss, Steel, Clay, Plum | selection, highlights, the active task |
| Translucent panel | on/off | `widgets/panel-background.svg` |
| Translucent popups & menus | on/off | `dialogs/background.svg` |
| Translucent applets | on/off | `widgets/background.svg` |

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

Adding a tint or an accent is an edit to that file — plus the matching value in `theme.json`, so
the installer offers it. `go test ./internal/gen` fails if the committed assets and the tokens have
drifted apart. See [DESIGN.md](DESIGN.md) for what is generated and what is not.

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
internal/theme/
  manifest.go               Theme, Component and Option, LoadManifest, path helpers
  availability.go           whether each component's files are actually present
  install.go                copying a component into place, backups, overlays
  copy.go                   copyTree and move
internal/ui/
  model.go                  root model, screen state machine, install plumbing
  keys.go                   bindings, enabled per screen so help stays honest
  styles.go                 lipgloss styles, adapting to light or dark terminals
  screen_*.go               the five screens plus the error view
assets/                     theme.json and the theme's files
```

## Tests

```sh
go test ./...
```

`internal/ui` drives the whole flow headlessly — running the commands the model returns and feeding
the messages back in, the way the runtime does. `internal/theme` installs the real shipped assets
into a temporary `$XDG_DATA_HOME`, both with transparency on and off, so the artwork and the
manifest are checked against each other rather than against a fixture.
