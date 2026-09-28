# Simple Deployment

For single-tenant, single-cluster environments, it is recommended that the
Application CRD is used to represent each component of the Slinky stack.

## Installation

1. Copy the directory `hack/sample-deployments/simple` from the
   [Slurm-operator repo] into your Git repository at the same path, or update
   every repository-relative path in the manifests. Replace
   `https://github.com/EXAMPLE_ORG/EXAMPLE_GITOPS.git` in the bootstrap command
   below with its URL. Make sure to use a Git revision that contains the copied
   directory.

1. Install Argo CD and configure a default StorageClass. If cert-manager is
   already installed on your cluster, `cert-manager.yaml` may be safely removed
   from `kustomization.yaml`

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

[slurm-operator repo]: https://github.com/SlinkyProject/slurm-operator
