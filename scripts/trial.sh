#!/bin/sh
# Safe trial of the animated lock (swaylock-plugin + `meltlock bg`).
#
# Locks the screen exactly as meltlock will once the animated lock is the
# default. Unlock with your password. If the screen is still locked after
# $TRIAL_SECONDS (default 60), it is unlocked automatically, so a lock that
# misbehaves does not leave you stuck.
set -eu

secs=${TRIAL_SECONDS:-60}
bin=$HOME/.local/bin

locked() { pgrep -x swaylock-plugin >/dev/null; }

start=$(date +%s)
"$bin/meltlock"
sleep 1
if ! locked; then
	echo "The lock did not start."
	exit 1
fi

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

while locked; do sleep 0.5; done
took=$(($(date +%s) - start))
if [ "$took" -ge "$secs" ]; then
	echo "Not unlocked by password within ${secs}s; it was unlocked automatically."
else
	echo "Unlocked by password after ${took}s."
fi
