#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

# https://kind.sigs.k8s.io/docs/user/quick-start/

set -euo pipefail

ROOT_DIR="$(readlink -f "$(dirname "$0")/..")"
DIR="$(readlink -f "$(dirname "$0")/")"

function kind::prerequisites() {
	go install sigs.k8s.io/kind@latest
	go install sigs.k8s.io/cloud-provider-kind@latest
}

function sys::check() {
	local fail=false
	if ! command -v docker >/dev/null 2>&1 && ! command -v podman >/dev/null 2>&1; then
		echo "'docker' or 'podman' is required:"
		echo "docker: https://www.docker.com/"
		echo "podman: https://podman.io/"
		fail=true
	fi
	if ! command -v go >/dev/null 2>&1; then
		echo "'go' is required: https://go.dev/"
		fail=true
	fi
	if ! command -v helm >/dev/null 2>&1; then
		echo "'helm' is required: https://helm.sh/"
		fail=true
	fi
	if ! command -v skaffold >/dev/null 2>&1; then
		echo "'skaffold' is required: https://skaffold.dev/"
		fail=true
	fi
	if ! command -v yq >/dev/null 2>&1; then
		echo "'yq' is required: https://github.com/mikefarah/yq"
		fail=true
	fi
	if ! command -v kubectl >/dev/null 2>&1; then
		echo "'kubectl' is recommended: https://kubernetes.io/docs/reference/kubectl/"
	fi
	if [[ $OSTYPE == 'linux'* ]]; then
		if [ "$(sysctl -n kernel.keys.maxkeys)" -lt 2000 ]; then
			echo "Recommended to increase 'kernel.keys.maxkeys':"
			echo "  $ sudo sysctl -w kernel.keys.maxkeys=2000"
			echo "  $ echo 'kernel.keys.maxkeys=2000' | sudo tee --append /etc/sysctl.d/kernel.conf"
		fi
		if [ "$(sysctl -n fs.file-max)" -lt 10000000 ]; then
			echo "Recommended to increase 'fs.file-max':"
			echo "  $ sudo sysctl -w fs.file-max=10000000"
			echo "  $ echo 'fs.file-max=10000000' | sudo tee --append /etc/sysctl.d/fs.conf"
		fi
		if [ "$(sysctl -n fs.inotify.max_user_instances)" -lt 65535 ]; then
			echo "Recommended to increase 'fs.inotify.max_user_instances':"
			echo "  $ sudo sysctl -w fs.inotify.max_user_instances=65535"
			echo "  $ echo 'fs.inotify.max_user_instances=65535' | sudo tee --append /etc/sysctl.d/fs.conf"
		fi
		if [ "$(sysctl -n fs.inotify.max_user_watches)" -lt 1048576 ]; then
			echo "Recommended to increase 'fs.inotify.max_user_watches':"
			echo "  $ sudo sysctl -w fs.inotify.max_user_watches=1048576"
			echo "  $ echo 'fs.inotify.max_user_watches=1048576' | sudo tee --append /etc/sysctl.d/fs.conf"
		fi
	fi

	if $fail; then
		exit 1
	fi
}

function kind::start() {
	sys::check
	kind::prerequisites
	local cluster_name="${1:-"kind"}"
	local kind_config="${2:-"$ROOT_DIR/hack/kind-config.yaml"}"
	if [ "$(kind get clusters | grep -oc kind)" -eq 0 ]; then
		if [ "$(command -v systemd-run)" ]; then
			CMD="systemd-run --scope --user"
		else
			CMD=""
		fi
		$CMD kind create cluster --name "$cluster_name" --config "$kind_config"
	fi
	kubectl cluster-info --context kind-"$cluster_name"
}

function helm::find() {
	local item="$1"
	if [ -z "$item" ]; then
		return 0
	elif [ "$(helm list --all-namespaces --short --filter="^${item}$" | wc -l)" -eq 0 ]; then
		return 1
	fi
	return 0
}

function kind::delete() {
	kind delete cluster --name "$cluster_name"
}

function slurm-operator-crds::install() {
	(
		cd "$ROOT_DIR"/helm/slurm-operator-crds
		skaffold run
	)
}

function slurm-operator::prerequisites() {
	local chartName

	helm repo add jetstack https://charts.jetstack.io
	helm repo update

	chartName=cert-manager
	if ! helm::find "$chartName"; then
		helm install "$chartName" jetstack/cert-manager \
			--namespace cert-manager --create-namespace \
			--set 'crds.enabled=true'
	fi
}

function slurm-operator::install() {
	slurm-operator::prerequisites
	(
		cd "$ROOT_DIR"/helm/slurm-operator
		skaffold run
	)
}

function slurm::install() {
	(
		cd "$ROOT_DIR"/helm/slurm
		skaffold run
	)
}

