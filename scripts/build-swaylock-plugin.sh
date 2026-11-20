#!/bin/sh
# Builds swaylock-plugin into ~/.local/bin without root.
#
# Two things differ from a normal build:
# - The PAM headers come from the libpam0g-dev package, unpacked locally.
# - The PAM service name is changed to "swaylock", so the existing
#   /etc/pam.d/swaylock is used and nothing has to be installed in /etc.
set -eu

B=${XDG_CACHE_HOME:-$HOME/.cache}/meltlock
SRC=$B/swaylock-plugin
PAM=$B/pamdev/root/usr
mkdir -p "$B/pamdev"

[ -x "$B/venv/bin/meson" ] || { python3 -m venv "$B/venv" && "$B/venv/bin/pip" -q install meson; }

[ -d "$SRC" ] || git clone -q --depth 1 https://github.com/mstoeckl/swaylock-plugin.git "$SRC"
# Ubuntu 24.04 ships wayland-protocols 1.45; the build needs 1.47.
[ -d "$SRC/subprojects/wayland-protocols" ] || git clone -q --depth 1 --branch 1.47 \
	https://gitlab.freedesktop.org/wayland/wayland-protocols.git "$SRC/subprojects/wayland-protocols"

if [ ! -f "$PAM/include/security/pam_appl.h" ]; then
	(cd "$B/pamdev" && apt-get download libpam0g-dev && dpkg -x libpam0g-dev_*.deb root)
	ln -sfn /lib/x86_64-linux-gnu/libpam.so.0 "$PAM/lib/x86_64-linux-gnu/libpam.so"
fi

sed -i 's/pam_start("swaylock-plugin"/pam_start("swaylock"/' "$SRC/pam.c"

export C_INCLUDE_PATH="$PAM/include" LIBRARY_PATH="$PAM/lib/x86_64-linux-gnu" PATH="$B/venv/bin:$PATH"
rm -rf "$SRC/build"
meson setup "$SRC/build" "$SRC" --prefix="$HOME/.local" --buildtype=release \
	-Dman-pages=disabled -Dwayland-protocols:tests=false >/dev/null
ninja -C "$SRC/build" >/dev/null
install -Dm755 "$SRC/build/swaylock-plugin" "$HOME/.local/bin/swaylock-plugin"
"$HOME/.local/bin/swaylock-plugin" --version
