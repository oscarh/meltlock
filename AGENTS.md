# Working on meltlock

Read `README.md` first.

## Status

First iteration. The picture pipeline is verified with `meltlock preview`,
and swaylock accepts the argument list (checked against a non-existent
display so it could not lock). A real lock and unlock has not been done by
an agent, because the agent cannot type the password.

## Rules

- **Never run `meltlock` without `preview`, or `swaylock`, from an agent
  session.** It locks the author's screen and only they can unlock it.
- **Fail closed.** Whatever goes wrong with the picture, swaylock must
  still start. Rendering has a 3 s timeout for that reason.
- Authentication stays in swaylock (session-lock protocol and PAM). Do not
  replace it with a home-made locker without the author asking for it; a
  bug there locks them out of a session with unsaved work.
- Keep cells at 12 px or more so desktop text cannot be read on the lock
  screen.
- Standard library only.

## Layout

| File | Contents |
|---|---|
| `main.go` | Commands, screenshot per output, locker arguments, exec |
| `render.go` | PPM reader, grid, and the still picture (melt, stamp, draw) |
| `scene.go` | Snapshot file, the shared scene and the effect table |
| `fx_*.go` | One animation effect each |
| `paint.go` | Cell shapes and incremental painting into a buffer |
| `wl.go`, `bg.go` | Minimal Wayland client and the background program |
| `font.go` | 5x7 bitmap font: digits, colon, capitals |

## Testing

    make test
    ./meltlock preview /some/dir   # writes one PNG per output, prints paths

Look at the PNGs to judge changes to the effect.

## Outside this repository

- `~/.config/sway/config` binds `$mod+Escape` to `~/.local/bin/meltlock`.
- `~/.local/share/applications/meltlock.desktop` ("Lock Screen").
- The sway config runs `swayidle -w`; `~/.config/swayidle/config` locks
  after 10 minutes idle, turns the screens off after 15, locks before
  sleep, and reloads sway 8 s after resume (waybar 0.9.24 sometimes draws
  two bars on the external screen when it comes back). The sway config also
  binds lid close and `$mod+Shift+Escape` to `systemctl suspend`.
- `~/.local/share/applications/suspend.desktop` ("Suspend").

## Animation

With swaylock-plugin the picture is animated, with the date and a running
clock on top. The author has run it on real locks and likes it.

- `meltlock` captures a `.snap` per output (cell grid only) into
  `$XDG_RUNTIME_DIR/meltlock`, picks an effect and execs
  `swaylock-plugin --command-each 'meltlock bg EFFECT'`.
- `meltlock bg` is a wallpaper client written against the raw Wayland
  protocol (`wl.go`, `bg.go`): wl_shm buffers on a layer surface, only
  changed cells repainted, paced by frame callbacks so it stops drawing
  when the output is off.
- `scripts/build-swaylock-plugin.sh` builds swaylock-plugin into
  `~/.local/bin` without root. It patches the PAM service name to
  `swaylock` so the existing `/etc/pam.d/swaylock` is used.
- Animated is the default when swaylock-plugin is found;
  `MELTLOCK_STATIC=1` forces the still picture with plain swaylock.
- `scripts/trial.sh` (installed as `meltlock-trial`) locks and unlocks
  automatically after 60 s. The author runs it; agents do not.

### Effects

`scene.go` owns what is shared: the picture (`lum`, `col`), vignette,
clock and palette. Each effect is a file `fx_*.go` implementing `effect`
(`step` and `cell`) and is registered in the `effects` table in `scene.go`
with its cell shape (`paint.go`: dot, square, bar, diamond).

An effect that also implements `painter` (`frame`) draws its own pixels at
full resolution; `scene.render` then lays the clock over it. These embed
`canvas` (`canvas.go`), which gives them `frame` and the drawing tools:
`line`, `put`, `splat`, `clear`, `fade`, the opening `picture`. `silk`,
`ridge`, `contour`, `frost` and `lava` work this way (`lava` fills areas
rather than drawing lines: it blends wax amounts from the lattice of cell
corners across each cell, at 15 frames a second). They still read the desktop
only through the cell grid, which is what keeps it unreadable. The canvas
must never hold a channel darker than the background, or `fade` breaks.

Painters cost more CPU than cell effects. Measured live over a few
seconds, share of one core per output at idle clock speed: silk 30%,
ridge 28%, contour 25%, frost 16%, against 6% for melt.
`go test -bench Painters` times one step of each.

`chooseEffect` in `main.go` reads `~/.config/meltlock/effects`
(`name on|off` lines, unmentioned effects are on, `effects.conf` is the
example that `make install` puts there once). `MELTLOCK_EFFECT` overrides
it.

To add one: write `fx_name.go`, add a line to the table, run
`make test` (one test runs every effect) and look at
`meltlock frames DIR 30 300`. A blank or dark desktop makes every effect
look empty; `MELTLOCK_SHOT=picture.ppm` uses a fixed picture instead of a
screenshot (`convert some.png picture.ppm`).

What the author has asked for so far:

- Keep the Catppuccin theme and the uncluttered look.
- Every effect starts from the screenshot and leaves no more than a hint
  of it (12 px cells already guarantee nothing is readable).
- It need not always be dots.
- melt: slow pace; the pool is a thick liquid that heaps up, slumps and
  runs off the sides (tuned twice, constants in `fx_melt.go`).
- More variants are welcome.
- Something that pulses very slowly (`pulse`, added on request).
- Something higher resolution but as dynamic (`silk`, added on request),
  and a small file to choose the effects.
- Three more high-resolution variants (`ridge`, `contour`, `frost`).
- Something like plasma or a lava lamp, starting from the screen (`lava`).

Only melt has been seen on a real lock. sand, ripple, drift, life and
pulse were judged from `frames` output; their pacing is a first guess.

## Ideas

From 2026-11-26, not built:

- Battery-aware effects: on battery pick only the cheap cell effects and
  keep the painters for mains power.
- Status on the lock screen: battery level and perhaps the next calendar
  entry under the clock, in the dot font.
- Sway window borders in Catppuccin Mocha (sway config, not this
  repository).

## Conventions

- Keep code and prose concise.
- Commit messages: plain, no tool or assistant attribution.
