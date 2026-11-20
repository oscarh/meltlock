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
- The sway config runs `swayidle -w before-sleep meltlock lock meltlock`,
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

To add one: write `fx_name.go`, add a line to the table, run
`make test` (one test runs every effect) and look at
`meltlock frames DIR 30 300`.

What the author has asked for so far:

- Keep the Catppuccin theme and the uncluttered look.
- Every effect starts from the screenshot and leaves no more than a hint
  of it (12 px cells already guarantee nothing is readable).
- It need not always be dots.
- melt: slow pace; the pool is a thick liquid that heaps up, slumps and
  runs off the sides (tuned twice, constants in `fx_melt.go`).
- More variants are welcome.

Only melt has been seen on a real lock. sand, ripple, drift and life were
judged from `frames` output; their pacing is a first guess.

## Conventions

- Keep code and prose concise.
- Commit messages: plain, no tool or assistant attribution.
