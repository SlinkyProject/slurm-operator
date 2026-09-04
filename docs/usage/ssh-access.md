# SSH Access

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [SSH Access](#ssh-access)
  - [Table of Contents](#table-of-contents)
  - [Generated sshd_config](#generated-sshd_config)
  - [Client Keepalives](#client-keepalives)
  - [Overriding Operator Settings](#overriding-operator-settings)

<!-- mdformat-toc end -->

## Generated sshd_config

The operator generates a complete `sshd_config` and mounts it read-only over
`/etc/ssh/sshd_config`, so the image's own file and any setting it carried do
not apply. Login pods always get it; worker pods get it when `spec.ssh.enabled`
is set, which is how [pam_slurm_adopt][pam-slurm-adopt] confines a session to
the job that owns the node.

`extraSshdConfig` is appended to the end of that file. It exists on a LoginSet,
and under `spec.ssh` on a NodeSet:

```yaml
loginsets:
  slinky:
    extraSshdConfig: |
      ClientAliveInterval 60
      ClientAliveCountMax 3
```

## Client Keepalives

Nothing in the generated file sets `ClientAliveInterval`, so it keeps OpenSSH's
default of `0` and `sshd` never probes its clients. That leaves a long-lived
session exposed at both ends:

- Idle sessions are dropped by whatever sits in front of the pod. The chart
  exposes login pods through a `LoadBalancer` service, and cloud load balancers
  close idle flows on their own schedule — 350 seconds on AWS NLB, 600 on GCP, 4
  minutes by default on Azure. `TCPKeepAlive` does not help, since Linux waits
  7200 seconds before its first probe.
- A client that goes away without closing leaves `sshd` waiting indefinitely. On
  a worker pod, that session also holds the Slurm job it was adopted into.

The settings above address both: the probes travel inside the encrypted
connection, so the load balancer sees traffic and keeps the flow open, and an
unresponsive client is disconnected after `ClientAliveCountMax` missed probes.
Keep the interval below the idle timeout of the load balancer in front of the
login service — 60 seconds clears the shortest of the timeouts above — and note
that a shorter interval reclaims dead sessions sooner but disconnects clients on
briefly flaky links.

## Overriding Operator Settings

`sshd_config` uses the **first** value obtained for each keyword, and
`extraSshdConfig` is appended last. A keyword the operator already emits
therefore cannot be changed there; the added line is parsed, found to be a
repeat, and ignored. Those keywords are `Include`, `UsePAM` and `Subsystem` on
both pod types, plus `X11Forwarding` on login pods and `AuthenticationMethods`
on worker pods. Anything else, `ClientAliveInterval` included, is undefined
until you set it.

To change one of them, mount a drop-in such as
`/etc/ssh/sshd_config.d/10-local.conf` through the pod template. The `Include`
directive is processed before the rest of the file, so a value set there wins
over both the operator's settings and `extraSshdConfig`.

<!-- Links -->

[pam-slurm-adopt]: https://slurm.schedmd.com/pam_slurm_adopt.html
