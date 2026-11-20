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
| `main.go` | Screenshot per output, swaylock arguments, exec |
| `render.go` | PPM reader, grid, melt, stamp, dot renderer |
| `font.go` | 5x7 bitmap font, only the characters the stamp uses |

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

## Animation: swaylock-plugin (in progress)

The plan for an animated lock screen is swaylock-plugin, a swaylock fork
that lets a wallpaper program draw the background.

- `scripts/build-swaylock-plugin.sh` builds it into `~/.local/bin` without
  root. It patches the PAM service name to `swaylock` so the existing
  `/etc/pam.d/swaylock` is used.
- `scripts/trial.sh` (installed as `meltlock-trial`) locks with it and
  unlocks automatically after 45 s. The author runs it; agents do not.
- Not done yet: confirming it locks and unlocks on sway 1.9, and writing
  the animated background program. meltlock still uses plain swaylock.

## Conventions

- Keep code and prose concise.
- Commit messages: plain, no tool or assistant attribution.
