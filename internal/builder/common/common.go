// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	_ "embed"
	"fmt"
	"slices"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/config"
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

func (b *CommonBuilder) GetPodResourceLimits(pod corev1.PodSpec) (int64, int64) {
	var cpu, memory int64 = 0, 0

	if pod.Resources != nil {
		cpu = pod.Resources.Limits.Cpu().Value()
		memory = pod.Resources.Limits.Memory().Value()
	}

	return cpu, memory
}

func (b *CommonBuilder) GetContainerResourceLimits(container corev1.Container) (int64, int64) {
	var cpu, memory int64 = 0, 0

	if container.Resources.Limits != nil {
		cpu = container.Resources.Limits.Cpu().Value()
		memory = container.Resources.Limits.Memory().Value()
	}

	return cpu, memory
}

func JwtSecretProjection(secret *corev1.SecretKeySelector, path string) corev1.SecretProjection {
	return corev1.SecretProjection{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secret.Name,
		},
		Items: []corev1.KeyToPath{
			{Key: secret.Key, Path: path},
		},
	}
}

func JwksConfigProjection(configMap *corev1.ConfigMapKeySelector, path string) corev1.ConfigMapProjection {
	return corev1.ConfigMapProjection{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: configMap.Name,
		},
		Items: []corev1.KeyToPath{
			{Key: configMap.Key, Path: path},
		},
	}
}

// BuildMergedConfig returns a slurm.conf snippet containing merged parameter options.
func BuildMergedConfig(confRaw string, mergeConfig map[string][]string) string {
	conf := config.NewBuilder().WithFinalNewline(false)

	lineVals := parseSlurmConfKV(confRaw)

	paramKeys := structutils.Keys(mergeConfig)
	slices.Sort(paramKeys)
	for _, pKey := range paramKeys {
		val, ok := lineVals[strings.ToLower(pKey)]
		if !ok {
			continue
		}
		mergeVals := set.New(mergeConfig[pKey]...)
		seenOptKeys := set.New[string]()
		for _, mv := range mergeVals.UnsortedList() {
			seenOptKeys.Insert(parseKVKey(mv))
		}
		for v := range strings.SplitSeq(val, ",") {
			if v == "" {
				continue
			}
			olk := parseKVKey(v)
			if seenOptKeys.Has(olk) {
				continue
			}
			mergeVals.Insert(strings.ToLower(v))
			seenOptKeys.Insert(olk)
		}
		conf.AddProperty(config.NewProperty(pKey, strings.Join(mergeVals.SortedList(), ",")))
	}

	return conf.Build()
}

func parseSlurmConfKV(confRaw string) map[string]string {
	out := make(map[string]string)
	for _, line := range parseSlurmConfLogicalLines(confRaw) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" || val == "" {
			continue
		}
		if strings.ContainsAny(val, " \n") {
			continue
		}
		out[strings.ToLower(key)] = val
	}
	return out
}

func parseSlurmConfLogicalLines(confRaw string) []string {
	out := make([]string, 0, strings.Count(confRaw, "\n")+1)
	var b strings.Builder
	for line := range strings.SplitSeq(confRaw, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		if before, ok := strings.CutSuffix(line, `\`); ok {
			b.WriteString(before)
			continue
		}
		b.WriteString(line)
		out = append(out, b.String())
		b.Reset()
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

func parseKVKey(s string) string {
	k, _, _ := strings.Cut(s, "=")
	return strings.ToLower(k)
}
