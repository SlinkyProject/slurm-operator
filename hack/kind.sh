#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

# https://kind.sigs.k8s.io/docs/user/quick-start/

set -euo pipefail

ROOT_DIR="$(readlink -f "$(dirname "$0")/..")"
DIR="$(readlink -f "$(dirname "$0")/")"

KUBE_PROMETHEUS_STACK_CHART_REPO="https://prometheus-community.github.io/helm-charts"
KUBE_PROMETHEUS_STACK_CHART_VERSION="88.6.2"

function kind::prerequisites() {
	go install sigs.k8s.io/kind@latest
	go install sigs.k8s.io/cloud-provider-kind@latest
}

function sys::check() {
	local require_build="${1:-true}"
	local fail=false
	if $require_build && ! command -v docker >/dev/null 2>&1 && ! command -v podman >/dev/null 2>&1; then
		echo "'docker' or 'podman' is required:"
		echo "docker: https://www.docker.com/"
		echo "podman: https://podman.io/"
		fail=true
	fi
	if $require_build && ! command -v go >/dev/null 2>&1; then
		echo "'go' is required: https://go.dev/"
		fail=true
	fi
	if ! command -v helm >/dev/null 2>&1; then
		echo "'helm' is required: https://helm.sh/"
		fail=true
	fi
	if $require_build && ! command -v skaffold >/dev/null 2>&1; then
		echo "'skaffold' is required: https://skaffold.dev/"
		fail=true
	fi
	if $require_build && ! command -v yq >/dev/null 2>&1; then
		echo "'yq' is required: https://github.com/mikefarah/yq"
		fail=true
	fi
	if ! command -v kubectl >/dev/null 2>&1; then
		echo "'kubectl' is required: https://kubernetes.io/docs/reference/kubectl/"
		fail=true
	fi
	if $require_build && [[ $OSTYPE == 'linux'* ]]; then
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
	if ! kind get clusters 2>/dev/null | grep -Fxq "$cluster_name"; then
		if [ "$(command -v systemd-run)" ]; then
			CMD="systemd-run --scope --user"
		else
			CMD=""
		fi
		$CMD kind create cluster --name "$cluster_name" --config "$kind_config"
	fi
	kubectl config use-context kind-"$cluster_name"
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
	local cluster_name="${1:-kind}"
	kind delete cluster --name "$cluster_name"
}

function cluster::use_existing() {
	local context
	context="$(kubectl config current-context)"
	echo "Using current kubectl context: $context"
	if $OPT_OPERATOR && [ -z "$OPT_REGISTRY" ]; then
		echo "WARNING: no --registry or SKAFFOLD_DEFAULT_REPO was provided; local images will only be available if Skaffold can load them into a Kind context." >&2
	fi
	kubectl cluster-info
}

function slurm-operator-crds::install() {
	(
		cd "$ROOT_DIR"/helm/slurm-operator-crds
		skaffold run
	)
}

