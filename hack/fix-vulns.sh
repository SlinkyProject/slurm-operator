#!/bin/bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0
#
# fix-vulns.sh - fix vulnerabilities identified by govulncheck
#

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOVULNCHECK_REPORT="${GOVULNCHECK_REPORT:-${REPO_ROOT}/govulncheck-vulns.csv}"
OUTPUT_FILE="${OUTPUT_FILE:-${REPO_ROOT}/hack/govulncheck-commit-message.txt}"
COMMIT=false

log_info() {
	echo "[INFO] $*" >&2
}

log_error() {
	echo "[ERROR] $*" >&2
}

usage() {
	cat <<EOF
fix-vulns.sh - fix vulnerabilities identified by govulncheck

	usage: fix-vulns.sh [-h|--help] [-c|--commit]

HELP OPTIONS:
	-h, --help		Show this help message.
	-c, --commit		Commit changes using Git

ENVIRONMENT:
	GOVULNCHECK         govulncheck binary; defaults to repo-local bin/govulncheck-latest
	OUTPUT_FILE         File to write commit messages to.
	PACKAGE_PATTERN     Packages to scan; defaults to ./...
EOF
}

write_commit_message() {
	if ! eval "$(grep -Ev "no fix exists|fixed_in_version" "$GOVULNCHECK_REPORT" | tr -d '"' | cut -d ',' -f 1 | tr '\n' ' ' | fold -w 60 -s | sed -e 's/^/ /')"; then
		{
			echo "ci: fix vulns identified by govulncheck"
			echo ""
			echo "Fixed vulnerabilities:"
			grep -Ev "no fix exists|fixed_in_version" "$GOVULNCHECK_REPORT" | tr -d '"' | awk -F, '{print "* " $1 " via " $2 "@" $3}'
			echo ""
			echo "Changelog: Fixed -$(grep -Ev "no fix exists|fixed_in_version" "$GOVULNCHECK_REPORT" | tr -d '"' | cut -d ',' -f 1 | tr '\n' ' ' | fold -w 60 -s | sed -e 's/^/ /')"
		} >"$OUTPUT_FILE"
	else
		exit 0
	fi
}

fix_vulnerabilities() {
	/bin/bash "$REPO_ROOT"/hack/govulncheck-report.sh -o "$GOVULNCHECK_REPORT" || true

	go_version="$(grep -Ev "no fix exists|fixed_in_version" "$GOVULNCHECK_REPORT" | tr -d '"' | awk -F, '$2 == "stdlib" { sub(/^(go|v)/, "", $3); print $3 }' | sort -V -r | head -n 1 || true)"
	if [ -n "$go_version" ]; then
		go mod edit -go="$go_version"
	fi

	versions="$(grep -Ev "no fix exists|fixed_in_version|stdlib" "$GOVULNCHECK_REPORT" | cut -d ',' -f 2- | tr -d '"' | sort -V -r | sort -u -V -t, -k1,1 | awk -F "," '{print $1"@"$2}' || true)"
	if [ -z "$versions" ]; then
		log_info "no module vulnerabilities with fixed versions found"
		return 0
	fi
	for version in $versions; do
		go get "$version"
	done
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		usage
		exit 0
		;;
	-c | --commit)
		COMMIT=true
		shift
		;;
	*)
		echo "Unknown argument: $1" >&2
		usage >&2
		exit 1
		;;
	esac
done

log_info "Running fix vulns..."

/bin/bash "$REPO_ROOT"/hack/govulncheck-report.sh -o "$GOVULNCHECK_REPORT" || true

write_commit_message

# Re-run govulncheck until eventual consistency is reached
for _ in {1..15}; do
	fix_vulnerabilities
	git diff --quiet -- go.mod go.sum && break
	git add go.mod go.sum
done

go mod tidy

git add go.mod go.sum

if [[ $COMMIT == 'true' ]]; then
	git commit -F "$OUTPUT_FILE"
fi
