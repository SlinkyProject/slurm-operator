#!/usr/bin/env sh
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -eu

SOCKET="${SOCKET:-"/tmp/logfile.log"}"

mkdir -v -p "$(dirname "$SOCKET")"
# Create the FIFO only if the path is not already one: removing an existing
# FIFO races a restarting writer, which can open the unlinked inode and block
# in fifo_open forever.
if ! [ -p "$SOCKET" ]; then
	rm -f "$SOCKET"
	mkfifo -m 777 "$SOCKET"
fi
# Reopen after every writer close (EOF); exiting would restart the container,
# recreate the FIFO, and re-expose the unlinked-inode race.
while true; do
	cat "$SOCKET" || true
done
