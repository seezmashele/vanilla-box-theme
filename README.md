# Vanilla Box

A terminal installer for the Vanilla Box KDE Plasma theme, built with
[Bubble Tea](https://charm.land/bubbletea).

> **Installs are simulated.** The interface is complete — select, review, install, summary — but
> `theme.Install` is a stub that sleeps and reports success. No files are written and no KDE commands
> are run. See [Making it real](#making-it-real).

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

### Keys

| Key | Does |
| --- | --- |
| `↑`/`k`, `↓`/`j` | Move |
| `space` | Toggle a component |
| `a` / `n` | Select all / none |
| `enter` | Continue, then install |
| `esc` | Back |
| `r` | Start over, from the summary |
| `q` | Quit |

`ctrl+c` always quits, including mid-install.

## The theme

`assets/theme.json` describes the theme and the components it ships. Adding a component is an edit to
that file, not to the code:

```json
{
  "id": "colors",
  "name": "Color scheme",
  "description": "Window and widget colors",
  "source": "color-schemes/VanillaBox.colors",
  "target": "color-schemes",
  "applyCmd": "plasma-apply-colorscheme",
  "applyArgs": ["VanillaBox"],
  "default": true
}
```

`source` is relative to the asset directory; `target` is relative to `~/.local/share`.

Components whose `source` is missing — or is an empty file or directory — are shown greyed out and
labelled `unavailable`, and cannot be selected. To see that state, point the installer at a partial
tree:

```sh
./vanillabox --assets /path/to/incomplete/assets
```

The artwork currently in `assets/` is placeholder: valid metadata files with no real graphics.

## Making it real

Everything that pretends lives in [internal/theme/install.go](internal/theme/install.go). Replace the
body of `Install` with, roughly:

1. copy `t.SourcePath(c)` into `t.TargetPath(c)`
2. run `c.ApplyCmd` with `c.ApplyArgs`

and set `Simulated = false` to drop the warnings from the review and summary screens. Nothing in
`internal/ui` needs to change: the UI already runs components one at a time and renders whatever
error comes back.

The Plasma 6 commands the manifest expects: `plasma-apply-colorscheme`, `plasma-apply-desktoptheme`,
`plasma-apply-cursortheme`, `plasma-apply-wallpaperimage`, and `kwriteconfig6` for icons and window
decoration.

## Layout

```
main.go                     flags, asset-dir resolution, program startup
internal/theme/
  manifest.go               Theme and Component, LoadManifest, path helpers
  availability.go           whether each component's files are actually present
  install.go                the stub — the one seam to real installs
internal/ui/
  model.go                  root model, screen state machine, install plumbing
  keys.go                   bindings, enabled per screen so help stays honest
  styles.go                 lipgloss styles, adapting to light or dark terminals
  screen_*.go               the four screens plus the error view
assets/                     theme.json and the theme's files
```

## Tests

```sh
go test ./...
```

`internal/ui` drives the whole flow headlessly — running the commands the model returns and feeding
the messages back in, the way the runtime does — so the install path is covered without a terminal.
