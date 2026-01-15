// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/domainname"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func ConfiglessArgs(controller *slinkyv1beta1.Controller) []string {
	host := controller.ServiceFQDNShort()
	port := SlurmctldPort
	if controller.Spec.External {
		externalConfig := controller.Spec.ExternalConfig
		host = externalConfig.Host
		port = externalConfig.Port
	}
	args := []string{
		"--conf-server",
		fmt.Sprintf("%s:%d", host, port),
	}
	return args
}

//go:embed scripts/logfile.sh
var logfileScript string

func (b *CommonBuilder) LogfileContainer(container slinkyv1beta1.ContainerWrapper, logfilePath string) corev1.Container {
	opts := ContainerOpts{
		Base: corev1.Container{
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
				{Name: SlurmLogFileVolume, MountPath: SlurmLogFileDir},
			},
		},
		Merge: container.Container,
	}

	return b.BuildContainer(opts)
}

func LogFileVolume() corev1.Volume {
	out := corev1.Volume{
		Name: SlurmLogFileVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	return out
}

func PidfileVolume() corev1.Volume {
	out := corev1.Volume{
		Name: SlurmPidFileVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	return out
}

func DefaultPort(port, def int32) int32 {
	if port == 0 {
		return def
	}
	return port
}

func MergeEnvVar(envVarList1, envVarList2 []corev1.EnvVar, sep string) []corev1.EnvVar {
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
	keys := structutils.Keys(envVarMap)
	sort.Strings(keys)
	envVarList := make([]corev1.EnvVar, 0, len(envVarMap))
	for _, k := range keys {
		v := envVarMap[k]
		envVar := corev1.EnvVar{
			Name:      k,
			Value:     strings.Join(v.Values, sep),
			ValueFrom: v.ValueFrom,
		}
		envVarList = append(envVarList, envVar)
	}
	return envVarList
}

func SlurmClusterWorkerService(controllerName, namespace string) string {
	return domainname.Fqdn(SlurmClusterWorkerServiceName(controllerName), namespace)
}

// slurmClusterWorkerServiceName returns the service name for all worker nodes in a Slurm cluster
// Format: "slurm-workers-{controller-name}"
func SlurmClusterWorkerServiceName(controllerName string) string {
	// Derive service name dynamically from component constants
	componentPlural := labels.WorkerComp + "s"
	return fmt.Sprintf("slurm-%s-%s", componentPlural, controllerName)
}

// slurmClusterWorkerPodDisruptionBudgetName returns the PDB name for all worker nodes in a Slurm cluster
// Format: "slurm-workers-pdb-{controller-name}"
func SlurmClusterWorkerPodDisruptionBudgetName(controllerName string) string {
	// Derive service name dynamically from component constants
	componentPlural := labels.WorkerComp + "s"
	return fmt.Sprintf("slurm-%s-pdb-%s", componentPlural, controllerName)
}

func EtcSlurmVolume() corev1.Volume {
	out := corev1.Volume{
		Name: SlurmEtcVolume,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	return out
}

//go:embed scripts/initconf.sh
var initConfScript string

func (b *CommonBuilder) InitconfContainer(container slinkyv1beta1.ContainerWrapper) corev1.Container {
	opts := ContainerOpts{
		Base: corev1.Container{
			Name: "initconf",
			Env: []corev1.EnvVar{
				{
					Name:  "SLURM_USER",
					Value: SlurmUser,
				},
			},
			Command: []string{
				"sh",
				"-c",
				initConfScript,
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: SlurmEtcVolume, MountPath: SlurmEtcMountDir},
				{Name: SlurmConfigVolume, MountPath: SlurmConfigDir, ReadOnly: true},
			},
		},
		Merge: container.Container,
	}

	return b.BuildContainer(opts)
}
