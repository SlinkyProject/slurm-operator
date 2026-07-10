# RestAPI Job Submission Guide

## Table of Contents

<!-- mdformat-toc start --slug=github --no-anchors --maxlevel=6 --minlevel=1 -->

- [RestAPI Job Submission Guide](#restapi-job-submission-guide)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Container Configuration](#container-configuration)
  - [Job Submission Process](#job-submission-process)
    - [Creating a JWT](#creating-a-jwt)
    - [Submitting a Job](#submitting-a-job)

<!-- mdformat-toc end -->

## Overview

This guide provides an overview of the configuration required to submit jobs to
the Slurm RestAPI when running in Slurm-operator. For complete documentation on
the Slurm RestAPI, please refer to the [official documentation][restapi-docs].

## Container Configuration

In order to submit jobs to the Slurm RestAPI as a specific user, that user must
exist in the container in which `slurmctld` is running. For the purposes of this
demo, we will be generating the user's JWT in the login pod, so they must exist
in the `login` container as well. The UID and GID of this user must match in
both containers.

There are several ways that a user can be made to exist within these container
environments. In a production environment, the installation and configuration of
an Identity Provider such as SSSD would be the recommended deployment pattern.

In a development environment, the user and group may be created in the
Dockerfile used to build the `slurmctld` and `login` container images.

For more information on building Slinky containers, please see [containers].
Configuration files such as `sssd.conf` can be mounted into the loginset and
controller pods' containers as described on [this page].

## Job Submission Process

### Creating a JWT

To submit a job using the Slurm RestAPI, a user must first generate a JWT using
the `scontrol token` command. To do this, the user should `ssh` to the deployed
LoginSet and execute the `scontrol token` command.

> [!WARNING]
> This token is all that is required in order to submit jobs as a user. As such,
> it should be protected like a password!

Once the user has generated a JWT, they should `export` that token as an
environment variable in their shell. For the purposes of this demo, we will
assume that this environment variable is `$SLURM_JWT`.

### Submitting a Job

Once a token has been generated and exported as an environment variable, a user
can submit jobs via the Slurm RestAPI. Below is a simple job that executes
`srun hostname` on a single node in the `all` partition:

```bash
❯ curl -sS -o /tmp/curl.log -k -vvvv \
  -H "X-SLURM-USER-TOKEN: $SLURM_JWT" \
  -H "Content-Type: application/json" \
  -X POST "http://172.18.0.8:6820/slurm/v0.0.45/job/submit" \
  -d '{
    "job": {
      "name": "demo",
      "partition": "all",
      "current_working_directory": "/tmp",
      "environment": [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
      ],
      "nodes": "1",
      "tasks": 1
    },
    "script": "#!/bin/bash\nsrun hostname\n"
  }'
```

After submitting the job via the RestAPI, you should see a response with the
status `HTTP/1.1 200 OK`, indicating that the job was submitted.

<!-- Links -->

[containers]: https://slinky.schedmd.com/projects/containers
[restapi-docs]: https://slurm.schedmd.com/rest.html
[this page]: override-config-file.md
