#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0
#
# kwok-nodes.sh - Create KWOK fake nodes for scale testing the operator

set -euo pipefail

COUNT="${1:-100}"

if [ "$#" -gt 1 ] || ! [[ $COUNT =~ ^[1-9][0-9]*$ ]]; then
	echo "usage: $(basename "$0") [COUNT]" >&2
	exit 1
fi

if ! kubectl --namespace kube-system get deployment kwok-controller >/dev/null 2>&1; then
	echo "KWOK is not installed. Run hack/kind.sh with --kwok first." >&2
	exit 1
fi

ARCH="$(kubectl get nodes --selector='!kwok.x-k8s.io/node' \
	--output=jsonpath='{.items[0].status.nodeInfo.architecture}')"

echo "[kwok] Creating $COUNT fake node(s)..." >&2

for ((index = 1; index <= COUNT; index++)); do
	printf -v node_name 'kwok-slurm-worker-%04d' "$index"

	cat <<EOF
---
apiVersion: v1
kind: Node
metadata:
  name: ${node_name}
  annotations:
    kwok.x-k8s.io/node: fake
    node.alpha.kubernetes.io/ttl: "0"
  labels:
    app.kubernetes.io/managed-by: slurm-operator-kwok
    kubernetes.io/arch: ${ARCH}
    kubernetes.io/hostname: ${node_name}
    kubernetes.io/os: linux
    kwok.x-k8s.io/node: fake
spec:
  taints:
    - effect: NoSchedule
      key: kwok.x-k8s.io/node
      value: fake
status:
  allocatable:
    cpu: "32"
    memory: 256Gi
    pods: "110"
  capacity:
    cpu: "32"
    memory: 256Gi
    pods: "110"
  nodeInfo:
    architecture: ${ARCH}
    kubeProxyVersion: fake
    kubeletVersion: fake
    operatingSystem: linux
  phase: Running
EOF
done | kubectl apply -f -

kubectl wait --for=condition=Ready node \
	--selector='kwok.x-k8s.io/node=fake' --timeout=300s
