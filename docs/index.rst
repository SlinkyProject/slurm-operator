Kubernetes Operator for Slurm Clusters
======================================

.. container::

   |License| |Tag| |Go-Version| |Last-Commit|

Run `Slurm <https://slurm.schedmd.com/overview.html>`__ on
`Kubernetes <https://kubernetes.io/>`__, by
`SchedMD <https://schedmd.com/>`__. A `Slinky <https://slinky.ai/>`__
project.

Table of Contents
-----------------

.. raw:: html

   <!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- `Kubernetes Operator for Slurm
  Clusters <#kubernetes-operator-for-slurm-clusters>`__

  - `Table of Contents <#table-of-contents>`__
  - `Overview <#overview>`__

    - `Slurm Cluster <#slurm-cluster>`__

  - `Features <#features>`__

    - `Controller <#controller>`__
    - `NodeSets <#nodesets>`__
    - `LoginSets <#loginsets>`__
    - `Hybrid Support <#hybrid-support>`__
    - `Slurm <#slurm>`__

  - `Compatibility <#compatibility>`__
  - `Quick Start <#quick-start>`__
  - `Upgrades <#upgrades>`__

    - `1.Y Releases <#1y-releases>`__
    - `0.Y Releases <#0y-releases>`__

  - `Documentation <#documentation>`__
  - `Support and Development <#support-and-development>`__
  - `License <#license>`__

.. raw:: html

   <!-- mdformat-toc end -->

Overview
--------

`Slurm <https://slurm.schedmd.com/overview.html>`__ and
`Kubernetes <https://kubernetes.io/>`__ are workload managers originally
designed for different kinds of workloads. In broad strokes: Kubernetes
excels at scheduling workloads that typically run for an indefinite
amount of time, with potentially vague resource requirements, on a
single node, with loose policy, but can scale its resource pool
infinitely to meet demand; Slurm excels at quickly scheduling workloads
that run for a finite amount of time, with well defined resource
requirements and topology, on multiple nodes, with strict policy, but
its resource pool is known.

This project enables the best of both workload managers, unified on
Kubernetes. It contains a `Kubernetes <https://kubernetes.io/>`__
operator to deploy and manage certain components of
`Slurm <https://slurm.schedmd.com/overview.html>`__ clusters. This
repository implements
`custom-controllers <https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-controllers>`__
and `custom resource definitions
(CRDs) <https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions>`__
designed for the lifecycle (creation, upgrade, graceful shutdown) of
Slurm clusters.

.. figure:: _static/images/architecture-operator.svg
   :alt: “Slurm Operator Architecture”

   “Slurm Operator Architecture”

For additional architectural notes, see the
`architecture <concepts/architecture.html>`__ docs.

Slurm Cluster
~~~~~~~~~~~~~

Slurm clusters are very flexible and can be configured in various ways.
Our Slurm helm chart provides a reference implementation that is highly
customizable and tries to expose everything Slurm has to offer.

.. figure:: _static/images/architecture-slurm.svg
   :alt: “Slurm Architecture”

   “Slurm Architecture”

For additional information about Slurm, see the
`slurm <concepts/slurm.html>`__ docs.

Features
--------

Controller
~~~~~~~~~~

The Slurm control-plane is responsible for scheduling Slurm workload
onto its worker nodes and managing their states.

Changes to the Slurm configuration files are automatically detected and
the Slurm cluster is reconfigured seamlessly with zero downtime of the
Slurm control-plane.

   [!NOTE] The kubelet’s ``configMapAndSecretChangeDetectionStrategy``
   and ``syncFrequency`` settings directly affect when pods have their
   mounted ConfigMaps and Secrets updated. By default, the kubelet is in
   ``Watch`` mode with a polling frequency of 60 seconds.

NodeSets
~~~~~~~~

A set of homogeneous Slurm workers (compute nodes), which are delegated
to execute the Slurm workload.

The operator will take into consideration the running workload among
Slurm nodes as it needs to scale-in, upgrade, or otherwise handle node
failures. Slurm nodes will be marked as
`drain <https://slurm.schedmd.com/scontrol.html#OPT_DRAIN>`__ before
their eventual termination pending scale-in or upgrade.

Slurm node states (e.g. Idle, Allocated, Mixed, Down, Drain, Not
Responding, etc…) are applied to each NodeSet pod via their pod
conditions; each NodeSet pod contains a pod status that reflects their
own Slurm node state.

