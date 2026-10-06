#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

: "${E2E_KUBE_CONTEXT:?E2E_KUBE_CONTEXT must name the kubeconfig context of the e2e cluster}"

ROOT_DIR="$(readlink -f "$(dirname "$0")/..")"
E2E_TMP="$(mktemp -d)"
trap 'rm -rf "$E2E_TMP"' EXIT

# Scope dependency installation and every child process to the selected context,
# without changing the caller's kubeconfig. Keep embedded credentials private.
umask 077
kubectl --context "$E2E_KUBE_CONTEXT" config view --minify --flatten --raw >"$E2E_TMP/kubeconfig"
export KUBECONFIG="$E2E_TMP/kubeconfig"
export HELM_KUBECONTEXT="$E2E_KUBE_CONTEXT"

"$ROOT_DIR/hack/kind.sh" --existing-cluster --extras
"$@"