function slurm-operator::prerequisites() {
	local chartName

	chartName=cert-manager
	if ! helm::find "$chartName"; then
		helm install "$chartName" oci://quay.io/jetstack/charts/cert-manager \
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

function metrics::install() {
	local config_dir="$DIR/metrics"

	echo "[metrics] Installing kube-prometheus-stack..."
	helm repo add prometheus-community "$KUBE_PROMETHEUS_STACK_CHART_REPO" --force-update
	helm upgrade --install prometheus prometheus-community/kube-prometheus-stack \
		--version "$KUBE_PROMETHEUS_STACK_CHART_VERSION" \
		--namespace monitoring --create-namespace \
		--values "$config_dir/values.yaml" \
		--wait --timeout=300s
	kubectl apply --kustomize "$config_dir"
	kubectl wait --for=create pod \
		--namespace monitoring \
		--selector=app.kubernetes.io/name=prometheus \
		--timeout=120s
	kubectl wait --for=condition=Ready pod \
		--namespace monitoring \
		--selector=app.kubernetes.io/name=prometheus \
		--timeout=300s
	kubectl wait --for=condition=Available deployment/prometheus-grafana \
		--namespace monitoring \
		--timeout=300s
	echo "[metrics] Ready. Forward the Prometheus UI with:"
	echo "kubectl --namespace monitoring port-forward service/prometheus-kube-prometheus-prometheus 9090:9090"
	echo "[metrics] Forward the Grafana UI with:"
	echo "kubectl --namespace monitoring port-forward service/prometheus-grafana 3000:80"
	echo "[metrics] Grafana username: admin"
	echo "[metrics] Read the Grafana password with:"
	echo "kubectl --namespace monitoring get secret prometheus-grafana --output=jsonpath='{.data.admin-password}' | base64 --decode"
}

function ldap::install() {
	local chartName

	helm repo add helm-openldap https://jp-gouin.github.io/helm-openldap/
	helm repo update helm-openldap

	chartName=openldap
	if ! helm::find "$chartName"; then
		helm install "$chartName" helm-openldap/openldap-stack-ha \
			--namespace ldap --create-namespace \
			--values "$DIR"/openldap-values.yaml
	fi
}

function main::help() {
	cat <<EOF
$(basename "$0") - Manage a kind cluster for local testing/development

	usage: $(basename "$0") [--config=KIND_CONFIG_PATH] [--existing-cluster]
	        [--recreate|--delete]
	        [--core|--prereqs][--extras][--metrics][--all] [--registry=REPO]
	        [--crds][--operator][--slurm]
	        [-h|--help] [KIND_CLUSTER_NAME]

KIND OPTIONS:
	--config=PATH       Use the specified Kind config when creating.
	--existing-cluster  Use the current kubectl context instead of creating or switching to a Kind cluster.
	--registry=REPO     Push locally built images to REPO with Skaffold before deploying.
	                    Can also be set with SKAFFOLD_DEFAULT_REPO.
	--recreate          Delete the Kind cluster and continue.
	--delete            Delete the Kind cluster and exit.

HELM OPTIONS:
	--all               Equivalent of: --core --extras
	--extras            Install extra charts (e.g. prometheus, keda, OpenLDAP, etc..).
	--metrics           Install metrics collection for Slurm Operator.
	--core              Equivalent of: --crds --operator --slurm
	--prereqs           Install operator prerequisites only (cert-manager).
	--crds              Install the operator CRDs chart.
	--operator          Install the operator chart.
	--slurm             Install the slurm chart.

HELP OPTIONS:
	--debug             Show script debug information.
	-h, --help          Show this help message.

EOF
}

function main::validate_options() {
	if $OPT_EXISTING_CLUSTER && { $OPT_DELETE || $OPT_RECREATE; }; then
		echo "--existing-cluster cannot be used with --delete or --recreate." >&2
		exit 1
	fi
	if $OPT_CORE && $OPT_PREREQS; then
		echo "--core and --prereqs cannot be used together." >&2
		exit 1
	fi
}

OPT_DEBUG=false
OPT_RECREATE=false
OPT_CONFIG="$ROOT_DIR/hack/kind.yaml"
OPT_DELETE=false
OPT_EXISTING_CLUSTER=false
OPT_CORE=false
OPT_PREREQS=false
OPT_REGISTRY="${SKAFFOLD_DEFAULT_REPO:-}"
OPT_OPERATOR_CRDS=false
OPT_OPERATOR=false
OPT_SLURM=false
OPT_EXTRAS=false
OPT_METRICS=false

SHORT="+h"
LONG="debug,config:,recreate,delete,existing-cluster,registry:,crds,operator,slurm,all,extras,metrics,core,prereqs,help"
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
	--existing-cluster)
		OPT_EXISTING_CLUSTER=true
		shift
		;;
	--registry)
		OPT_REGISTRY="$2"
		if [ -z "$OPT_REGISTRY" ]; then
			echo "--registry requires a non-empty REPO" >&2
			exit 1
		fi
		export SKAFFOLD_DEFAULT_REPO="$OPT_REGISTRY"
		shift 2
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
		OPT_CORE=true
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
	--metrics)
		OPT_METRICS=true
		shift
		;;
	--core)
		OPT_CORE=true
		OPT_OPERATOR_CRDS=true
		OPT_OPERATOR=true
		OPT_SLURM=true
		shift
		;;
	--prereqs)
		OPT_PREREQS=true
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
	main::validate_options
	local cluster_name="${1:-"kind"}"
	if $OPT_DELETE || $OPT_RECREATE; then
		kind::delete "$cluster_name"
		$OPT_DELETE && return
	fi

	if $OPT_EXISTING_CLUSTER; then
		if $OPT_OPERATOR_CRDS || $OPT_OPERATOR || $OPT_SLURM; then
			sys::check
		else
			sys::check false
		fi
		cluster::use_existing
	else
		kind::start "$cluster_name" "$OPT_CONFIG"
	fi

	if $OPT_OPERATOR_CRDS || $OPT_OPERATOR || $OPT_SLURM; then
		make -C "$ROOT_DIR" values-dev || true
	fi

	if $OPT_PREREQS; then
		slurm-operator::prerequisites
	fi

	if $OPT_EXTRAS; then
		extras::install
		ldap::install
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
		kubectl create namespace slurm --dry-run=client -o yaml | kubectl apply -f -
		until kubectl apply --namespace slurm -f "$DIR"/resources; do
			sleep 2
		done
	fi

	if $OPT_METRICS; then
		metrics::install
	fi
}

main "$@"
