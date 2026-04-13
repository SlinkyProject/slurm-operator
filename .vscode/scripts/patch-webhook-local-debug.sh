#!/bin/bash
# SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

CERT_DIR="${TMPDIR:-/tmp}/k8s-webhook-server/serving-certs"
BACKUP_DIR="/tmp/webhook-config-backup"
WEBHOOK_PORT="9443"
API_VERSION="v1beta1"

# On macOS/Docker Desktop, containers resolve the host via host.docker.internal.
# On Linux, the Docker bridge gateway is directly reachable from containers.
if [[ "$(uname -s)" == "Darwin" ]]; then
	WEBHOOK_HOST="host.docker.internal"
	BASE_URL="https://${WEBHOOK_HOST}:${WEBHOOK_PORT}"
else
	WEBHOOK_HOST=$(docker network inspect kind -f '{{(index .IPAM.Config 0).Gateway}}')
	if [[ ${WEBHOOK_HOST} =~ : ]]; then
		# If host contains colons, it's IPv6
		BASE_URL="https://[${WEBHOOK_HOST}]:${WEBHOOK_PORT}"
	else
		# Otherwise, no need to wrap it in brackets
		BASE_URL="https://${WEBHOOK_HOST}:${WEBHOOK_PORT}"
	fi
fi

# Create directory to store backups of the current webhook configuration
mkdir -p "${BACKUP_DIR}"

# Create directory to store certs for the webhook
mkdir -p "${CERT_DIR}"

# Compose a comma-separated list of Subject Alternative Names for which to generate a certificate
if [[ ${WEBHOOK_HOST} =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	# IPv4
	SAN="DNS:localhost,DNS:host.docker.internal,IP:127.0.0.1,IP:${WEBHOOK_HOST}"
elif [[ ${WEBHOOK_HOST} =~ : ]]; then
	# IPv6 (contains colons)
	SAN="DNS:localhost,DNS:host.docker.internal,IP:127.0.0.1,IP:${WEBHOOK_HOST}"
else
	# Hostname
	SAN="DNS:localhost,DNS:host.docker.internal,DNS:${WEBHOOK_HOST},IP:127.0.0.1"
fi

# Generate cert for webhooks
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
	-keyout "${CERT_DIR}/tls.key" -out "${CERT_DIR}/tls.crt" \
	-subj '/CN=localhost' \
	-addext "subjectAltName=${SAN}"

# Write out existing webhook configurations to backup files in TMP
kubectl get validatingwebhookconfiguration slurm-operator-webhook -o yaml >"${BACKUP_DIR}/validating.yaml"
kubectl get mutatingwebhookconfiguration slurm-operator-webhook -o yaml >"${BACKUP_DIR}/mutating.yaml"

# Put the CA bundle in a variable
CA_BUNDLE=$(cat "${CERT_DIR}/tls.crt" | base64 | tr -d '\n')

# Patch the webhooks
kubectl patch validatingwebhookconfiguration slurm-operator-webhook --type='json' -p='[
  {"op": "replace", "path": "/webhooks/0/clientConfig", "value": {"url": "'"${BASE_URL}"'/validate-slinky-slurm-net-'"${API_VERSION}"'-accounting", "caBundle": "'"${CA_BUNDLE}"'"}},
  {"op": "replace", "path": "/webhooks/0/timeoutSeconds", "value": 30},
  {"op": "replace", "path": "/webhooks/1/clientConfig", "value": {"url": "'"${BASE_URL}"'/validate-slinky-slurm-net-'"${API_VERSION}"'-controller", "caBundle": "'"${CA_BUNDLE}"'"}},
  {"op": "replace", "path": "/webhooks/1/timeoutSeconds", "value": 30},
  {"op": "replace", "path": "/webhooks/2/clientConfig", "value": {"url": "'"${BASE_URL}"'/validate-slinky-slurm-net-'"${API_VERSION}"'-loginset", "caBundle": "'"${CA_BUNDLE}"'"}},
  {"op": "replace", "path": "/webhooks/2/timeoutSeconds", "value": 30},
  {"op": "replace", "path": "/webhooks/3/clientConfig", "value": {"url": "'"${BASE_URL}"'/validate-slinky-slurm-net-'"${API_VERSION}"'-nodeset", "caBundle": "'"${CA_BUNDLE}"'"}},
  {"op": "replace", "path": "/webhooks/3/timeoutSeconds", "value": 30},
  {"op": "replace", "path": "/webhooks/4/clientConfig", "value": {"url": "'"${BASE_URL}"'/validate-slinky-slurm-net-'"${API_VERSION}"'-restapi", "caBundle": "'"${CA_BUNDLE}"'"}},
  {"op": "replace", "path": "/webhooks/4/timeoutSeconds", "value": 30},
  {"op": "replace", "path": "/webhooks/5/clientConfig", "value": {"url": "'"${BASE_URL}"'/validate-slinky-slurm-net-'"${API_VERSION}"'-token", "caBundle": "'"${CA_BUNDLE}"'"}},
  {"op": "replace", "path": "/webhooks/5/timeoutSeconds", "value": 30}
]'

kubectl patch mutatingwebhookconfiguration slurm-operator-webhook --type='json' -p='[
  {"op": "replace", "path": "/webhooks/0/clientConfig", "value": {"url": "'"${BASE_URL}"'/mutate--v1-binding", "caBundle": "'"${CA_BUNDLE}"'"}},
  {"op": "replace", "path": "/webhooks/0/timeoutSeconds", "value": 30},
]'
