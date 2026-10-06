#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

# https://kind.sigs.k8s.io/docs/user/quick-start/

set -euo pipefail

ROOT_DIR="$(readlink -f "$(dirname "$0")/..")"
DIR="$(readlink -f "$(dirname "$0")/")"

KUBE_PROMETHEUS_STACK_CHART_REPO="https://prometheus-community.github.io/helm-charts"
KUBE_PROMETHEUS_STACK_CHART_VERSION="88.6.2"

KWOK_CHART_REPO="https://kwok.sigs.k8s.io/charts/"
KWOK_CHART_VERSION="0.3.0"

KIND_VERSION="0.33.0"

function kind::require_version() {
	if ! command -v kind >/dev/null 2>&1; then
		echo "'kind' is required: https://kind.sigs.k8s.io/" >&2
		return 1
	fi
	local have
	if ! have="$(kind version 2>/dev/null | awk '{print $2}' | sed 's/^v//')" || [ -z "$have" ]; then
		echo "Could not determine 'kind' version." >&2
		return 1
	fi
	if [ "$have" != "$KIND_VERSION" ]; then
		echo "'kind' $have is unsupported (need exactly $KIND_VERSION): https://kind.sigs.k8s.io/" >&2
		return 1
	fi
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

function kind::defaults() {
	export KUBERNETES_VERSION="${KUBERNETES_VERSION:-v1.37}"
	# CI node image pins for Kind v0.33.0. Update these with KIND_VERSION.
	case "$KUBERNETES_VERSION" in
	v1.35 | v1.35.*)
		KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.35.8@sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0}"
		OPT_CONFIG="${OPT_CONFIG:-$DIR/kind.yaml}"
		;;
	v1.36 | v1.36.*)
		KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed}"
		OPT_CONFIG="${OPT_CONFIG:-$DIR/kind.yaml}"
		;;
	v1.37 | v1.37.*)
		KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5}"
		OPT_CONFIG="${OPT_CONFIG:-$DIR/kind.yaml}"
		;;
	*)
		if [ -z "${KIND_NODE_IMAGE:-}" ] || [ -z "$OPT_CONFIG" ]; then
			echo "Unsupported KUBERNETES_VERSION '$KUBERNETES_VERSION'; select v1.35, v1.36 or v1.37, or set KIND_NODE_IMAGE and KIND_CONFIG." >&2
			return 1
		fi
		;;
	esac
}

function kind::start() {
	sys::check
	local cluster_name="${1:-"kind"}"
	local kind_config="${2:-"$DIR/kind.yaml"}"
	if ! kind get clusters 2>/dev/null | grep -Fxq "$cluster_name"; then
		if [ "$(command -v systemd-run)" ]; then
			CMD="systemd-run --scope --user"
		else
			CMD=""
		fi
		$CMD kind create cluster --name "$cluster_name" --config "$kind_config" --image "$KIND_NODE_IMAGE"
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
	slurm-operator::wait_webhook
}

function slurm-operator::wait_webhook() {
	kubectl wait --for=condition=Available deployment/slurm-operator-webhook \
		-n slinky --timeout=120s || return 1

	# Pod readiness can precede Service routing updates. Exercise admission from
	# the API server without persisting a resource or requiring a running Slurm.
	echo "[slurm] Waiting for slurm-operator webhook admission..."
	local deadline=$((SECONDS + 120))
	local output=""
	local request_timeout
	while ((SECONDS < deadline)); do
		request_timeout=$((deadline - SECONDS))
		if ((request_timeout <= 0)); then
			break
		fi
		if ((request_timeout > 10)); then
			request_timeout=10
		fi
		if output="$(
			kubectl create --dry-run=server --namespace=slinky \
				--request-timeout="${request_timeout}s" -f - 2>&1 <<-EOF
					apiVersion: slinky.slurm.net/v1beta1
					kind: RestApi
					metadata:
					  generateName: slurm-operator-webhook-check-
					spec:
					  controllerRef:
					    name: slurm-operator-webhook-check
				EOF
		)"; then
			echo "[slurm] Slurm-operator webhook admission is ready."
			return 0
		fi
		sleep 2
	done

	echo "[slurm] Timed out waiting for slurm-operator webhook admission after 120s." >&2
	printf '%s\n' "$output" >&2
	return 1
}

function slurm::install() {
	(
		cd "$ROOT_DIR"/helm/slurm
		skaffold run
	)
}

function mariadb::install() {
	local chartName=mariadb-operator
	helm repo add mariadb-operator https://helm.mariadb.com/mariadb-operator
	helm repo update mariadb-operator
	if ! helm::find "$chartName"; then
		helm install "$chartName" mariadb-operator/mariadb-operator \
			--namespace mariadb --create-namespace \
			--set 'crds.enabled=true'
	fi
}

function metrics_server::install() {
	local chartName=metrics-server
	helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
	helm repo update metrics-server
	if ! helm::find "$chartName"; then
		helm install "$chartName" metrics-server/metrics-server \
			--namespace metrics-server --create-namespace \
			--set args="{--kubelet-insecure-tls}"
	fi
}

function keda::install() {
	local chartName=keda
	helm repo add kedacore https://kedacore.github.io/charts
	helm repo update kedacore
	if ! helm::find "$chartName"; then
		helm install "$chartName" kedacore/keda \
			--namespace keda --create-namespace
	fi
}

