#!/usr/bin/env sh
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -eu

SOCKET="${SOCKET:-"/tmp/logfile.log"}"

mkdir -v -p "$(dirname "$SOCKET")"
rm -f "$SOCKET"
if ! [ -f "$SOCKET" ]; then
	mkfifo -m 777 "$SOCKET"
fi
while IFS="" read data; do
	echo "$data"
done <"$SOCKET"