function extras::install() {
	local chartName

	helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
	helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo add kedacore https://kedacore.github.io/charts
	helm repo add nfs-ganesha https://kubernetes-sigs.github.io/nfs-ganesha-server-and-external-provisioner/
	helm repo update

	chartName=mariadb-operator
	if ! helm::find "$chartName"; then
		helm install "$chartName" mariadb-operator/mariadb-operator \
			--namespace mariadb --create-namespace \
			--set 'crds.enabled=true'
	fi

	chartName=metrics-server
	if ! helm::find "$chartName"; then
		helm install "$chartName" metrics-server/metrics-server \
			--namespace metrics-server --create-namespace \
			--set args="{--kubelet-insecure-tls}"
	fi

	chartName=prometheus
	if ! helm::find "$chartName"; then
		helm install "$chartName" prometheus-community/kube-prometheus-stack \
			--namespace prometheus --create-namespace \
			--set installCRDs=true \
			--set 'prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false'
	fi

	chartName=keda
	if ! helm::find "$chartName"; then
		helm install "$chartName" kedacore/keda \
			--namespace keda --create-namespace
	fi

	chartName=nfs-server-provisioner
	if ! helm::find "$chartName"; then
		helm install "$chartName" nfs-ganesha/nfs-server-provisioner \
			--namespace nfs --create-namespace
	fi
}

function main::help() {
	cat <<EOF
$(basename "$0") - Manage a kind cluster for local testing/development

	usage: $(basename "$0") [--config=KIND_CONFIG_PATH]
	        [--recreate|--delete]
	        [--core][--extras][--all]
	        [--crds][--operator][--slurm]
	        [-h|--help] [KIND_CLUSTER_NAME]

KIND OPTIONS:
	--config=PATH       Use the specified Kind config when creating.
	--recreate          Delete the Kind cluster and continue.
	--delete            Delete the Kind cluster and exit.

HELM OPTIONS:
	--all               Equivalent of: --core --extras
	--extras            Install extra charts (e.g. prometheus, keda, etc..).
	--core              Equivalent of: --crds --operator --slurm
	--crds              Install the operator CRDs chart.
	--operator          Install the operator chart.
	--slurm             Install the slurm chart.

HELP OPTIONS:
	--debug             Show script debug information.
	-h, --help          Show this help message.

EOF
}

OPT_DEBUG=false
OPT_RECREATE=false
OPT_CONFIG="$ROOT_DIR/hack/kind.yaml"
OPT_DELETE=false
OPT_OPERATOR_CRDS=false
OPT_OPERATOR=false
OPT_SLURM=false
OPT_EXTRAS=false

SHORT="+h"
LONG="debug,config:,recreate,delete,crds,operator,slurm,all,extras,core,help"
OPTS="$(getopt -a --options "$SHORT" --longoptions "$LONG" -- "$@")"
eval set -- "${OPTS}"
while :; do
	case "$1" in
	--debug)
		OPT_DEBUG=true
		shift
		;;
	--config)
		OPT_CONFIG="$2"
		shift 2
		;;
	--recreate)
		OPT_RECREATE=true
		shift
		;;
	--delete)
		OPT_DELETE=true
		shift
		;;
	--crds)
		OPT_OPERATOR_CRDS=true
		shift
		;;
	--operator)
		OPT_OPERATOR=true
		shift
		;;
	--slurm)
		OPT_SLURM=true
		shift
		;;
	--all)
		OPT_OPERATOR_CRDS=true
		OPT_OPERATOR=true
		OPT_SLURM=true
		OPT_EXTRAS=true
		shift
		;;
	--extras)
		OPT_EXTRAS=true
		shift
		;;
	--core)
		OPT_OPERATOR_CRDS=true
		OPT_OPERATOR=true
		OPT_SLURM=true
		shift
		;;
	-h | --help)
		main::help
		shift
		exit 0
		;;
	--)
		shift
		break
		;;
	*)
		echo "Unknown option: $1" >&2
		exit 1
		;;
	esac
done

function main() {
	if $OPT_DEBUG; then
		set -x
	fi
	local cluster_name="${1:-"kind"}"
	if $OPT_DELETE || $OPT_RECREATE; then
		kind::delete "$cluster_name"
		$OPT_DELETE && return
	fi

	kind::start "$cluster_name" "$OPT_CONFIG"

	make -C "$ROOT_DIR" values-dev || true

	if $OPT_EXTRAS; then
		extras::install
	fi

	if $OPT_OPERATOR_CRDS; then
		slurm-operator-crds::install
	fi
	if $OPT_OPERATOR; then
		slurm-operator::install
	fi
	if $OPT_SLURM; then
		slurm::install
	fi

	if $OPT_EXTRAS; then
		until kubectl apply -f "$DIR"/resources; do
			sleep 2
		done
	fi
}

main "$@"
