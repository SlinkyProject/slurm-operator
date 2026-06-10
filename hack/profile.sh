#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0
#
# profile.sh - Use pprof to profile Slurm-operator components
#

set -euo pipefail

PORT=8123

usage() {
	cat <<EOF
profile.sh - Use pprof to profile Slurm-operator components

	usage: profile.sh [-h|--help] [-b|--builder] [COMPONENT] [-c|--controller] [COMPONENT]

HELP OPTIONS:
	-h, --help          Show this help message.
    -a, --all           Profile both the builder and controller for all components.
    -b, --builder       Profile the builder for the specified component.
    -c, --controller    Profile the reconciliation of the specified component.

COMPONENT can be one of:
    * accounting
    * controller
    * loginset
    * restapi
    * nodeset
EOF
}

trap 'trap - SIGTERM && kill -- -$$' SIGINT SIGTERM

profile_component() {
	case "$1" in
	builder)
		case "$2" in
		accounting)
			filename=accounting_app_test.go
			benchmark=BenchmarkBuilder_BuildAccounting
			;;
		controller)
			filename=controller_app_test.go
			benchmark=BenchmarkBuilder_BuildController
			;;
		loginset)
			filename=login_app_test.go
			benchmark=BenchmarkBuilder_BuildLogin
			;;
		nodeset)
			filename=worker_app_test.go
			benchmark=BenchmarkBuilder_BuildWorkerPodTemplate
			;;
		restapi)
			filename=restapi_app_test.go
			benchmark=BenchmarkBuilder_BuildRestapi
			;;
		esac
		;;
	controller)
		case "$2" in
		accounting)
			filename=accounting_sync_test.go
			benchmark=BenchmarkAccountingReconciler_sync
			;;
		controller)
			filename=controller_sync_test.go
			benchmark=BenchmarkControllerReconciler_sync
			;;
		loginset)
			filename=loginset_sync_test.go
			benchmark=BenchmarkLoginsetReconciler_sync
			;;
		nodeset)
			filename=nodeset_sync_test.go
			benchmark=BenchmarkNodeSetReconciler_sync
			;;
		restapi)
			filename=restapi_sync_test.go
			benchmark=BenchmarkRestapiReconciler_sync
			;;
		esac
		;;
	esac

	filepath=$(find . -name "$filename" | cut -d/ -f-4)
	go test -test.fullpath=true -run '^$$' -bench=$benchmark github.com/SlinkyProject/slurm-operator/$filepath -cpuprofile=$filepath/cpu.prof
	go tool pprof -http=:"$PORT" $filepath/cpu.prof &
	PORT=$((PORT + 1))
}

while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help)
		usage
		exit 0
		;;
	-a | --all)
		builder_components=("accounting" "controller" "loginset" "nodeset" "restapi")
		controller_components=("accounting" "controller" "loginset" "nodeset" "restapi")
		shift 1
		;;
	-b | --builder)
		if [[ $# -lt 2 ]]; then
			builder_components=("accounting" "controller" "loginset" "nodeset" "restapi")
			shift 1
			continue
		fi
		builder_components+=("$(echo "$2" | awk '{print tolower($0)}')")
		shift 2
		;;
	-c | --controller)
		if [[ $# -lt 2 ]]; then
			controller_components=("accounting" "controller" "loginset" "nodeset" "restapi")
			shift 1
			continue
		fi
		controller_components+=("$(echo "$2" | awk '{print tolower($0)}')")
		shift 2
		;;
	*)
		echo "Unknown argument: $1" >&2
		usage >&2
		exit 1
		;;
	esac
done

for b in "${builder_components[@]}"; do
	profile_component "builder" $b
done

for c in "${controller_components[@]}"; do
	profile_component "controller" $c
done

sleep infinity
