#!/bin/bash
#
# govulncheck-report.sh - run govulncheck -format json
# write a CSV report; exit non-zero when a vulnerability is found that has a fix available.
#

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GOVULNCHECK="${GOVULNCHECK:-${REPO_ROOT}/bin/govulncheck-latest}"
PACKAGE_PATTERN="${PACKAGE_PATTERN:-./...}"
OUTPUT_FILE=""

log_info() {
	echo "[INFO] $*" >&2
}

log_error() {
	echo "[ERROR] $*" >&2
}

jq_govulncheck() {
	local program="${JQ_COMMON}${1}"
	printf '%s\n' "${JSON_OUT}" | jq -s -r "${program}"
}

usage() {
	cat <<EOF
govulncheck-report.sh - Run govulncheck and write a vulnerability CSV report

	usage: govulncheck-report.sh [-o FILE] [-h|--help]

OPTIONS:
	-o FILE             Write CSV to FILE. Use - for stdout.

HELP OPTIONS:
	-h, --help          Show this help message.

ENVIRONMENT:
	GOVULNCHECK         govulncheck binary; defaults to repo-local bin/govulncheck-latest
	PACKAGE_PATTERN     Packages to scan; defaults to ./...
EOF
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		usage
		exit 0
		;;
	-o)
		if [[ $# -lt 2 ]]; then
			echo "Missing path after ${1}" >&2
			exit 1
		fi
		OUTPUT_FILE="$2"
		[[ ${OUTPUT_FILE} == - ]] && OUTPUT_FILE=/dev/stdout
		shift 2
		;;
	*)
		echo "Unknown argument: $1" >&2
		usage >&2
		exit 1
		;;
	esac
done

if ! command -v "${GOVULNCHECK}" >/dev/null 2>&1; then
	log_error "govulncheck not found: ${GOVULNCHECK}"
	exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
	log_error "jq not found"
	exit 1
fi

JQ_COMMON='
def vulnerability_findings:
  [
    .[]
    | select(.finding != null)
    | select((.finding.trace // []) | any(
        ((.package // "") | length > 0) or ((.module // "") | length > 0)
      ))
    | {
        osv: .finding.osv,
        mod: (.finding.trace[0].module // ""),
        fix: .finding.fixed_version
      }
  ]
  | unique_by([.osv, .mod]);

def has_fix:
  if (.fix | type) == "string" then
    (.fix | length) > 0
  else
    false
  end;

def fix_label:
  if has_fix then .fix else "no fix exists" end;
'

JQ_CSV='
vulnerability_findings
| (["osv_id", "module", "fixed_in_version"] | @csv),
  (.[] | [.osv, .mod, fix_label] | @csv)
'

JQ_VULN_WITH_FIX='
vulnerability_findings[]
| select(has_fix)
| "  \(.osv): \(.mod): \(.fix)"
'

JQ_VULN_NO_KNOWN_FIX='
vulnerability_findings[]
| select(has_fix | not)
| "  \(.osv): \(.mod): \(fix_label)"
'

log_info "Running govulncheck..."
JSON_OUT=$("${GOVULNCHECK}" -format json "${PACKAGE_PATTERN}")

VULNS_WITH_FIX=$(jq_govulncheck "${JQ_VULN_WITH_FIX}")
VULNS_NO_KNOWN_FIX=$(jq_govulncheck "${JQ_VULN_NO_KNOWN_FIX}")

if [[ -n ${OUTPUT_FILE} ]]; then
	jq_govulncheck "${JQ_CSV}" >"${OUTPUT_FILE}"
fi

if [[ -n ${VULNS_NO_KNOWN_FIX} ]]; then
	log_info "Vulnerabilities with no known fix:"
	printf '%s\n' "${VULNS_NO_KNOWN_FIX}" >&2
fi

if [[ -n ${VULNS_WITH_FIX} ]]; then
	log_error "Vulnerabilities with fix available:"
	printf '%s\n' "${VULNS_WITH_FIX}" >&2
	exit 1
fi

exit 0
