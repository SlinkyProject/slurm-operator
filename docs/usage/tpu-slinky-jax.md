# GKE with Cloud TPUs and Slurm-operator on JAX

This tutorial guides you through setting up a Slurm cluster on Google Kubernetes
Engine (GKE) with Cloud TPU (v5e) support using **Slurm-operator**. You will
deploy the operator, configure TPU GRES support with necessary GKE workarounds,
and run a JAX model training test job.

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [GKE with Cloud TPUs and Slurm-operator on JAX](#gke-with-cloud-tpus-and-slurm-operator-on-jax)
  - [Table of Contents](#table-of-contents)
    - [1. Plan & Create GKE Infrastructure](#1-plan--create-gke-infrastructure)
    - [2. Install Operator Prerequisites](#2-install-operator-prerequisites)
      - [2.1. Install Cert-Manager](#21-install-cert-manager)
    - [3. Deploy Slurm-operator](#3-deploy-slurm-operator)
    - [4. Configure TPU Support in Slurm](#4-configure-tpu-support-in-slurm)
    - [5. Deploy the Slurm Cluster](#5-deploy-the-slurm-cluster)
      - [Verify GRES Registration](#verify-gres-registration)
    - [6. Running a JAX TPU Workload](#6-running-a-jax-tpu-workload)
      - [6.1. Create the JAX Test Script (`jax_test.py`)](#61-create-the-jax-test-script-jax_testpy)
      - [6.2. Create the Slurm Submission Script (`submit_jax.sh`)](#62-create-the-slurm-submission-script-submit_jaxsh)
      - [6.3. Copy and Submit the Job](#63-copy-and-submit-the-job)
      - [6.4. Verify Results](#64-verify-results)
    - [7. Conclusion](#7-conclusion)

<!-- mdformat-toc end -->

______________________________________________________________________

### 1. Plan & Create GKE Infrastructure

To get started, you need a GKE cluster (Standard) and a TPU v5e node pool. In
this tutorial, we will target a single-host **8-chip TPU v5e** node pool.

Set up your environment variables and create the cluster:

```bash
PROJECT="<YOUR_GCP_PROJECT_ID>"
CLUSTER="tpu-slurm-cluster"
ZONE="us-central1-a"
VERSION="1.35"

# Create the base GKE Standard cluster
gcloud container clusters create "${CLUSTER}" \
    --cluster-version="${VERSION}" \
    --zone="${ZONE}" \
    --project="${PROJECT}" \
    --num-nodes="3"

# Create the TPU v5e Spot node pool (8-chip ct5lp-hightpu-8t)
gcloud container node-pools create tpu-v5-single-host-spot \
    --cluster="${CLUSTER}" \
    --zone="${ZONE}" \
    --project="${PROJECT}" \
    --machine-type="ct5lp-hightpu-8t" \
    --spot \
    --num-nodes="1" \
    --node-locations="${ZONE}"
```

______________________________________________________________________

### 2. Install Operator Prerequisites

Slinky requires `cert-manager` to manage admission webhook certificates.

#### 2.1. Install Cert-Manager

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
helm install cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.14.4 \
  --set installCRDs=true
```

______________________________________________________________________

### 3. Deploy Slurm-operator

We will use the latest stable release of Slurm-operator. A list of releases for
Slurm-operator can be found
[here](https://github.com/SlinkyProject/slurm-operator/tags).

1. Install the operator with CRDs enabled:

   ```bash
   helm install slurm-operator oci://ghcr.io/slinkyproject/charts/slurm-operator \
     --set crds.enabled=true \
     -n slinky \
     --create-namespace
   ```

Verify both operator pods are running successfully:

```bash
kubectl get pods -n slinky
```

______________________________________________________________________

### 4. Configure TPU Support in Slurm

We must prepare a custom `tpu-values.yaml` for the Slurm cluster deployment.
Because GKE and Slurm-operator have specific requirements, we must apply three
critical workarounds:

1. **GKE Warden Topology Alignment**: GKE Warden validating webhook requires
   Pods requesting TPUs to have exact topology selectors matching the node pool.
   We specify `tpu-v5-lite-podslice` and `2x4` (8 chips).
1. **Manual host `/dev` mount**: GKE TPU v5e uses the `vfio-pci` driver (devices
   exposed via `/dev/vfio/*`). Since the GKE webhook might skip automatic device
   mounting for custom slurmd pods, we manually mount `/dev` from the host.
1. **Count-Based GRES**: Because traditional `/dev/accel*` files do not exist on
   VFIO-based TPU v5e nodes, listing file paths in `gres.conf` will cause
   `slurmd` startup crashes. We configure Slurm GRES by **Count only**
   (`Name=tpu Count=8`), which is safe since device constraints are not enforced
   by cgroups by default.

Save the following as `tpu-values.yaml` in your workspace root:

```yaml
# tpu-values.yaml
clusterName: slurm-tpu-cluster

controller:
  # Add TPU to general GRES types
  extraConf: |
    GresTypes=gpu,tpu

configFiles:
  # Use Count-based GRES (no File paths) to avoid slurmd stat crashes.
  gres.conf: |
    Name=tpu Count=8

nodesets:
  tpu:
    enabled: true
    scalingMode: StatefulSet
    replicas: 1  # Deploy 1 worker pod on our single 8-chip node

    slurmd:
      image:
        repository: ghcr.io/slinkyproject/slurmd
        tag: 25.11-ubuntu24.04
      resources:
        limits:
          google.com/tpu: 8  # Request all 8 chips on the node
        requests:
          google.com/tpu: 8
      # Mount host /dev to access /dev/vfio/* TPU devices
      volumeMounts:
        - name: host-dev
          mountPath: /dev

    logfile:
      image:
        repository: docker.io/library/alpine
        tag: latest

    extraConfMap:
      Gres: ["tpu:8"]

    podSpec:
      # GKE Warden Topology Requirements
      nodeSelector:
        cloud.google.com/gke-tpu-accelerator: "tpu-v5-lite-podslice"
        cloud.google.com/gke-tpu-topology: "2x4"
      tolerations:
        - key: "google.com/tpu"
          operator: "Exists"
          effect: "NoSchedule"
      volumes:
        - name: host-dev
          hostPath:
            path: /dev
            type: Directory

partitions:
  tpu:
    enabled: true
    nodesets:
      - tpu
    configMap:
      Default: "YES"
      MaxTime: "UNLIMITED"
      State: "UP"
```

______________________________________________________________________

### 5. Deploy the Slurm Cluster

Now, deploy the Slurm cluster using Helm:

```bash
helm install slurm-tpu oci://ghcr.io/slinkyproject/charts/slurm -f tpu-values.yaml -n slurm --create-namespace
```

#### Verify GRES Registration

Once all pods in the `slurm` namespace are `Running`, check if Slurmctld has
successfully registered the 8 TPUs:

```bash
kubectl exec -n slurm slurm-tpu-controller-0 -c slurmctld -- sinfo -o "%n %G"
```

Expected output:

```
HOSTNAMES GRES
slinky-0 (null)
tpu-0 tpu:8
```

The TPU node `tpu-0` successfully registers `tpu:8`!

______________________________________________________________________

### 6. Running a JAX TPU Workload

> [!NOTE]
> For a proof-of-concept, this tutorial installs Python system packages and JAX
> dynamically at runtime inside the job script. For a production environment, it
> is highly recommended to bake JAX and its dependencies directly into the
> `slurmd` container image at build-time to reduce startup latency, improve
> reliability, and ensure network isolation. This can be done similarly to the
> [Nvidia/PyTorch tutorial](./tutorial-pytorch.md) by extending Slinky's base
> Dockerfile.

Since the default Slinky `slurmd` image is a minimal Ubuntu image without Python
packages, we will write a submission script that installs `python3-pip` and
`python3-venv` inside the container at runtime (made possible because Slurm jobs
run as `root` inside Slurm-operator by default).

#### 6.1. Create the JAX Test Script (`jax_test.py`)

Save the following as `jax_test.py`:

```python
# jax_test.py
# Simple MLP training loop in pure JAX to verify TPU execution.
import jax
import jax.numpy as jnp
import time

print("--- JAX TPU Model Training Verification ---")
print("JAX version:", jax.__version__)
print("Available devices:", jax.devices())

devices = jax.devices()
tpu_found = any(d.device_kind.lower().startswith('tpu') for d in devices)

if tpu_found:
    print("SUCCESS: JAX is using TPU!")
else:
    print("WARNING: JAX is NOT using TPU!")

# --- Simple MLP Training Loop ---

# Generate synthetic data
key = jax.random.PRNGKey(0)
key, x_key, y_key = jax.random.split(key, 3)
X = jax.random.normal(x_key, (1000, 64))  # 1000 samples, 64 features
# Target: simple non-linear function
Y = jnp.sin(X[:, 0:1]) + 0.5 * X[:, 1:2]

# Initialize weights (Simple 2-layer MLP: 64 -> 32 -> 1)
key, w1_key, w2_key = jax.random.split(key, 3)
w1 = jax.random.normal(w1_key, (64, 32)) * 0.1
b1 = jnp.zeros((32,))
w2 = jax.random.normal(w2_key, (32, 1)) * 0.1
b2 = jnp.zeros((1,))
params = {'w1': w1, 'b1': b1, 'w2': w2, 'b2': b2}

# Predict function
def predict(params, x):
    h1 = jax.nn.relu(jnp.dot(x, params['w1']) + params['b1'])
    return jnp.dot(h1, params['w2']) + params['b2']

# MSE Loss
def loss_fn(params, x, y):
    preds = predict(params, x)
    return jnp.mean((preds - y) ** 2)

# Update step (JIT-compiled for speed on TPU)
@jax.jit
def update_step(params, x, y, lr=0.05):
    loss, grads = jax.value_and_grad(loss_fn)(params, x, y)
    # SGD update using tree_map
    new_params = jax.tree_util.tree_map(lambda p, g: p - lr * g, params, grads)
    return new_params, loss

# Training Loop
print("Starting training loop (100 epochs)...")
start_time = time.time()
loss = 0.0
for epoch in range(100):
    params, loss = update_step(params, X, Y)
    if epoch % 10 == 0:
        print(f"Epoch {epoch}: Loss = {loss:.6f}")

duration = time.time() - start_time
print(f"Training completed in {duration:.4f} seconds!")
print(f"Final Loss: {loss:.6f}")
print("-------------------------------------------")
```

#### 6.2. Create the Slurm Submission Script (`submit_jax.sh`)

Save the following as `submit_jax.sh`. Note that it requests `--gres=tpu:8` to
allocate all chips:

```bash
#!/usr/bin/env bash
set -euo pipefail

#SBATCH --job-name=jax-tpu-test
#SBATCH --partition=tpu
#SBATCH --nodes=1
#SBATCH --ntasks-per-node=1
#SBATCH --gres=tpu:8  # Request all 8 TPUs on the node
#SBATCH --output=jax_tpu_%j.out
#SBATCH --error=jax_tpu_%j.err
#SBATCH --time=00:15:00

echo "=== Job started at $(date) ==="
echo "Running on node: $(hostname)"

# Install pip & venv dynamically (requires internet access in container)
echo "Installing system packages (pip and venv)..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -y && apt-get install -y python3-pip python3-venv python3-full

# Create virtualenv and install JAX with TPU support
VENV_DIR="/tmp/jax_tpu_venv"
echo "Creating virtualenv..."
python3 -m venv --system-site-packages "$VENV_DIR"
source "$VENV_DIR"/bin/activate

echo "Installing JAX..."
python3 -m pip install --upgrade pip
python3 -m pip install "jax[tpu]" -f https://storage.googleapis.com/jax-releases/libtpu_releases.html

# Execute the JAX test
echo "Running JAX test script..."
python3 jax_test.py

echo "=== Job finished at $(date) ==="
```

#### 6.3. Copy and Submit the Job

Copy the scripts to the worker pod and submit the job via Slurm:

```bash
# Copy scripts to the worker pod "slurm-tpu-worker-tpu-0"
kubectl cp jax_test.py slurm/slurm-tpu-worker-tpu-0:/tmp/jax_test.py -c slurmd
kubectl cp submit_jax.sh slurm/slurm-tpu-worker-tpu-0:/tmp/submit_jax.sh -c slurmd

# Submit the job
kubectl exec -n slurm slurm-tpu-worker-tpu-0 -c slurmd -- bash -c "cd /tmp && sbatch submit_jax.sh"
```

#### 6.4. Verify Results

Once `squeue` shows the job has completed, check the output log
(`/tmp/jax_tpu_<JOBID>.out`):

```bash
kubectl exec -n slurm slurm-tpu-worker-tpu-0 -c slurmd -- cat /tmp/jax_tpu_1.out
```

It should look like the following, showing the successful JAX environment
bootstrap, detection of the 8 TPU devices, and the loss successfully decreasing
over the 100 training epochs:

```
=== Job started at Thu May 21 18:16:24 UTC 2026 ===
Running on node: tpu-0
Allocated GRES:
...
JAX successfully imported inside job!
Running JAX test script...
--- JAX TPU Model Training Verification ---
JAX version: 0.10.1
Available devices: [TpuDevice(id=0, process_index=0, coords=(0,0,0), core_on_chip=0), TpuDevice(id=1, process_index=0, coords=(1,0,0), core_on_chip=0), TpuDevice(id=2, process_index=0, coords=(0,1,0), core_on_chip=0), TpuDevice(id=3, process_index=0, coords=(1,1,0), core_on_chip=0), TpuDevice(id=4, process_index=0, coords=(0,2,0), core_on_chip=0), TpuDevice(id=5, process_index=0, coords=(1,2,0), core_on_chip=0), TpuDevice(id=6, process_index=0, coords=(0,3,0), core_on_chip=0), TpuDevice(id=7, process_index=0, coords=(1,3,0), core_on_chip=0)]
SUCCESS: JAX is using TPU!
Starting training loop (100 epochs)...
Epoch 0: Loss = 0.693804
Epoch 10: Loss = 0.535265
Epoch 20: Loss = 0.427468
Epoch 30: Loss = 0.329034
Epoch 40: Loss = 0.245817
Epoch 50: Loss = 0.187892
Epoch 60: Loss = 0.152073
Epoch 70: Loss = 0.129508
Epoch 80: Loss = 0.114930
Epoch 90: Loss = 0.104988
Training completed in 0.1254 seconds!
Final Loss: 0.098378
-------------------------------------------
=== Job finished at Thu May 21 18:16:38 UTC 2026 ===
```

This confirms a successful end-to-end TPU Slurm deployment on GKE!

______________________________________________________________________

### 7. Conclusion

This pattern enables you to quickly deploy Slurm clusters on GKE and run
large-scale JAX training jobs leveraging Google Cloud's physical TPU hardware
slices!
