# Multi-cluster Multi-tenant Deployment

In environments where a single Git repo contains configuration for multiple
Slinky clusters each with multiple tenants, Argo ApplicationSets can be used to
reduce redundant per-cluster configuration. Argo uses the configuration provided
in an ApplicationSet to automatically generate ArgoCD Applications based on
provided parameters.

## Installation

1. Copy the directory `hack/sample-deployments/multi-tenant` from the
   [Slurm-operator repo] into your Git repository at the same path, or update
   every repository-relative path in the manifests. Replace every
   `https://github.com/EXAMPLE_ORG/EXAMPLE_GITOPS.git` with that repository's
   URL and adjust `main` if needed. The Git generators and `$values` source must
   point to the same revision. Configure Argo CD to read the repository.

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

1. Bootstrap only the cluster overlay, not `clusters/base`:

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

   The two `ldif.yaml` files are Kubernetes ConfigMaps with sample directory
   entries. The sample users have no passwords or SSH keys; add authentication
   material through a secure process before allowing user logins. OpenLDAP runs
   without persistence for this demo, and LDIF changes do not automatically
   re-import into a running directory. The sample SSSD settings also skip TLS
   certificate validation; configure a trusted CA and set
   `ldap_tls_reqcert = hard` before production usage.

[slurm-operator repo]: https://github.com/SlinkyProject/slurm-operator
