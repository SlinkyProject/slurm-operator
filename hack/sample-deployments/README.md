# Argo CD sample deployments

These examples assume Argo CD is already installed. Copy the chosen directory
into your own Git repository, replace the example repository URL and revision,
then create an Argo CD Application pointing at its Kustomize directory.

- [simple](simple/README.md): one Slurm cluster, with flat Applications and no
  LDAP.
- [multi-tenant](multi-tenant/README.md): shared platform Applications and
  tenant Applications discovered from nested directories.

Both examples use public charts at a pinned release. A default StorageClass is
needed for the Slurm controller's StateSave PVC. Review resource sizing,
storage, networking, and access control before using either layout outside a
demo.
