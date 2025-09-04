# NVIDIA DCGM Integration for Slurm

This integration provides GPU-to-job mapping for NVIDIA DCGM Exporter, enabling GPU metrics to be labeled with Slurm job IDs.

## Configuration

Add to your `values.yaml`:

```yaml
vendor:
  nvidia:
    dcgm:
      enabled: true
      jobMappingDir: "/var/lib/dcgm-exporter/job-mapping"
      scriptPriority: "90"
```

## How It Works

1. **Auto-detects** nodesets with `nvidia.com/gpu` resource limits
2. **Adds prolog/epilog scripts** that create/cleanup GPU job mapping files
3. **Mounts volume** at `/var/lib/dcgm-exporter/job-mapping` for DCGM exporter access

## Testing

Submit a GPU job and verify mapping files:

```bash
# Submit job
sbatch --gres=gpu:2 --wrap="nvidia-smi; sleep 60"

# Check mapping files (inside slurmd pod)
ls /var/lib/dcgm-exporter/job-mapping/
# Should show files like: 0, 1 (containing job ID)
```
