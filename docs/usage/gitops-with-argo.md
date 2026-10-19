# Deployment with ArgoCD

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Deployment with ArgoCD](#deployment-with-argocd)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Prerequisites](#prerequisites)
  - [Simple Deployment](#simple-deployment)
    - [Installation](#installation)
  - [Multi-cluster Multi-tenant Deployment](#multi-cluster-multi-tenant-deployment)
    - [Installation](#installation-1)

<!-- mdformat-toc end -->

## Overview

Argo CD is a GitOps continuous delivery tool for Kubernetes. When used with
Slurm-operator, Argo can improve the portability of Slurm cluster deployments
and make it convenient and easy to rapidly bring new Slurm clusters up and down
with minimal manual intervention.

The sample deployments described on this page can be found in the
[Slurm-operator repo] in `hack/sample-deployments`.

## Prerequisites

This guide assumes that you have a functional Kubernetes cluster with Argo CD
installed and a default StorageClass configured. For information on installing
Argo CD, see [the official documentation].

## Simple Deployment

For single-tenant, single-cluster environments, it is recommended that the
[Application CRD] is used to represent each component of the Slinky stack. The
`simple` deployment provided in `hack/sample-deployments` contains `Application`
CRDs that represent both the Slurm-operator and the Slurm Helm charts.

### Installation

1. Copy the directory `hack/sample-deployments/simple` from the
   [Slurm-operator repo] into your Git repository at the same path, or update
   every repository-relative path in the manifests. Replace
   `https://github.com/EXAMPLE_ORG/EXAMPLE_GITOPS.git` in the bootstrap command
   below with its URL. Make sure to use a Git revision that contains the copied
   directory.

1. Apply the parent application on your cluster:

   ```bash
   kubectl apply -n argocd -f - <<'EOF'
   apiVersion: argoproj.io/v1alpha1
   kind: Application
   metadata:
     name: slurm-simple-bootstrap
   spec:
     project: default
     source:
       repoURL: https://github.com/EXAMPLE_ORG/EXAMPLE_GITOPS.git
       targetRevision: main
       path: hack/sample-deployments/simple
     destination:
       server: https://kubernetes.default.svc
       namespace: argocd
     syncPolicy:
       automated:
         prune: true
         selfHeal: true
   EOF
   ```

Argo will retry deployment of the Slurm application until the Slurm-operator
deployment comes up.

## Multi-cluster Multi-tenant Deployment

In environments where a single Git repo contains configuration for multiple
Slinky clusters each with multiple tenants, Argo [ApplicationSets] can be used
to reduce redundant per-cluster configuration. Argo uses the configuration
provided in an ApplicationSet to automatically generate ArgoCD Applications
based on provided parameters.

### Installation

1. Copy the directory `hack/sample-deployments/multi-tenant` from the
   [Slurm-operator repo] into your Git repository at the same path, or update
   every repository-relative path in the manifests. Replace every
   `https://github.com/EXAMPLE_ORG/EXAMPLE_GITOPS.git` with that repository's
   URL. Make sure to use a Git revision that contains the copied directory: the
   Git generators and `$values` source must point to the same revision.
   Configure Argo CD to read the repository.

1. Create each tenant namespace and its `openldap-bootstrap` Secret with
   `LDAP_ADMIN_PASSWORD` and `LDAP_CONFIG_ADMIN_PASSWORD` keys. Generate unique
   passwords outside Git, for example:

   ```sh
   for t in alpha beta; do
     kubectl create namespace "tenant-$t"
     kubectl create secret generic openldap-bootstrap -n "tenant-$t" \
       --from-literal=LDAP_ADMIN_PASSWORD="$(openssl rand -base64 24)" \
       --from-literal=LDAP_CONFIG_ADMIN_PASSWORD="$(openssl rand -base64 24)"
   done
   ```

1. Bootstrap the cluster overlay:

   ```sh
   kubectl apply -n argocd -f - <<'EOF'
   apiVersion: argoproj.io/v1alpha1
   kind: Application
   metadata:
     name: slurm-multi-bootstrap
   spec:
     project: default
     source:
       repoURL: https://github.com/EXAMPLE_ORG/EXAMPLE_GITOPS.git
       targetRevision: main
       path: hack/sample-deployments/multi-tenant/clusters/demo
     destination:
       server: https://kubernetes.default.svc
       namespace: argocd
     syncPolicy:
       automated:
         prune: true
         selfHeal: true
   EOF
   ```

   The `ldif.yaml` file under each tenant is a Kubernetes ConfigMap with sample
   LDAP directory entries. The sample users have no passwords or SSH keys; add
   authentication material through a secure process before allowing user logins.
   OpenLDAP runs without persistence for this demo, and LDIF changes do not
   automatically re-import into a running directory. The sample SSSD settings
   also skip TLS certificate validation; configure a trusted CA and set
   `ldap_tls_reqcert = hard` before production usage.

<!-- Links -->

[application crd]: https://argo-cd.readthedocs.io/en/stable/user-guide/application-specification/
[applicationsets]: https://argo-cd.readthedocs.io/en/stable/user-guide/application-set/
[slurm-operator repo]: https://github.com/SlinkyProject/slurm-operator
[the official documentation]: https://argo-cd.readthedocs.io/en/stable/getting_started/
