# Autoscaling

The slurm-operator may be configured to autoscale NodeSets pods based on Slurm
metrics. This guide discusses how to configure autoscaling using [KEDA].

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [Autoscaling](#autoscaling)
  - [Table of Contents](#table-of-contents)
  - [Getting Started](#getting-started)
    - [Dependencies](#dependencies)
      - [Verify KEDA Metrics API Server is running](#verify-keda-metrics-api-server-is-running)
  - [Autoscaling](#autoscaling-1)
    - [NodeSet Scale Subresource](#nodeset-scale-subresource)
    - [KEDA ScaledObject](#keda-scaledobject)

<!-- mdformat-toc end -->

## Getting Started

Before attempting to autoscale NodeSets, Slinky should be fully deployed to a
Kubernetes cluster and Slurm jobs should be able to run.

### Dependencies

Autoscaling requires additional services that are not included in Slinky. Follow
documentation to install [Prometheus], [Metrics Server], and [KEDA].

Prometheus will install tools to report metrics and view them with Grafana. The
Metrics Server is needed to report CPU and memory usage for tools like
`kubectl top`. KEDA is recommended for autoscaling as it provides usability
improvements over standard the Horizontal Pod Autoscaler ([HPA]).

To add KEDA in the helm install, run

```sh
helm repo add kedacore https://kedacore.github.io/charts
```

Install the [slurm-exporter]. This chart is installed as a dependency of the
slurm helm chart by default. Configure using helm/slurm/values.yaml.

#### Verify KEDA Metrics API Server is running

```sh
$ kubectl get apiservice -l app.kubernetes.io/instance=keda
NAME                              SERVICE                                AVAILABLE   AGE
v1beta1.external.metrics.k8s.io   keda/keda-operator-metrics-apiserver   True        22h
```

[KEDA] provides the metrics apiserver required by HPA to scale on custom metrics
from Slurm. An alternative like [Prometheus Adapter] could be used for this, but
KEDA offers usability enhancements and improvements to HPA in addition to
including a metrics apiserver.

## Autoscaling

Autoscaling NodeSets allows Slurm partitions to expand and contract in response
to the CPU and memory usage. Using Slurm metrics, NodeSets may also scale based
on Slurm specific information like the number of pending jobs or the size of the
largest pending job in a partition. There are many ways to configure
autoscaling. Experiment with different combinations based on the types of jobs
being run and the resources available in the cluster.

### NodeSet Scale Subresource

Scaling a resource in Kubernetes requires that resources such as Deployments and
StatefulSets support the [scale subresource]. This is also true of the NodeSet
Custom Resource.

The scale subresource gives a standard interface to observe and control the
number of replicas of a resource. In the case of NodeSet, it allows Kubernetes
and related services to control the number of `slurmd` replicas running as part
of the NodeSet.

To manually scale a NodeSet, use the `kubectl scale` command. In this example,
the NodeSet (nss) `slurm-worker-radar` is scaled to 1.

```sh
$ kubectl scale -n slurm nss/slurm-worker-radar --replicas=1
nodeset.slinky.slurm.net/slurm-worker-radar scaled

$ kubectl get pods -o wide -n slurm -l app.kubernetes.io/instance=slurm-worker-radar
NAME                   READY   STATUS    RESTARTS   AGE     IP            NODE          NOMINATED NODE   READINESS GATES
slurm-worker-radar-0   1/1     Running   0          2m48s   10.244.4.17   kind-worker   <none>           <none>
```

This corresponds to the Slurm partition `radar`.

```sh
$ kubectl exec -n slurm statefulset/slurm-controller -- sinfo
PARTITION AVAIL  TIMELIMIT  NODES  STATE NODELIST
radar        up   infinite      1   idle kind-worker
```

NodeSets may be scaled to zero. In this case, there are no replicas of `slurmd`
running and all jobs scheduled to that partition will remain in a pending state.

```sh
$ kubectl scale nss/slurm-worker-radar -n slurm --replicas=0
nodeset.slinky.slurm.net/slurm-worker-radar scaled
```

For NodeSets to scale on demand, an autoscaler needs to be deployed. KEDA allows
resources to scale from 0\<->1 and also creates an HPA to scale based on scalers
like Prometheus and more.

### KEDA ScaledObject

KEDA uses the Custom Resource [ScaledObject] to monitor and scale a resource. It
will automatically create the HPA needed to scale based on external triggers
like Prometheus. With Slurm metrics, NodeSets may be scaled based on data
collected from the Slurm restapi.

This example [ScaledObject] will watch the number of jobs pending for the
partition `radar` and scale the NodeSet `slurm-worker-radar` until a threshold
value is satisfied or `maxReplicaCount` is reached.

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: scale-radar
spec:
  scaleTargetRef:
    apiVersion: slinky.slurm.net/v1beta1
    kind: NodeSet
    name: slurm-worker-radar
  idleReplicaCount: 0
  minReplicaCount: 1
  maxReplicaCount: 3
  triggers:
    - type: prometheus
      metricType: Value
      metadata:
        serverAddress: http://prometheus-kube-prometheus-prometheus.prometheus:9090
        query: slurm_partition_pending_jobs{partition="radar"}
        threshold: '5'
```

**Note**: The Prometheus trigger is using `metricType: Value` instead of the
default `AverageValue`. `AverageValue` calculates the replica count by averaging
the threshold across the current replica count.

Check [ScaledObject] documentation for a full list of allowable options.

In this scenario, the ScaledObject `scale-radar` will query the Slurm metric
`slurm_partition_pending_jobs` from Prometheus with the label
`partition="radar"`.

When there is activity on the trigger (at least one pending job), KEDA will
scale the NodeSet to `minReplicaCount` and then let HPA handle scaling up to
`maxReplicaCount` or back down to `minReplicaCount`. When there is no activity
on the trigger after a configurable amount of time, KEDA will scale the NodeSet
to `idleReplicaCount`. See the [KEDA] documentation on [idleReplicaCount] for
more examples.

**Note**: The only supported value for `idleReplicaCount` is 0 due to
limitations on how the HPA controller works.

To verify a KEDA ScaledObject, apply it to the cluster in the appropriate
namespace on a NodeSet that has no replicas.

```sh
$ kubectl scale nss/slurm-worker-radar -n slurm --replicas=0
nodeset.slinky.slurm.net/slurm-worker-radar scaled
```

Wait for Slurm to report that the partition has no nodes.

```sh
$ slurm@slurm-controller-0:/tmp$ sinfo -p radar
PARTITION AVAIL  TIMELIMIT  NODES  STATE NODELIST
radar        up   infinite      0    n/a
```

Apply the ScaledObject using `kubectl` to the correct namespace and verify the
KEDA and HPA resources are created.

```sh
$ kubectl apply -f scaledobject.yaml -n slurm
scaledobject.keda.sh/scale-radar created

$ kubectl get -n slurm scaledobjects
NAME           SCALETARGETKIND                     SCALETARGETNAME        MIN   MAX   TRIGGERS     AUTHENTICATION   READY   ACTIVE   FALLBACK   PAUSED    AGE
scale-radar    slinky.slurm.net/v1alpha1.NodeSet   slurm-worker-radar    1     5     prometheus                    True    False    Unknown    Unknown   28s

$ kubectl get -n slurm hpa
NAME                    REFERENCE                      TARGETS       MINPODS   MAXPODS   REPLICAS   AGE
keda-hpa-scale-radar    NodeSet/slurm-worker-radar    <unknown>/5   1         5         0          32s
```

Once the [ScaledObject] and HPA are created, initiate some jobs to test that the
`NodeSet` scale subresource is scaled in response.

```sh
$ sbatch --wrap "sleep 30" --partition radar --exclusive
```

The NodeSet will scale to `minReplicaCount` in response to activity on the
trigger. Once the number of pending jobs crosses the configured `threshold`
(submit more exclusive jobs to the partition), more replicas will be created to
handle the additional demand. Until the `threshold` is exceeded, the NodeSet
will remain at `minReplicaCount`.

**Note**: This example only works well for single node jobs, unless `threshold`
is set to 1. In this case, HPA will continue to scale up NodeSet as long as
there is a pending job until up until it reaches the `maxReplicaCount`.

After the default `coolDownPeriod` of 5 minutes without activity on the trigger,
KEDA will scale the NodeSet down to 0.

<!-- Links -->

[hpa]: https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/
[idlereplicacount]: https://keda.sh/docs/concepts/scaling-deployments/#idlereplicacount
[keda]: https://keda.sh/docs/
[metrics server]: https://github.com/kubernetes-sigs/metrics-server
[prometheus]: https://prometheus-operator.dev/docs/getting-started/introduction/
[prometheus adapter]: https://github.com/kubernetes-sigs/prometheus-adapter
[scale subresource]: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#scale-subresource
[scaledobject]: https://keda.sh/docs/concepts/scaling-deployments/
[slurm-exporter]: https://github.com/SlinkyProject/slurm-exporter
