// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	slurmtaints "github.com/SlinkyProject/slurm-operator/pkg/taints"
)

const (
	SlurmdPort = 6818
	SshPort    = 22

	slurmdUser = "root"

	slurmdLogFile     = "slurmd.log"
	slurmdLogFilePath = slurmLogFileDir + "/" + slurmdLogFile

	slurmdSpoolDir = "/var/spool/slurmd"
)

func (b *Builder) BuildWorkerPodTemplate(nodeset *slinkyv1beta1.NodeSet, controller *slinkyv1beta1.Controller) corev1.PodTemplateSpec {
	key := nodeset.Key()

	objectMeta := metadata.NewBuilder(key).
		WithMetadata(nodeset.Spec.Template.PodMetadata).
		WithLabels(labels.NewBuilder().WithWorkerLabels(nodeset).Build()).
		WithAnnotations(map[string]string{
			annotationDefaultContainer: labels.WorkerApp,
		}).
		Build()

	spec := nodeset.Spec
	template := spec.Template.PodSpecWrapper

	opts := PodTemplateOpts{
		Key: key,
		Metadata: slinkyv1beta1.Metadata{
			Annotations: objectMeta.Annotations,
			Labels:      objectMeta.Labels,
		},
		base: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			EnableServiceLinks:           ptr.To(false),
			Containers: []corev1.Container{
				b.slurmdContainer(nodeset, controller),
			},
			Subdomain: slurmClusterWorkerServiceName(spec.ControllerRef.Name),
			DNSConfig: &corev1.PodDNSConfig{
				Searches: []string{
					slurmClusterWorkerService(spec.ControllerRef.Name, nodeset.Namespace),
				},
			},
			InitContainers: []corev1.Container{
				b.logfileContainer(spec.LogFile, slurmdLogFilePath),
			},
			Volumes: nodesetVolumes(controller),
			Tolerations: []corev1.Toleration{
				slurmtaints.TolerationWorkerNode,
			},
		},
		merge: template.PodSpec,
	}

	return b.buildPodTemplate(opts)
}

func nodesetVolumes(controller *slinkyv1beta1.Controller) []corev1.Volume {
	out := []corev1.Volume{
		{
			Name: slurmEtcVolume,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0o600),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controller.AuthSlurmRef().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: controller.AuthSlurmRef().Key, Path: slurmKeyFile},
								},
							},
						},
					},
				},
			},
		},
		logFileVolume(),
	}
	return out
}

func (b *Builder) slurmdContainer(nodeset *slinkyv1beta1.NodeSet, controller *slinkyv1beta1.Controller) corev1.Container {
	merge := nodeset.Spec.Slurmd.Container

	opts := ContainerOpts{
		base: corev1.Container{
			Name: labels.WorkerApp,
			Args: slurmdArgs(nodeset, controller),
			Ports: []corev1.ContainerPort{
				{
					Name:          labels.WorkerApp,
					ContainerPort: SlurmdPort,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          "ssh",
					ContainerPort: SshPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/livez",
						Port: intstr.FromString(labels.WorkerApp),
					},
				},
				FailureThreshold: 6,
				PeriodSeconds:    10,
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/livez",
						Port: intstr.FromString(labels.WorkerApp),
					},
				},
				FailureThreshold: 6,
				PeriodSeconds:    10,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readyz",
						Port: intstr.FromString(labels.WorkerApp),
					},
				},
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged: ptr.To(true),
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{
						"BPF",
						"NET_ADMIN",
						"SYS_ADMIN",
						"SYS_NICE",
					},
				},
			},
			Lifecycle: &corev1.Lifecycle{
				PreStop: &corev1.LifecycleHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"/usr/bin/sh",
							"-c",
							"scontrol update nodename=$(hostname) state=down reason='Pod is terminating' && scontrol delete nodename=$(hostname);",
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: slurmEtcVolume, MountPath: slurmEtcDir, ReadOnly: true},
				{Name: slurmLogFileVolume, MountPath: slurmLogFileDir},
			},
		},
		merge: merge,
	}

	return b.BuildContainer(opts)
}

func slurmdArgs(nodeset *slinkyv1beta1.NodeSet, controller *slinkyv1beta1.Controller) []string {
	args := []string{"-Z"}
	args = append(args, configlessArgs(controller)...)
	args = append(args, slurmdConfArgs(nodeset)...)
	return args
}

func slurmdConfArgs(nodeset *slinkyv1beta1.NodeSet) []string {
	extraConf := []string{}
	if nodeset.Spec.ExtraConf != "" {
		extraConf = strings.Split(nodeset.Spec.ExtraConf, " ")
	}

	name := nodeset.Name
	template := nodeset.Spec.Template.PodSpecWrapper
	if template.Hostname != "" {
		name = strings.Trim(template.Hostname, "-")
	}

	confMap := map[string]string{
		"Features": name,
	}
	for _, item := range extraConf {
		pair := strings.SplitN(item, "=", 2)
		key := cases.Title(language.English).String(pair[0])
		if len(pair) != 2 {
			panic(fmt.Sprintf("malformed --conf item: %v", item))
		}
		val := pair[1]
		if key == "Features" || key == "Feature" {
			// Slurm treats trailing 's' as optional. We have to
			// specially handle 'Feature(s)' because we require at
			// least one feature but the user can request additional.
			key = "Features"
		}
		if ret, ok := confMap[key]; !ok {
			confMap[key] = val
		} else {
			confMap[key] = ret + fmt.Sprintf(",%s", val)
		}
	}

	confList := []string{}
	for key, val := range confMap {
		confList = append(confList, fmt.Sprintf("%s=%s", key, val))
	}
	sort.Strings(confList)

	args := []string{
		"--conf",
		fmt.Sprintf("'%s'", strings.Join(confList, " ")),
	}

	return args
}
