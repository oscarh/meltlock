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
| `scene.go` | Snapshot file and the animated simulation |
| `paint.go` | Dot masks and incremental painting into a buffer |
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

With swaylock-plugin the dot wall is animated: drips run down the picture
and wear it away, a pool collects at the bottom, bands glitch sideways,
and the date and a running clock are shown.

- `meltlock` captures a `.snap` per output (cell grid only) into
  `$XDG_RUNTIME_DIR/meltlock` and execs
  `swaylock-plugin --command-each 'meltlock bg'`.
- `meltlock bg` is a wallpaper client written against the raw Wayland
  protocol (`wl.go`, `bg.go`): wl_shm buffers on a layer surface, only
  changed cells repainted, paced by frame callbacks so it stops drawing
  when the output is off. `scene.go` is the simulation, `paint.go` the dots.
- `scripts/build-swaylock-plugin.sh` builds swaylock-plugin into
  `~/.local/bin` without root. It patches the PAM service name to
  `swaylock` so the existing `/etc/pam.d/swaylock` is used.
- Animated is the default when swaylock-plugin is found;
  `MELTLOCK_STATIC=1` forces the still picture with plain swaylock.
- `scripts/trial.sh` (installed as `meltlock-trial`) runs the animated lock
  and unlocks automatically after 60 s. The author runs it; agents do not.

The author has run the animated lock through `meltlock-trial` and liked
it; pace was then slowed on request (constants in `scene.go`: spawn rate,
drip speed, wear per pass, pool growth, trail fade).

The pool is a thick liquid (`land` and `flow` in `scene.go`): each drip
lands as a heap that slumps sideways, runs off at the screen edges and
seeps away. `meltlock frames DIR 180 183 186` shows a heap relaxing.

Next: the author wants more animation variants of different kinds.

To look at the animation without locking: `meltlock frames DIR` writes
PNGs at 1 s, 8 s, 45 s, 4 min and 20 min; `timeout 5 meltlock bg overlay`
shows it live on top of everything (it takes the pointer while it runs).

## Conventions

- Keep code and prose concise.
- Commit messages: plain, no tool or assistant attribution.
