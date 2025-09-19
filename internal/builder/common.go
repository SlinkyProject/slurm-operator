// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	_ "embed"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	clientutils "github.com/SlinkyProject/slurm-client/pkg/utils"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
)

const (
	slurmUser    = "slurm"
	slurmUserUid = int64(401)
	slurmUserGid = slurmUserUid

	slurmConfigVolume = "slurm-config"
	slurmConfigDir    = "/mnt/slurm"

	slurmEtcVolume   = "slurm-etc"
	slurmEtcMountDir = "/mnt/etc/slurm"
	slurmEtcDir      = "/etc/slurm"

	slurmPidFileVolume = "run"
	slurmPidFileDir    = "/run"

	slurmLogFileVolume = "slurm-logfile"
	slurmLogFileDir    = "/var/log/slurm"

	slurmKeyFile = "slurm.key"
	authType     = "auth/slurm"
	credType     = "cred/slurm" // #nosec G101
	authInfo     = "use_client_ids"

	authAltTypes      = "auth/jwt"
	JwtHs256KeyFile   = "jwt_hs256.key"
	jwtHs256KeyPath   = slurmEtcDir + "/" + JwtHs256KeyFile
	authAltParameters = "jwt_key=" + jwtHs256KeyPath

	logTimeFormat = "iso8601,format_stderr"

	devNull = "/dev/null"
)

const (
	annotationAuthSlurmKeyHash    = slinkyv1alpha1.SlinkyPrefix + "slurm-key-hash"
	annotationAuthJwtHs256KeyHash = slinkyv1alpha1.SlinkyPrefix + "jwt-hs256-key-hash"
)

func configlessArgs(controller *slinkyv1alpha1.Controller) []string {
	args := []string{
		"--conf-server",
		fmt.Sprintf("%s:%d", controller.PrimaryFQDN(), SlurmctldPort),
	}
	return args
}

//go:embed scripts/initconf.sh
var initConfScript string

func (b *Builder) initconfContainer(container slinkyv1alpha1.ContainerMinimal) corev1.Container {
	merge := &corev1.Container{}
	clientutils.RemarshalOrDie(container, merge)

	opts := ContainerOpts{
		base: corev1.Container{
			Name: "initconf",
			Env: []corev1.EnvVar{
				{
					Name:  "SLURM_USER",
					Value: slurmUser,
				},
			},
			Command: []string{
				"tini",
				"-g",
				"--",
				"bash",
				"-c",
				initConfScript,
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: slurmEtcVolume, MountPath: slurmEtcMountDir},
				{Name: slurmConfigVolume, MountPath: slurmConfigDir, ReadOnly: true},
			},
		},
		merge: *merge,
	}

	return b.BuildContainer(opts)
}

//go:embed scripts/logfile.sh
var logfileScript string

func (b *Builder) logfileContainer(container slinkyv1alpha1.ContainerMinimal, logfilePath string) corev1.Container {
	merge := &corev1.Container{}
	clientutils.RemarshalOrDie(container, merge)

	opts := ContainerOpts{
		base: corev1.Container{
			Name: "logfile",
			Env: []corev1.EnvVar{
				{
					Name:  "SOCKET",
					Value: logfilePath,
				},
			},
			Command: []string{
				"sh",
				"-c",
				logfileScript,
			},
			RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
			VolumeMounts: []corev1.VolumeMount{
				{Name: slurmLogFileVolume, MountPath: slurmLogFileDir},
			},
		},
		merge: *merge,
	}

	return b.BuildContainer(opts)
}

func logFileVolume() corev1.Volume {
	out := corev1.Volume{
		Name: slurmLogFileVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	return out
}

func etcSlurmVolume() corev1.Volume {
	out := corev1.Volume{
		Name: slurmEtcVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	return out
}

func pidfileVolume() corev1.Volume {
	out := corev1.Volume{
		Name: slurmPidFileVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	return out
}

func defaultPort(port, def int32) int32 {
	if port == 0 {
		return def
	}
	return port
}

func mergeEnvVar(envVarList1, envVarList2 []corev1.EnvVar, sep string) []corev1.EnvVar {
	type _envVar struct {
		Values    []string
		ValueFrom *corev1.EnvVarSource
	}
	envVarMap := make(map[string]_envVar, 0)
	for _, env := range envVarList1 {
		ev := envVarMap[env.Name]
		if env.Value != "" {
			ev.Values = append(ev.Values, env.Value)
		}
		if env.ValueFrom != nil {
			ev.ValueFrom = env.ValueFrom
		}
		envVarMap[env.Name] = ev
	}
	for _, env := range envVarList2 {
		ev := envVarMap[env.Name]
		if env.Value != "" {
			ev.Values = append(ev.Values, env.Value)
		}
		if env.ValueFrom != nil {
			ev.Values = []string{}
			ev.ValueFrom = env.ValueFrom
		}
		envVarMap[env.Name] = ev
	}
	envVarList := make([]corev1.EnvVar, 0, len(envVarMap))
	for k, v := range envVarMap {
		envVar := corev1.EnvVar{
			Name:      k,
			Value:     strings.Join(v.Values, sep),
			ValueFrom: v.ValueFrom,
		}
		envVarList = append(envVarList, envVar)
	}
	return envVarList
}

// slurmClusterWorkerServiceName returns the service name for all worker nodes in a Slurm cluster
// Format: "slurm-workers-{controller-name}"
func slurmClusterWorkerServiceName(controllerName string) string {
	// Derive service name dynamically from component constants
	componentPlural := labels.WorkerComp + "s"
	return fmt.Sprintf("slurm-%s-%s", componentPlural, controllerName)
}
