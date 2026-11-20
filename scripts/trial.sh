#!/bin/sh
# Safe trial of swaylock-plugin.
#
# Locks the screen with swaylock-plugin showing the meltlock picture. Try to
# unlock with your password. If the screen is still locked after
# $TRIAL_SECONDS (default 45), it is unlocked automatically, so a locker
# that cannot check the password does not leave you stuck.
set -eu

secs=${TRIAL_SECONDS:-45}
dir=${XDG_RUNTIME_DIR}/meltlock
bin=$HOME/.local/bin

img=$("$bin/meltlock" preview "$dir" | head -n 1 | cut -d: -f2-)

(
	sleep "$secs"
	# SIGUSR1 asks the locker to unlock cleanly.
	pkill -USR1 -x swaylock-plugin || exit 0
	sleep 3
	# Still there: replace it with plain swaylock, which is known to work.
	if pkill -KILL -x swaylock-plugin; then
		swaylock -f -c 1e1e2e
	fi
) &

start=$(date +%s)
"$bin/swaylock-plugin" --ignore-empty-password --command-each "swaybg -m fill -i $img"
status=$?
took=$(($(date +%s) - start))

if [ "$took" -ge "$secs" ]; then
	echo "Not unlocked by password within ${secs}s; it was unlocked automatically."
else
	echo "Unlocked after ${took}s (exit $status)."
fi
