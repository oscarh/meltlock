# meltlock

A lock screen for sway. It takes a screenshot of each output, reduces it to
a grid of dots in the Catppuccin Mocha palette, lets the bright edges drip,
stamps `LOCKED SINCE hh:mm` on top and hands the picture to `swaylock`.

The dots are 12 px, so nothing on the desktop stays readable, and every
lock looks different.

`swaylock` does the locking and the password check. meltlock only makes the
picture, in about 0.2 s, and starts swaylock even if the picture fails.

## Animated

With `swaylock-plugin` installed (`scripts/build-swaylock-plugin.sh`) the
dots move: drips run down the picture and slowly wear it away, a pool
gathers at the bottom, and the date and a running clock are shown. This is
used whenever swaylock-plugin is found; `MELTLOCK_STATIC=1` forces the
still picture. `meltlock-trial` locks with an automatic unlock after 60 s.

## Use

| How | |
|---|---|
| `Mod+Escape` | bound in `~/.config/sway/config` |
| `Mod+d`, "Lock Screen" | desktop entry |
| `meltlock` | from a terminal |
| `meltlock preview [DIR]` | render the pictures without locking |

## Install

    make install

Needs `swaylock`, `grim` and `swaymsg`.

## Notes

- Pictures are written to `$XDG_RUNTIME_DIR/meltlock`, readable only by you.
- The time on the picture is when the screen was locked; plain swaylock
  cannot show a running clock.
- Indicator colours are the `lockArgs` list in `main.go`.