The operator supports NodeSet scale to zero, scaling the resource down
to zero replicas. Hence, any Horizontal Pod Autoscaler (HPA) that also
support scale to zero can be best paired with NodeSets.

NodeSets can be resolved by hostname. This enables hostname-based
resolution between login pods and worker pods, enabling direct
pod-to-pod communication using predictable hostnames (e.g., ``cpu-1-0``,
``gpu-2-1``).

LoginSets
~~~~~~~~~

A set of homogeneous login nodes (submit node, jump host) for Slurm,
which manage user identity via SSSD.

The operator supports LoginSet scale to zero, scaling the resource down
to zero replicas. Hence, any Horizontal Pod Autoscaler (HPA) that also
support scale to zero can be best paired with LoginSets.

Hybrid Support
~~~~~~~~~~~~~~

Sometimes a Slurm cluster has some, but not all, of its components in
Kubernetes. The operator and its CRDs are designed support these use
cases.

Slurm
~~~~~

Slurm is a full featured HPC workload manager. To highlight a few
features:

- `Accounting <https://slurm.schedmd.com/accounting.html>`__: collect
  accounting information for every job and job step executed.
- `Partitions <https://slurm.schedmd.com/quickstart.html#arch>`__: job
  queues with sets of resources and constraints (e.g. job size limit,
  job time limit, users permitted).
- `Reservations <https://slurm.schedmd.com/reservations.html>`__:
  reserve resources for jobs being executed by select users and/or
  select accounts.
- `Job
  Dependencies <https://slurm.schedmd.com/sbatch.html#OPT_dependency>`__:
  defer the start of jobs until the specified dependencies have been
  satisfied.
- `Job Containers <https://slurm.schedmd.com/containers.html>`__: jobs
  which run an unprivileged OCI container bundle.
- `MPI <https://slurm.schedmd.com/mpi_guide.html>`__: launch parallel
  MPI jobs, supports various MPI implementations.
- `Priority <https://slurm.schedmd.com/priority_multifactor.html>`__:
  assigns priorities to jobs upon submission and on an ongoing basis
  (e.g. as they age).
- `Preemption <https://slurm.schedmd.com/preempt.html>`__: stop one or
  more low-priority jobs to let a high-priority job run.
- `QoS <https://slurm.schedmd.com/qos.html>`__: sets of policies
  affecting scheduling priority, preemption, and resource limits.
- `Fairshare <https://slurm.schedmd.com/fair_tree.html>`__: distribute
  resources equitably among users and accounts based on historical
  usage.
- `Node Health
  Check <https://slurm.schedmd.com/slurm.conf.html#OPT_HealthCheckProgram>`__:
  periodically check node health via script.

Compatibility
-------------

+------------+-----------------------------------------------------------------------------+
| Software   | Minimum Version                                                             |
+============+=============================================================================+
| Kubernetes | `v1.29 <https://kubernetes.io/blog/2023/12/13/kubernetes-v1-29-release/>`__ |
+------------+-----------------------------------------------------------------------------+
| Slurm      | `25.11 <https://www.schedmd.com/slurm-version-25-11-0-is-now-available/>`__ |
+------------+-----------------------------------------------------------------------------+
| Cgroup     | `v2 <https://docs.kernel.org/admin-guide/cgroup-v2.html>`__                 |
+------------+-----------------------------------------------------------------------------+

Quick Start
-----------

Install the
`cert-manager <https://cert-manager.io/docs/installation/helm/>`__ with
its CRDs:

.. code:: sh

   helm repo add jetstack https://charts.jetstack.io
   helm repo update
   helm install cert-manager jetstack/cert-manager \
     --set 'crds.enabled=true' \
     --namespace cert-manager --create-namespace

Install the slurm-operator and its CRDs:

.. code:: sh

   helm install slurm-operator-crds oci://ghcr.io/slinkyproject/charts/slurm-operator-crds
   helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
     --namespace=slinky --create-namespace

Install a Slurm cluster:

.. code:: sh

   helm install slurm oci://ghcr.io/slinkyproject/charts/slurm \
     --namespace=slurm --create-namespace

For additional instructions, see the
`installation <installation.html>`__ guide.

Upgrades
--------

Slinky versions are expressed as **X.Y.Z**, where **X** is the major
version, **Y** is the minor version, and **Z** is the patch version,
following `Semantic Versioning <https://semver.org/>`__ terminology.

