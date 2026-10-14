#!/usr/bin/env bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0
#
# kwok-restapi.sh - restapi for the KWOK scale fixture, with optional
# random failures of slurmrestd and slurmd pod creates.

set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

NAMESPACE="${NAMESPACE:-}"
CONTROLLER="${CONTROLLER:-slurm-kwok}"
SLURM_ERROR_RATE="${SLURM_ERROR_RATE:-0}"
KUBE_ERROR_RATE="${KUBE_ERROR_RATE:-0}"
SLURM_DELAY="${SLURM_DELAY:-0}"
SLURM_JITTER="${SLURM_JITTER:-0}"
DELETE=false

usage() {
	cat <<EOF
	usage: $(basename "$0") [--namespace=NS] [--controller=NAME] \
	          [--slurm=RATE] [--kube=RATE] [--delay=DURATION] [--jitter=DURATION]

OPTIONS:
	--slurm   Chance that a restapi request returns HTTP 500.
	--kube    Chance that a slurmd pod create is denied.
	--delay   Delay added to each restapi request.
	--jitter  Maximum random deviation above or below the delay.
	--delete  Deletes the restapi from deployment.

RATE is a floating point on [0, 1] (e.g. "0.05" is 5%).

DURATION is a duration expression (e.g. "1ms").

The Controller from hack/resources/kwok-nodeset.yaml must already exist.
Re-run to change the rates.

Request counts are served at http://<restapi>:6820/restapi/stats.
EOF
}

SHORT="+h"
LONG="namespace:,controller:,slurm:,kube:,delay:,jitter:,delete,help"
OPTS="$(getopt -a --options "$SHORT" --longoptions "$LONG" -- "$@")"
eval set -- "${OPTS}"
while :; do
	case "$1" in
	--namespace)
		NAMESPACE="$2"
		shift 2
		;;
	--controller)
		CONTROLLER="$2"
		shift 2
		;;
	--slurm)
		SLURM_ERROR_RATE="$2"
		shift 2
		;;
	--kube)
		KUBE_ERROR_RATE="$2"
		shift 2
		;;
	--delay)
		SLURM_DELAY="$2"
		shift 2
		;;
	--jitter)
		SLURM_JITTER="$2"
		shift 2
		;;
	--delete)
		DELETE=true
		shift
		;;
	-h | --help)
		usage
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

if [ -z "$NAMESPACE" ]; then
	NAMESPACE="$(kubectl config view --minify --output 'jsonpath={..namespace}')"
	NAMESPACE="${NAMESPACE:-default}"
fi

if ! kubectl --namespace "$NAMESPACE" get "controller.slinky.slurm.net/$CONTROLLER" >/dev/null 2>&1; then
	echo "Controller $NAMESPACE/$CONTROLLER not found. Apply hack/resources/kwok-nodeset.yaml first." >&2
	exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
	echo "openssl is required to sign the admission webhook certificate." >&2
	exit 1
fi

RESTAPI="${CONTROLLER}"
SERVICE="${RESTAPI}-restapi"
SECRET="${CONTROLLER}-restapi-tls"
ACCOUNT="${CONTROLLER}-restapi"
WEBHOOK="${CONTROLLER}-restapi"
IMAGE="docker.io/library/slurm-kwok-restapi:dev"

echo "[restapi] namespace=$NAMESPACE controller=$CONTROLLER slurm=$SLURM_ERROR_RATE kube=$KUBE_ERROR_RATE delay=$SLURM_DELAY jitter=$SLURM_JITTER" >&2

if [ "$DELETE" = true ]; then
	kubectl -n "$NAMESPACE" delete roles.rbac.authorization.k8s.io "$ACCOUNT"
	kubectl -n "$NAMESPACE" delete rolebindings.rbac.authorization.k8s.io "$ACCOUNT"
	kubectl -n "$NAMESPACE" delete serviceaccounts "$ACCOUNT"
	kubectl -n "$NAMESPACE" delete validatingwebhookconfigurations.admissionregistration.k8s.io "$WEBHOOK"
	kubectl -n "$NAMESPACE" delete secrets "$SECRET"
	kubectl -n "$NAMESPACE" delete restapis.slinky.slurm.net "$RESTAPI"
	exit 0
fi