function kwok::install() {
	echo "[kwok] Installing the KWOK controller and fast stage configuration..."
	helm repo add kwok "$KWOK_CHART_REPO" --force-update
	helm upgrade --install kwok kwok/kwok \
		--version "$KWOK_CHART_VERSION" \
		--namespace kube-system \
		--wait --timeout=300s

	# Without stages, pods on fake nodes never reach Running.
	helm upgrade --install kwok-stage-fast kwok/stage-fast \
		--version "$KWOK_CHART_VERSION" \
		--namespace kube-system \
		--wait --timeout=300s
}

function nfs::install() {
	local chartName=nfs-server-provisioner
	helm repo add nfs-ganesha https://kubernetes-sigs.github.io/nfs-ganesha-server-and-external-provisioner/
	helm repo update nfs-ganesha
	if ! helm::find "$chartName"; then
		helm install "$chartName" nfs-ganesha/nfs-server-provisioner \
			--namespace nfs --create-namespace
	fi
}

function metrics::install() {
	local config_dir="$DIR/metrics"

	metrics_server::install

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
	        [--core|--prereqs][--extras][--mariadb][--keda][--metrics]
	        [--nfs][--ldap][--kwok][--all] [--registry=REPO]
	        [--crds][--operator][--slurm]
	        [--print-image] [-h|--help] [KIND_CLUSTER_NAME]

KIND OPTIONS:
	--config=PATH       Use the specified Kind config when creating.
	                    Can also be set with KIND_CONFIG.
	--print-image       Print the selected node image and exit without creating a cluster.
	                    KUBERNETES_VERSION selects v1.35, v1.36 or v1.37 (default).
	                    Set KIND_NODE_IMAGE to override the node image for all nodes.
	--existing-cluster  Use the current kubectl context instead of creating or switching to a Kind cluster.
	--registry=REPO     Push locally built images to REPO with Skaffold before deploying.
	                    Can also be set with SKAFFOLD_DEFAULT_REPO.
	--recreate          Delete the Kind cluster and continue.
	--delete            Delete the Kind cluster and exit.

HELM OPTIONS:
	--all               Equivalent of: --core --extras
	--extras            Equivalent of: --mariadb --metrics --nfs --ldap
	--mariadb           Install mariadb-operator and the example MariaDB CR.
	--keda              Install KEDA.
	--metrics           Install metrics-server and kube-prometheus-stack.
	--nfs               Install the NFS provisioner and example PVCs.
	--ldap              Install OpenLDAP.
	--kwok              Install KWOK. Create nodes with hack/kwok-nodes.sh.
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

function extras::enable() {
	OPT_EXTRAS=true
	OPT_MARIADB=true
	OPT_METRICS=true
	OPT_NFS=true
	OPT_LDAP=true
}

OPT_DEBUG=false
OPT_RECREATE=false
OPT_CONFIG="${KIND_CONFIG:-}"
OPT_PRINT_IMAGE=false
OPT_DELETE=false
OPT_EXISTING_CLUSTER=false
OPT_CORE=false
OPT_PREREQS=false
OPT_REGISTRY="${SKAFFOLD_DEFAULT_REPO:-}"
OPT_OPERATOR_CRDS=false
OPT_OPERATOR=false
OPT_SLURM=false
OPT_EXTRAS=false
OPT_MARIADB=false
OPT_KEDA=false
OPT_NFS=false
OPT_LDAP=false
OPT_METRICS=false
OPT_KWOK=false

SHORT="+h"
LONG="debug,config:,print-image,recreate,delete,existing-cluster,registry:,crds,operator,slurm,all,extras,mariadb,keda,metrics,nfs,ldap,kwok,core,prereqs,help"
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
	--print-image)
		OPT_PRINT_IMAGE=true
		shift
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
		extras::enable
		shift
		;;
	--extras)
		extras::enable
		shift
		;;
	--mariadb)
		OPT_MARIADB=true
		shift
		;;
	--keda)
		OPT_KEDA=true
		shift
		;;
	--kwok)
		OPT_KWOK=true
		shift
		;;
	--metrics)
		OPT_METRICS=true
		shift
		;;
	--nfs)
		OPT_NFS=true
		shift
		;;
	--ldap)
		OPT_LDAP=true
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
	if $OPT_PRINT_IMAGE; then
		kind::defaults
		printf '%s\n' "$KIND_NODE_IMAGE"
		return
	fi
	if ! $OPT_EXISTING_CLUSTER && ! $OPT_DELETE; then
		kind::require_version
		kind::defaults
	fi
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

	if $OPT_MARIADB; then
		mariadb::install
	fi
	if $OPT_METRICS; then
		metrics::install
	fi
	if $OPT_KEDA; then
		keda::install
	fi
	if $OPT_NFS; then
		nfs::install
	fi
	if $OPT_LDAP; then
		ldap::install
	fi
	if $OPT_KWOK; then
		kwok::install
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

	if $OPT_MARIADB || $OPT_NFS || $OPT_EXTRAS; then
		kubectl create namespace slurm --dry-run=client -o yaml | kubectl apply -f -
	fi
	if $OPT_MARIADB; then
		until kubectl apply --namespace slurm -f "$DIR"/resources/mariadb.yaml; do
			sleep 2
		done
	fi
	if $OPT_NFS; then
		until kubectl apply --namespace slurm \
			-f "$DIR"/resources/pvc-nfs-data.yaml \
			-f "$DIR"/resources/pvc-nfs-home.yaml \
			-f "$DIR"/resources/pvc-statesave.yaml; do
			sleep 2
		done
	fi
	if $OPT_EXTRAS; then
		until kubectl apply --namespace slurm -f "$DIR"/resources/token.yaml; do
			sleep 2
		done
	fi
}

main "$@"
