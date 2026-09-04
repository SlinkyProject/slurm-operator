# SSH Access

This guide documents how the operator configures `sshd` for login pods and for
worker pods with SSH enabled, and how to extend that configuration.

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [SSH Access](#ssh-access)
  - [Table of Contents](#table-of-contents)
  - [How sshd_config Is Assembled](#how-sshd_config-is-assembled)
  - [Keeping Long-Lived Sessions Alive](#keeping-long-lived-sessions-alive)
  - [Settings the Operator Already Defines](#settings-the-operator-already-defines)

<!-- mdformat-toc end -->

## How sshd_config Is Assembled

The operator generates a complete `sshd_config` and mounts it read-only over
`/etc/ssh/sshd_config`. The image's own `sshd_config` is therefore not used, and
any setting it carried no longer applies. Only the settings the operator emits
and the ones you supply are in effect.

Login pods always get this file. Worker pods get it when `spec.ssh.enabled` is
set on the NodeSet, which is how
[pam_slurm_adopt](https://slurm.schedmd.com/pam_slurm_adopt.html) confines an
incoming SSH session to the job that owns the node.

Use `extraSshdConfig` to add settings. It is appended to the end of the
generated file:

```yaml
loginsets:
  slinky:
    extraSshdConfig: |
      LoginGraceTime 600
```

The same field exists under `spec.ssh` on a NodeSet:

```yaml
nodesets:
  slinky:
    ssh:
      enabled: true
      extraSshdConfig: |
        LoginGraceTime 600
```

## Keeping Long-Lived Sessions Alive

Nothing in the generated file sets `ClientAliveInterval`, so it keeps OpenSSH's
default of `0` and `sshd` never probes its clients. Two consequences are worth
planning for on a long-lived interactive session.

An idle session is dropped by whatever sits between the client and the pod. The
`slurm` chart exposes login pods through a `LoadBalancer` service by default,
and cloud load balancers close idle TCP flows on their own schedule — 350
seconds on AWS NLB, 600 on GCP, 4 minutes by default on Azure. The session
appears to hang with no error until the client notices. `TCPKeepAlive` is
enabled by default but does not prevent this, because Linux waits 7200 seconds
of idle before its first probe, far longer than any of those timeouts.

In the other direction, a client that goes away without closing its session
leaves `sshd` waiting indefinitely. On a worker pod that session also holds the
Slurm job it was adopted into.

Client-alive probes address both. They travel inside the encrypted connection,
so the load balancer sees traffic and keeps the flow open, and a client that
stops answering is disconnected after a bounded number of missed probes:

```yaml
loginsets:
  slinky:
    extraSshdConfig: |
      ClientAliveInterval 60
      ClientAliveCountMax 3
```

Pick an interval below the idle timeout of the load balancer in front of the
login service; 60 seconds clears the shortest of the timeouts above. With
`ClientAliveCountMax 3`, an unresponsive client is disconnected after three
missed probes, roughly three minutes. A shorter interval reclaims dead sessions
sooner but disconnects clients on briefly flaky links.

Setting this per NodeSet is worthwhile for the same reason when workers accept
SSH:

```yaml
nodesets:
  slinky:
    ssh:
      enabled: true
      extraSshdConfig: |
        ClientAliveInterval 60
        ClientAliveCountMax 3
```

Users can achieve the same from the client side with `ServerAliveInterval` in
their own `ssh_config`, but configuring the server covers every user without
each of them having to know.

## Settings the Operator Already Defines

`sshd_config` uses the **first** value obtained for each keyword, and
`extraSshdConfig` is appended **after** the settings the operator emits. A
keyword the operator already defines therefore cannot be overridden through
`extraSshdConfig` — the added line is parsed, found to be a repeat, and ignored.

Login pods have these defined already:

```
Include /etc/ssh/sshd_config.d/*.conf
UsePAM yes
X11Forwarding yes
Subsystem sftp internal-sftp
```

Worker pods with SSH enabled have these:

```
Include /etc/ssh/sshd_config.d/*.conf
UsePAM yes
Subsystem sftp internal-sftp
AuthenticationMethods publickey password
```

To change one of these, write the setting into a file matched by the `Include`
directive, such as `/etc/ssh/sshd_config.d/10-local.conf`, mounted through the
pod template. Because the `Include` is processed before anything else in the
file, a value set there wins over both the operator's settings and
`extraSshdConfig`.

Any keyword not in the lists above — `ClientAliveInterval` among them — is
undefined until you set it, so `extraSshdConfig` is the right place for it.
