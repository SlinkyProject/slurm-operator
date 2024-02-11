#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

# https://kind.sigs.k8s.io/docs/user/quick-start/

set -euo pipefail

ROOT_DIR="$(readlink -f "$(dirname "$0")/..")"

function kind::prerequisites() {
	go install sigs.k8s.io/kind@latest
}

function kind::start() {
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

function kind::delete() {
	kind delete cluster --name "$cluster_name"
}

function helm::install() {
	slurm-operator::prerequisites
	slurm::prerequisites

	cd "$ROOT_DIR"
	make install
}

function helm::uninstall() {
	local namespace=(
		"slurm"
		"slinky"
		"keda"
		"metrics-server"
		"prometheus"
		"cert-manager"
	)
	for name in "${namespace[@]}"; do
		if [ "$(helm --namespace="$name" list --all --short | wc -l)" -gt 0 ]; then
			helm uninstall --namespace="$name" "$(helm --namespace="$name" ls --all --short)"
		fi
	done

	cd "$ROOT_DIR"
	make uninstall
}

function slurm::prerequisites() {
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo add kedacore https://kedacore.github.io/charts
	helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm repo update

	local prometheus="prometheus"
	if [ "$(helm list --all-namespaces --short --filter="$prometheus" | wc -l)" -eq 0 ]; then
		helm install "$prometheus" prometheus-community/kube-prometheus-stack \
			--namespace "$prometheus" --create-namespace --set installCRDs=true \
			--set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false

	fi
	local keda="keda"
	if [ "$(helm list --all-namespaces --short --filter="$keda" | wc -l)" -eq 0 ]; then
		helm install "$keda" kedacore/keda \
			--namespace "$keda" --create-namespace
	fi
	local metrics="metrics-server"
	if [ "$(helm list --all-namespaces --short --filter="$metrics" | wc -l)" -eq 0 ]; then
		helm install "$metrics" metrics-server/metrics-server \
			--set args="{--kubelet-insecure-tls}" \
			--namespace "$metrics" --create-namespace
	fi
}

function slurm::helm() {
	slurm::prerequisites

	local slurm_values_yaml="$ROOT_DIR/helm/slurm/values-dev.yaml"
	if [ ! -f "$slurm_values_yaml" ]; then
		echo "ERROR: Missing values file: $slurm_values_yaml"
		exit 1
	fi
	local helm_release="slurm"
	if [ "$(helm list --all-namespaces --short --filter="$helm_release" | wc -l)" -eq 0 ]; then
		helm install "$helm_release" "$ROOT_DIR"/helm/slurm/ -f "$slurm_values_yaml"
	else
		echo "WARNING: helm release '$helm_release' exists. Skipping."
	fi
}

function slurm::skaffold() {
	slurm::prerequisites
	(
		cd "$ROOT_DIR"/helm/slurm
		skaffold run
	)
}

function slurm-operator::prerequisites() {
	helm repo add jetstack https://charts.jetstack.io
	helm repo update

	local certManager="cert-manager"
	if [ "$(helm list --all-namespaces --short --filter="$certManager" | wc -l)" -eq 0 ]; then
		helm install "$certManager" jetstack/cert-manager \
			--namespace "$certManager" --create-namespace --set crds.enabled=true
	fi
}

function slurm-operator::helm() {
	slurm-operator::prerequisites

	local slurm_values_yaml="$ROOT_DIR/helm/slurm-operator/values-dev.yaml"
	if [ ! -f "$slurm_values_yaml" ]; then
		echo "ERROR: Missing values file: $slurm_values_yaml"
		exit 1
	fi
	local helm_release="slurm-operator"
	if [ "$(helm list --all-namespaces --short --filter="$helm_release" | wc -l)" -eq 0 ]; then
		helm install "$helm_release" "$ROOT_DIR"/helm/slurm-operator/ -f "$slurm_values_yaml"
	else
		echo "WARNING: helm release '$helm_release' exists. Skipping."
	fi
}

function slurm-operator::skaffold() {
	slurm-operator::prerequisites
	(
		cd "$ROOT_DIR"/helm/slurm-operator
		skaffold run
	)
}

function main::help() {
	cat <<EOF
$(basename "$0") - Manage a kind cluster for local testing/development

	usage: $(basename "$0") [--create|--delete] [--config=KIND_CONFIG_PATH]
	        [--install|--uninstall] [--operator] [--slurm] [--helm]
	        [-h|--help] [KIND_CLUSTER_NAME]

ONESHOT OPTIONS:
	--create            Create kind cluster and nothing else.
	--delete            Delete kind cluster and nothing else.
	--install           Install dependent helm releases and nothing else.
	--uninstall         Uninstall all helm releases and nothing else.

OPTIONS:
	--config=PATH       Use the specified kind config when creating.
	--helm              Deploy with helm instead of skaffold.
	--operator          Deploy helm/slurm-operator with skaffold.
	--slurm             Deploy helm/slurm with skaffold.

HELP OPTIONS:
	--debug             Show script debug information.
	-h, --help          Show this help message.

EOF
}

function main() {
	if $FLAG_DEBUG; then
		set -x
	fi
	local cluster_name="${1:-"kind"}"
	if $FLAG_DELETE; then
		kind::delete "$cluster_name"
		return
	elif $FLAG_UNINSTALL; then
		helm::uninstall
		return
	elif $FLAG_CREATE; then
		kind::start "$cluster_name" "$FLAG_CONFIG"
		return
	fi

	kind::start "$cluster_name" "$FLAG_CONFIG"

	if $FLAG_INSTALL; then
		helm::install
		return
	fi
	if $FLAG_OPERATOR; then
		if $FLAG_HELM; then
			slurm-operator::helm
		else
			slurm-operator::skaffold
		fi
	fi
	if $FLAG_SLURM; then
		if $FLAG_HELM; then
			slurm::helm
		else
			slurm::skaffold
		fi
	fi
}

FLAG_DEBUG=false
FLAG_CREATE=false
FLAG_CONFIG="$ROOT_DIR/hack/kind-config.yaml"
FLAG_DELETE=false
FLAG_HELM=false
FLAG_INSTALL=false
FLAG_UNINSTALL=false
FLAG_SLURM=false
FLAG_OPERATOR=false

SHORT="+h"
LONG="create,config:,delete,debug,helm,slurm,operator,install,uninstall,help"
OPTS="$(getopt -a --options "$SHORT" --longoptions "$LONG" -- "$@")"
eval set -- "${OPTS}"
while :; do
	case "$1" in
	--debug)
		FLAG_DEBUG=true
		shift
		;;
	--create)
		FLAG_CREATE=true
		shift
		if $FLAG_CREATE && $FLAG_DELETE; then
			echo "Flags --create and --delete are mutually exclusive!"
			exit 1
		fi
		;;
	--config)
		FLAG_CONFIG="$2"
		shift 2
		;;
	--delete)
		FLAG_DELETE=true
		shift
		if $FLAG_CREATE && $FLAG_DELETE; then
			echo "Flags --create and --delete are mutually exclusive!"
			exit 1
		fi
		;;
	--helm)
		FLAG_HELM=true
		shift
		;;
	--slurm)
		FLAG_SLURM=true
		shift
		;;
	--operator)
		FLAG_OPERATOR=true
		shift
		;;
	--install)
		FLAG_INSTALL=true
		shift
		if $FLAG_INSTALL && $FLAG_UNINSTALL; then
			echo "Flags --install and --uninstall are mutually exclusive!"
			exit 1
		fi
		;;
	--uninstall)
		FLAG_UNINSTALL=true
		shift
		if $FLAG_INSTALL && $FLAG_UNINSTALL; then
			echo "Flags --install and --uninstall are mutually exclusive!"
			exit 1
		fi
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
		log::error "Unknown option: $1"
		exit 1
		;;
	esac
done
main "$@"