kubectl --namespace "$NAMESPACE" apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${ACCOUNT}
  namespace: ${NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ${ACCOUNT}
  namespace: ${NAMESPACE}
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${ACCOUNT}
  namespace: ${NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: ${ACCOUNT}
subjects:
  - kind: ServiceAccount
    name: ${ACCOUNT}
    namespace: ${NAMESPACE}
EOF

if ! command -v go >/dev/null 2>&1; then
	echo "go is required to build the restapi engine." >&2
	exit 1
fi
runtime="docker"
if ! command -v docker >/dev/null 2>&1; then
	runtime="podman"
fi
if ! command -v "$runtime" >/dev/null 2>&1; then
	echo "docker or podman is required to build the restapi image." >&2
	exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
echo "[restapi] Building restapi engine..." >&2
(
	cd "$DIR/.."
	CGO_ENABLED=0 GOOS=linux go build -trimpath -o "$work/restapi" ./hack/restapi
)
"$runtime" build -t "$IMAGE" -f "$DIR/restapi/Dockerfile" "$work"
PULL="IfNotPresent"
ctx="$(kubectl config current-context 2>/dev/null || true)"
if [[ $ctx == kind-* ]] && command -v kind >/dev/null 2>&1; then
	"$runtime" save "$IMAGE" -o "$work/image.tar"
	kind load image-archive "$work/image.tar" --name "${ctx#kind-}"
	PULL="Never"
else
	echo "[restapi] Context '$ctx' is not a kind cluster. The image $IMAGE must already be available to the nodes." >&2
fi
CONFIG_HASH="$(sha256sum "$work/restapi" | awk '{print $1}')"

cert_dir="$work"
openssl req -x509 -newkey rsa:2048 \
	-keyout "$cert_dir/tls.key" -out "$cert_dir/tls.crt" \
	-days 365 -nodes \
	-subj "/CN=${SERVICE}.${NAMESPACE}.svc" \
	-addext "subjectAltName=DNS:${SERVICE}.${NAMESPACE}.svc,DNS:${SERVICE}.${NAMESPACE}.svc.cluster.local" \
	2>/dev/null
kubectl --namespace "$NAMESPACE" create secret tls "$SECRET" \
	--cert="$cert_dir/tls.crt" --key="$cert_dir/tls.key" \
	--dry-run=client --output=yaml | kubectl apply -f -
ca_bundle="$(base64 -w0 "$cert_dir/tls.crt")"

kubectl --namespace "$NAMESPACE" apply -f - <<EOF
apiVersion: slinky.slurm.net/v1beta1
kind: RestApi
metadata:
  name: ${RESTAPI}
  namespace: ${NAMESPACE}
spec:
  controllerRef:
    name: ${CONTROLLER}
  replicas: 1
  slurmrestd:
    image: ${IMAGE}
    imagePullPolicy: ${PULL}
    command: ["/restapi"]
    env:
      - name: SLURM_ERROR_RATE
        value: "${SLURM_ERROR_RATE}"
      - name: KUBE_ERROR_RATE
        value: "${KUBE_ERROR_RATE}"
      - name: SLURM_DELAY
        value: "${SLURM_DELAY}"
      - name: SLURM_JITTER
        value: "${SLURM_JITTER}"
      - name: POD_NAMESPACE
        valueFrom:
          fieldRef:
            fieldPath: metadata.namespace
    ports:
      - name: restapi
        containerPort: 8443
    volumeMounts:
      - name: restapi-tls
        mountPath: /tls
        readOnly: true
  template:
    metadata:
      annotations:
        restapi.slinky.slurm.net/config: "${CONFIG_HASH}-${SLURM_ERROR_RATE}-${KUBE_ERROR_RATE}-${SLURM_DELAY}-${SLURM_JITTER}"
    spec:
      automountServiceAccountToken: true
      serviceAccountName: ${ACCOUNT}
      volumes:
        - name: restapi-tls
          secret:
            secretName: ${SECRET}
  service:
    spec:
      ports:
        - name: restapi
          port: 8443
          targetPort: 8443
EOF

kubectl apply -f - <<EOF
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: ${WEBHOOK}
webhooks:
  - name: restapi.slinky.slurm.net
    failurePolicy: Ignore
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 5
    rules:
      - apiGroups: [""]
        apiVersions: ["v1"]
        operations: ["CREATE"]
        resources: ["pods"]
    objectSelector:
      matchLabels:
        app.kubernetes.io/name: slurmd
    clientConfig:
      service:
        name: ${SERVICE}
        namespace: ${NAMESPACE}
        port: 8443
        path: /admit
      caBundle: ${ca_bundle}
EOF

echo "[restapi] Waiting for the restapi pod..." >&2
kubectl --namespace "$NAMESPACE" rollout status "deployment/${SERVICE}" --timeout=180s
echo "[restapi] Ready. Scale the NodeSet, then re-run this script to change the rates." >&2
