// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"

const (
	SlurmUser    = "slurm"
	SlurmUserUid = int64(401)
	SlurmUserGid = SlurmUserUid

	SlurmConfigVolume = "slurm-config"
	SlurmConfigDir    = "/mnt/slurm"

	SlurmEtcVolume   = "slurm-etc"
	SlurmEtcMountDir = "/mnt/etc/slurm"
	SlurmEtcDir      = "/etc/slurm"

	SlurmPidFileVolume = "run"
	SlurmPidFileDir    = "/run"

	SlurmLogFileVolume = "slurm-logfile"
	SlurmLogFileDir    = "/var/log/slurm"

	SlurmKeyFile = "slurm.key"
	AuthType     = "auth/slurm"
	CredType     = "cred/slurm" // #nosec G101
	AuthInfo     = "use_client_ids"

	AuthAltTypes      = "auth/jwt"
	JwtHs256KeyFile   = "jwt_hs256.key"
	JwtHs256KeyPath   = SlurmEtcDir + "/" + JwtHs256KeyFile
	AuthAltParameters = "jwt_key=" + JwtHs256KeyPath

	LogTimeFormat = "iso8601,format_stderr"

	DevNull = "/dev/null"
)

// Worker
const (
	SlurmdPort = 6818
	SshPort    = 22

	SlurmdUser = "root"

	SlurmdLogFile     = "slurmd.log"
	SlurmdLogFilePath = SlurmLogFileDir + "/" + SlurmdLogFile

	SlurmdSpoolDir = "/var/spool/slurmd"
)

// Controller
const (
	SlurmctldPort = 6817

	SlurmctldLogFile     = "slurmctld.log"
	SlurmctldLogFilePath = SlurmLogFileDir + "/" + SlurmctldLogFile

	SlurmAuthSocketVolume  = "slurm-authsocket"
	SlurmctldAuthSocketDir = "/run/slurmctld"

	SlurmctldStateSaveVolume = "statesave"

	SlurmctldSpoolDir = "/var/spool/slurmctld"
)

// Accounting
const (
	SlurmdbdPort = 6819

	SlurmdbdConfFile = "slurmdbd.conf"
)

const (
	AnnotationAuthSlurmKeyHash    = slinkyv1beta1.SlinkyPrefix + "slurm-key-hash"
	AnnotationAuthJwtHs256KeyHash = slinkyv1beta1.SlinkyPrefix + "jwt-hs256-key-hash"
)

const (
	AnnotationSshdConfHash    = slinkyv1beta1.SlinkyPrefix + "sshd-conf-hash"
	AnnotationSssdConfHash    = slinkyv1beta1.SlinkyPrefix + "sssd-conf-hash"
	AnnotationSshHostKeysHash = slinkyv1beta1.SlinkyPrefix + "ssh-host-keys-hash"
)
