# Overriding Image Configuration Files

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Overriding Image Configuration Files](#overriding-image-configuration-files)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Pre-requisites](#pre-requisites)
  - [Overriding a Config File Using Volumes and ConfigMaps](#overriding-a-config-file-using-volumes-and-configmaps)

<!-- mdformat-toc end -->

## Overview

Configuration of Slinky Helm charts is done via the `values.yaml` file, present
within `/helm/chart-name/`. However, in some environments it may be necessary to
override the values of a file that is not controlled by the Helm chart, and is
instead present in the [Slinky images]. This can be done using a [Volume], a
VolumeMount, and a [ConfigMap].

## Pre-requisites

This guide assumes that the user has access to a functional Kubernetes cluster
running slurm-operator. See the [quickstart guide] for details on setting up
slurm-operator on a Kubernetes cluster.

## Overriding a Config File Using Volumes and ConfigMaps

One of the few configuration files that is provided in the [Slinky images] that
is not configurable by default via the Helm charts is `enroot.conf`. As such,
this guide will use `enroot.conf` as an example of how a file can be overridden
using a [Volume] and [ConfigMap].

First, a ConfigMap will be needed that contains the contents of `enroot.conf`
that will be used to override the file already present in the container. Here is
an example of such a ConfigMap, named `enroot-config.yaml` for the purposes of
this demonstration:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: enroot-config
  namespace: slurm
data:
  enroot: |
    ENROOT_RUNTIME_PATH         /run/enroot/${UID}/run
    ENROOT_CONFIG_PATH          /run/enroot/${UID}/config
    ENROOT_CACHE_PATH           /run/enroot/${UID}/cache
    ENROOT_DATA_PATH            /run/enroot/${UID}/data
    ENROOT_TEMP_PATH            /run/${UID}/tmp
```

Apply this ConfigMap to your cluster:

```bash
kubectl apply -f enroot-config.yaml
```

After creating the ConfigMap, a [Volume] must be used to override the default
contents of `/etc/enroot/enroot.conf`. This is to be done on a per-NodeSet
basis, using the volumes and volume mount variables in `helm/slurm/values.yaml`:

```yaml
nodesets:
  slinky:
    ...
    # -- List of volumes to use.
    # Ref: https://kubernetes.io/docs/concepts/storage/volumes/
    volumes: []
      # - name: nfs-home
      #   nfs:
      #     server: nfs-server.example.com
    #     path: /exports/home
    # -- List of volume mounts to use.
    # Ref: https://kubernetes.io/docs/concepts/storage/volumes/
    volumeMounts: []
      # - name: nfs-home
      #   mountPath: /home
```

Modify this section of the NodeSet spec to refer to the ConfigMap that was
created above:

```yaml
nodesets:
  slinky:
    ...
    # -- List of volumes to use.
    # Ref: https://kubernetes.io/docs/concepts/storage/volumes/
    volumes:
    - name: enroot-config
      configMap:
        name: enroot-config
        items:
          - key: enroot
            path: "enroot.conf"

    # -- List of volume mounts to use.
    # Ref: https://kubernetes.io/docs/concepts/storage/volumes/
    volumeMounts:
    - name: enroot-config
      mountPath: "/etc/enroot/enroot.conf"
      subPath: "enroot.conf"
```

At this point, the Helm chart may be installed. The NodeSet that was modified
should show the NodeSet Spec as containing the Volumes and VolumeMounts that
were specified above:

```yaml
kubectl describe NodeSet -n slurm
Name:         slurm-compute-slinky
Namespace:    slurm
...
Spec:
  ...
  Template:
    Container:
      ...
      Volume Mounts:
        Mount Path:  /etc/enroot/enroot.conf
        Name:        enroot-config
        Sub Path:    enroot.conf
    ...
    Volumes:
      Config Map:
        Items:
          Key:   enroot
          Path:  enroot.conf
        Name:    enroot-config
      Name:      enroot-config

```

Within the `slurmd` container of the `slurm-compute-` pod, the file at the
specified Mount Path should be replaced with the contents of the ConfigMap
above:

```bash
kubectl exec -it -n slurm slurm-compute-slinky-0 -- cat /etc/enroot/enroot.conf
ENROOT_RUNTIME_PATH         /run/enroot/${UID}/run
ENROOT_CONFIG_PATH          /run/enroot/${UID}/config
ENROOT_CACHE_PATH           /run/enroot/${UID}/cache
ENROOT_DATA_PATH            /run/enroot/${UID}/data
ENROOT_TEMP_PATH            /run/${UID}/tmp
```

The rest of the files in the `/etc/enroot` directory should remain as-is:

```bash
kubectl exec -it -n slurm slurm-compute-slinky-0 -- ls /etc/enroot
enroot.conf  enroot.conf.d  environ.d  hooks.d	mounts.d
```

If the objective is to override the entire contents of `/etc/enroot` with custom
configurations, that can also be done with this approach. The `SubPath`
directive on the Volume Mount would need to be removed, the filename
`enroot.conf` would need to be removed from the Volume Mount's mount path, and
the Volume would need to have an item added for each file that will be derived
from the ConfigMap.

<!-- Links -->

[configmap]: https://kubernetes.io/docs/concepts/configuration/configmap/
[quickstart guide]: ../installation.md
[slinky images]: https://github.com/SlinkyProject/containers/tree/main
[volume]: https://kubernetes.io/docs/concepts/storage/volumes/
