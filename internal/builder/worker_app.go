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
)

const (
	SlurmdPort = 6818

	slurmdUser = "root"

	slurmdLogFile     = "slurmd.log"
	slurmdLogFilePath = slurmLogFileDir + "/" + slurmdLogFile

	slurmdSpoolDir = "/var/spool/slurmd"
)

func (b *Builder) BuildWorkerPodTemplate(nodeset *slinkyv1alpha1.NodeSet, controller *slinkyv1alpha1.Controller) corev1.PodTemplateSpec {
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
		Metadata: slinkyv1alpha1.Metadata{
			Annotations: objectMeta.Annotations,
			Labels:      objectMeta.Labels,
		},
		base: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			EnableServiceLinks:           ptr.To(false),
			Affinity:                     template.Affinity,
			Containers: []corev1.Container{
				b.slurmdContainer(nodeset, controller),
			},
			Hostname:         template.Hostname,
			Subdomain:        b.getSubdomainForHostnameServices(nodeset),
			DNSConfig:        b.getDNSConfigForHostnameServices(nodeset),
			ImagePullSecrets: template.ImagePullSecrets,
			InitContainers: []corev1.Container{
				b.logfileContainer(spec.LogFile, slurmdLogFilePath),
			},
			NodeSelector:      template.NodeSelector,
			PriorityClassName: template.PriorityClassName,
			Tolerations:       template.Tolerations,
			Volumes:           nodesetVolumes(controller),
		},
		merge: template.PodSpec,
	}

	o := b.buildPodTemplate(opts)

	return o
}

// getClusterWideServiceName returns the cluster-wide service name for hostname services
func (b *Builder) getClusterWideServiceName(nodeset *slinkyv1alpha1.NodeSet) string {
	return slurmClusterWorkerServiceName(nodeset.Spec.ControllerRef.Name)
}

// getSubdomainForHostnameServices returns the subdomain for hostname services
func (b *Builder) getSubdomainForHostnameServices(nodeset *slinkyv1alpha1.NodeSet) string {
	return b.getClusterWideServiceName(nodeset)
}

// getDNSConfigForHostnameServices returns DNS configuration for hostname services
func (b *Builder) getDNSConfigForHostnameServices(nodeset *slinkyv1alpha1.NodeSet) *corev1.PodDNSConfig {
	clusterServiceName := b.getClusterWideServiceName(nodeset)
	searches := []string{
		// Cluster-wide worker service (all worker pods in this Slurm cluster)
		fmt.Sprintf("%s.%s.svc.cluster.local", clusterServiceName, nodeset.Namespace),
		// Standard search domains
		fmt.Sprintf("%s.svc.cluster.local", nodeset.Namespace),
		"svc.cluster.local",
		"cluster.local",
	}

	return &corev1.PodDNSConfig{
		Searches: searches,
	}
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

func (b *Builder) slurmdContainer(nodeset *slinkyv1alpha1.NodeSet, controller *slinkyv1alpha1.Controller) corev1.Container {
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
			},
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
			VolumeMounts: []corev1.VolumeMount{
				{Name: slurmEtcVolume, MountPath: slurmEtcDir, ReadOnly: true},
				{Name: slurmLogFileVolume, MountPath: slurmLogFileDir},
			},
		},
		merge: merge,
	}

	return b.BuildContainer(opts)
}

func slurmdArgs(nodeset *slinkyv1alpha1.NodeSet, controller *slinkyv1alpha1.Controller) []string {
	args := []string{"-Z"}
	args = append(args, configlessArgs(controller)...)
	args = append(args, slurmdConfArgs(nodeset)...)
	return args
}

func slurmdConfArgs(nodeset *slinkyv1alpha1.NodeSet) []string {
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
