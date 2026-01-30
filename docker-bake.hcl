// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

################################################################################

variable "REGISTRY" {
  default = "ghcr.io/slinkyproject"
}

variable "VERSION" {
  default = "0.0.0"
}

function "format_tag" {
  params = [registry, stage, version]
  result = format("%s:%s", join("/", compact([registry, stage])), join("-", compact([version])))
}

################################################################################

target "_common" {
  labels = {
    # Ref: https://github.com/opencontainers/image-spec/blob/v1.0/annotations.md
    "org.opencontainers.image.authors" = "slinky@schedmd.com"
    "org.opencontainers.image.documentation" = "https://github.com/togethercomputer/slurm-operator"
    "org.opencontainers.image.license" = "Apache-2.0"
    "org.opencontainers.image.vendor" = "SchedMD LLC."
    "org.opencontainers.image.version" = "${VERSION}"
    "org.opencontainers.image.source" = "https://github.com/togethercomputer/slurm-operator"
    # Ref: https://docs.redhat.com/en/documentation/red_hat_software_certification/2025/html/red_hat_openshift_software_certification_policy_guide/assembly-requirements-for-container-images_openshift-sw-cert-policy-introduction#con-image-metadata-requirements_openshift-sw-cert-policy-container-images
    "vendor" = "SchedMD LLC."
    "version" = "${VERSION}"
    "release" = "https://github.com/togethercomputer/slurm-operator"
  }
}

target "_multiarch" {
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}

################################################################################

group "default" {
  targets = [
    "operator",
    "webhook",
  ]
}

################################################################################

target "operator" {
  inherits = ["_common", "_multiarch"]
  dockerfile = "Dockerfile"
  target = "manager"
  labels = {
    # Ref: https://github.com/opencontainers/image-spec/blob/v1.0/annotations.md
    "org.opencontainers.image.title" = "Slurm Operator"
    "org.opencontainers.image.description" = "Kubernetes Operator for Slurm"
    # Ref: https://docs.redhat.com/en/documentation/red_hat_software_certification/2025/html/red_hat_openshift_software_certification_policy_guide/assembly-requirements-for-container-images_openshift-sw-cert-policy-introduction#con-image-metadata-requirements_openshift-sw-cert-policy-container-images
    "name" = "Slurm Operator"
    "summary" = "Kubernetes Operator for Slurm"
    "description" = "Kubernetes Operator for Slurm"
  }
  tags = [
    format_tag("${REGISTRY}", "slurm-operator", "${VERSION}"),
  ]
}

################################################################################

target "webhook" {
  inherits = ["_common", "_multiarch"]
  dockerfile = "Dockerfile"
  target = "webhook"
  labels = {
    # Ref: https://github.com/opencontainers/image-spec/blob/v1.0/annotations.md
    "org.opencontainers.image.title" = "Slurm Operator Webhook"
    "org.opencontainers.image.description" = "Kubernetes Operator Webhook for Slurm"
    # Ref: https://docs.redhat.com/en/documentation/red_hat_software_certification/2025/html/red_hat_openshift_software_certification_policy_guide/assembly-requirements-for-container-images_openshift-sw-cert-policy-introduction#con-image-metadata-requirements_openshift-sw-cert-policy-container-images
    "name" = "Slurm Operator Webhook"
    "summary" = "Kubernetes Operator Webhook for Slurm"
    "description" = "Kubernetes Operator Webhook for Slurm"
  }
  tags = [
    format_tag("${REGISTRY}", "slurm-operator-webhook", "${VERSION}"),
  ]
}