See `versioning <versioning.html>`__ for more details.

1.Y Releases
~~~~~~~~~~~~

Breaking changes may be introduced into newer
`CRDs <https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions>`__.
To upgrade between ``v1.Y`` versions (e.g. ``v1.0.Z`` => ``v1.1.Z``),
upgrade the slurm-operator-crds chart followed by the slurm-operator
chart. Any Slurm charts will automatically be handled via CRD
conversion; no further action is required. It is still recommended to
upgrade Slurm charts to make use of the new features and functionality.

.. code:: bash

   helm upgrade slurm-operator-crds oci://ghcr.io/slinkyproject/charts/slurm-operator-crds
   helm upgrade slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
     --namespace=slinky

To make use of new Slinky CRD features, please review changes made to
the CRDs and the Slurm chart. Update your values.yaml appropriately and
upgrade the chart.

.. code:: sh

   helm upgrade slurm oci://ghcr.io/slinkyproject/charts/slurm \
     --namespace=slurm

.. _y-releases-1:

0.Y Releases
~~~~~~~~~~~~

Breaking changes may be introduced into existing
`CRDs <https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#customresourcedefinitions>`__.
To upgrade between ``v0.Y`` versions (e.g. ``v0.1.Z`` => ``v0.2.Z``),
uninstall all Slinky charts and delete Slinky CRDs, then install the new
release like normal.

.. code:: bash

   helm --namespace=slurm uninstall slurm
   helm --namespace=slinky uninstall slurm-operator
   helm uninstall slurm-operator-crds

If the CRDs were not installed via ``slurm-operator-crds`` helm chart:

.. code:: bash

   kubectl delete customresourcedefinitions.apiextensions.k8s.io accountings.slinky.slurm.net
   kubectl delete customresourcedefinitions.apiextensions.k8s.io clusters.slinky.slurm.net # defunct
   kubectl delete customresourcedefinitions.apiextensions.k8s.io loginsets.slinky.slurm.net
   kubectl delete customresourcedefinitions.apiextensions.k8s.io nodesets.slinky.slurm.net
   kubectl delete customresourcedefinitions.apiextensions.k8s.io restapis.slinky.slurm.net
   kubectl delete customresourcedefinitions.apiextensions.k8s.io tokens.slinky.slurm.net

Documentation
-------------

Project documentation is located in the docs directory of this
repository.

Slinky documentation can be found
`here <https://slinky.schedmd.com/>`__.

Support and Development
-----------------------

Feature requests, code contributions, and bug reports are welcome!

Github/Gitlab submitted issues and PRs/MRs are handled on a best effort
basis.

The SchedMD official issue tracker is at https://support.schedmd.com/.

To schedule a demo or simply to reach out, please `contact
SchedMD <https://www.schedmd.com/slurm-resources/contact-schedmd/>`__.

License
-------

Copyright (C) SchedMD LLC.

Licensed under the `Apache License, Version
2.0 <http://www.apache.org/licenses/LICENSE-2.0>`__ you may not use
project except in compliance with the license.

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an “AS IS” BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

.. raw:: html

   <!-- links -->

.. |License| image:: https://img.shields.io/badge/License-Apache_2.0-blue.svg?style=for-the-badge
   :target: ./LICENSES/Apache-2.0.txt
.. |Tag| image:: https://img.shields.io/github/v/tag/togethercomputer/slurm-operator?style=for-the-badge
   :target: https://github.com/togethercomputer/slurm-operator/tags/
.. |Go-Version| image:: https://img.shields.io/github/go-mod/go-version/togethercomputer/slurm-operator?style=for-the-badge
   :target: ./go.mod
.. |Last-Commit| image:: https://img.shields.io/github/last-commit/togethercomputer/slurm-operator?style=for-the-badge
   :target: https://github.com/togethercomputer/slurm-operator/commits/

.. toctree::
    :maxdepth: 2
    :glob:
    :hidden:
    :caption: Getting started

    installation.md

.. toctree::
    :maxdepth: 2
    :glob:
    :hidden:
    :caption: Versioning

    versioning.md

.. toctree::
    :maxdepth: 2
    :glob:
    :hidden:
    :caption: Usage & Configuration

    usage/*

.. toctree::
    :maxdepth: 2
    :glob:
    :hidden:
    :caption: Concepts & Design

    concepts/*
