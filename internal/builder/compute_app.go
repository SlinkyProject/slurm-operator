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
	"k8s.io/utils/ptr"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils"
)

const (
	SlurmdPort = 6818

	slurmdUser = "root"

	slurmdLogFile     = "slurmd.log"
	slurmdLogFilePath = slurmLogFileDir + "/" + slurmdLogFile

	slurmdSpoolDir = "/var/spool/slurmd"
)

func (b *Builder) BuildComputePodTemplate(nodeset *slinkyv1alpha1.NodeSet, controller *slinkyv1alpha1.Controller) corev1.PodTemplateSpec {
	key := nodeset.Key()

	objectMeta := metadata.NewBuilder(key).
		WithMetadata(nodeset.Spec.Template.PodMetadata).
		WithLabels(labels.NewBuilder().WithComputeLabels(nodeset).Build()).
		WithAnnotations(map[string]string{
			annotationDefaultContainer: labels.ComputeApp,
		}).
		Build()

	template := nodeset.Spec.Template

	o := corev1.PodTemplateSpec{
		ObjectMeta: objectMeta,
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			EnableServiceLinks:           ptr.To(false),
			Affinity:                     template.Affinity,
			Containers: []corev1.Container{
				slurmdContainer(nodeset, controller),
			},
			Hostname:         template.Hostname,
			ImagePullSecrets: template.ImagePullSecrets,
			InitContainers: []corev1.Container{
				logfileContainer(template.LogFile, slurmdLogFilePath),
			},
			NodeSelector:      template.NodeSelector,
			PriorityClassName: template.PriorityClassName,
			Tolerations:       template.Tolerations,
			Volumes:           utils.MergeList(nodesetVolumes(controller), template.Volumes),
		},
	}

	return o
}

func nodesetVolumes(controller *slinkyv1alpha1.Controller) []corev1.Volume {
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

func slurmdContainer(nodeset *slinkyv1alpha1.NodeSet, controller *slinkyv1alpha1.Controller) corev1.Container {
	template := nodeset.Spec.Template

	out := corev1.Container{
		Name:            labels.ComputeApp,
		Env:             template.Container.Env,
		Args:            slurmdArgs(nodeset, controller),
		Image:           template.Container.Image,
		ImagePullPolicy: template.Container.ImagePullPolicy,
		Ports: []corev1.ContainerPort{
			{
				Name:          labels.ComputeApp,
				ContainerPort: SlurmdPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Resources: template.Container.Resources,
		StartupProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"scontrol",
						"show",
						"slurmd",
					},
				},
			},
			FailureThreshold: 6,
			PeriodSeconds:    10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"scontrol",
						"show",
						"slurmd",
					},
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
						"scontrol update nodename=$(hostname) state=down reason=preStop && scontrol delete nodename=$(hostname);",
					},
				},
			},
		},
		VolumeDevices: template.Container.VolumeDevices,
		VolumeMounts: []corev1.VolumeMount{
			{Name: slurmEtcVolume, MountPath: slurmEtcDir, ReadOnly: true},
			{Name: slurmLogFileVolume, MountPath: slurmLogFileDir},
		},
	}
	out.VolumeMounts = append(out.VolumeMounts, template.Container.VolumeMounts...)
	return out
}

func slurmdArgs(nodeset *slinkyv1alpha1.NodeSet, controller *slinkyv1alpha1.Controller) []string {
	args := []string{"-Z"}
	args = append(args, configlessArgs(controller)...)
	args = append(args, slurmdConfArgs(nodeset)...)
	return utils.MergeList(args, nodeset.Spec.Template.Container.Args)
}

func slurmdConfArgs(nodeset *slinkyv1alpha1.NodeSet) []string {
	extraConf := []string{}
	if nodeset.Spec.Template.ExtraConf != "" {
		extraConf = strings.Split(nodeset.Spec.Template.ExtraConf, " ")
	}

	name := nodeset.Name
	if nodeset.Spec.Template.Hostname != "" {
		name = strings.Trim(nodeset.Spec.Template.Hostname, "-")
	}

	confMap := map[string]string{
		"Features": name,
	}
	for _, item := range extraConf {
		pair := strings.SplitN(item, "=", 2)
		key := cases.Title(language.English).String(pair[0])
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
