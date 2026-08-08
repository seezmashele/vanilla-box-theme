# Plan: stop the hover plate growing past the button

Status: agreed diagnosis, not yet implemented. Background for the terms used here is in
[plasma-controls.md](plasma-controls.md).

## The complaint

Buttons that read as transparent at rest — the lock screen login button, buttons inside system tray
popups — grow a container larger than themselves when hovered. What is wanted is the button's own
background changing colour, and nothing appearing around it. Flat buttons that are transparent at
rest and stay tight on hover (applet header configure, expander arrows) are correct as they are and
must not move.

## Cause

Two things meeting:

1. `ButtonHover.qml` anchors the `hover` frame with negative margins, so Plasma grows it outward
   past the button by that prefix's margins on every side. The prefix is meant to be a halo — Breeze
   paints its `hover-center` at `opacity:0.001`.
2. `controls()` in `internal/gen/main.go` builds `hover` as a solid nine-tile fill
   (`layer{btnBg, "1.0"}, layer{btnHvr, "0.25"}`), and `widgets/button.svg` declares no
   `hint-*-margin` elements anywhere, so its margins fall back to the tile size of 6.

Six-pixel margins on a solid fill means the hover plate is 12px wider and 12px taller than the
button under it.

A second, unrelated reason these buttons read as transparent at rest: `normal` paints
`ColorScheme-ButtonBackground` `#2f2f2f` on a `#292929` popup — six units apart. That is not part of
this fix, but it is why the plate is the first thing you see.

## Scope

Fixed by this change, because they all reach `ButtonHover`: raised `PlasmaComponents3.Button`
(lock screen login, "Configure…" buttons in popups), `ComboBox`, `RoundButton`, `CheckIndicator`.

Untouched, because they anchor normally and already paint the control's own rect: flat toolbuttons
via `toolbutton-hover`, and list, Kickoff and system tray highlights via `widgets/viewitem`.

Note the one place this exceeds the original ask: **dropdowns move too**, because `ComboBox` uses
`ButtonHover`. They have the same oversized plate today, so this fixes them rather than changing
something that was right.

## The change

Emit `hover` as **centre tile only** — no border tiles, no margin hints. With no border elements
FrameSvg reports zero margins, `ButtonHover`'s negative anchors collapse to zero, the frame lands
exactly on the button, and the centre covers precisely its rect.

The centre paints **only the wash**, not the button background again: `surfaceNormal` is already
underneath, so a wash over it is literally the button's background changing colour. Use
`ColorScheme-Text` at `0.08`, the value `toolbutton-hover` already uses, so the two hover
treatments agree.

1. `internal/gen/control.go` — `tileSet` gains a centre-only flag; `geometry()` returns just the
   centre when it is set.
2. `internal/gen/main.go` — `hover` becomes that centre-only set with one wash layer.
3. Tests — pin the invariant with its reason: the `hover` prefix must declare no border tiles,
   because Plasma anchors it with negative margins and any margin it declares becomes overhang.
   Assert that `pressed`, `toolbutton-hover` and `toolbutton-pressed` keep their nine tiles, since
   those are anchored normally and must stay exact.
4. `DESIGN.md` — record the `ButtonHover` quirk under the controls section.

## The trade-off, decided

An exact-bounds hover can only be square-cornered: the centre tile is stretched to the control's
size, so a radius baked into it distorts. `normal` underneath is a 6px rounded rect, so a square
wash over it leaves four slivers of about 1.8px at the corners. At `0.08` they are effectively
invisible, which is the reason for the subtle wash over the current `0.25`.

The alternative — keeping the plate rounded by leaving a small halo — was rejected: a smaller
container is still a container, and the ask was for none.

## Open afterwards

Whether the system tray complaint is fully answered. Raised buttons inside tray popups are fixed by
this. If what is wrong there is a highlight simply *larger than the icon*, that comes from
`widgets/viewitem` through `PlasmaExtras.Highlight`, which fills its delegate exactly and cannot
overhang — a separate change, to be judged on screen after this lands.
