// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package workerbuilder

import (
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	loginbuilder "github.com/SlinkyProject/slurm-operator/internal/builder/loginbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
	slurmtaints "github.com/SlinkyProject/slurm-operator/pkg/taints"
)

func (b *WorkerBuilder) BuildWorkerPodTemplate(nodeset *slinkyv1beta1.NodeSet, controller *slinkyv1beta1.Controller) corev1.PodTemplateSpec {
	ctx := context.TODO()
	key := nodeset.Key()

	hashMap, err := b.getWorkerHashes(ctx, nodeset)
	if err != nil {
		return corev1.PodTemplateSpec{}
	}

	objectMeta := metadata.NewBuilder(key).
		WithAnnotations(nodeset.Annotations).
		WithLabels(nodeset.Labels).
		WithMetadata(nodeset.Spec.Template.Metadata).
		WithLabels(labels.NewBuilder().WithWorkerLabels(nodeset).Build()).
		WithAnnotations(hashMap).
		WithAnnotations(map[string]string{
			annotationDefaultContainer: labels.WorkerApp,
		}).
		Build()

	spec := nodeset.Spec
	template := spec.Template.PodSpecWrapper

	opts := common.PodTemplateOpts{
		Key: key,
		Metadata: slinkyv1beta1.Metadata{
			Annotations: objectMeta.Annotations,
			Labels:      objectMeta.Labels,
		},
		Base: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			EnableServiceLinks:           ptr.To(false),
			Containers: []corev1.Container{
				b.slurmdContainer(nodeset, controller),
			},
			Subdomain: common.SlurmClusterWorkerServiceName(spec.ControllerRef.Name),
			DNSConfig: &corev1.PodDNSConfig{
				Searches: []string{
					common.SlurmClusterWorkerService(spec.ControllerRef.Name, nodeset.Namespace),
				},
			},
			InitContainers: []corev1.Container{
				b.CommonBuilder.LogfileContainer(spec.LogFile, common.SlurmdLogFilePath),
			},
			Volumes: nodesetVolumes(nodeset, controller),
			Tolerations: []corev1.Toleration{
				slurmtaints.TolerationWorkerNode,
			},
		},
		Merge: template.PodSpec,
	}

	return b.CommonBuilder.BuildPodTemplate(opts)
}

func nodesetVolumes(nodeset *slinkyv1beta1.NodeSet, controller *slinkyv1beta1.Controller) []corev1.Volume {
	out := []corev1.Volume{
		{
			Name: common.SlurmEtcVolume,
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
									{Key: controller.AuthSlurmRef().Key, Path: common.SlurmKeyFile},
								},
							},
						},
					},
				},
			},
		},
		common.LogFileVolume(),
	}

	// Add SSH host keys volume if SSH is enabled
	if nodeset.Spec.Ssh.Enabled {
		out = structutils.MergeList(out, []corev1.Volume{
			{
				Name: loginbuilder.SshConfigVolume,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: ptr.To[int32](0o600),
						Sources: []corev1.VolumeProjection{
							{
								ConfigMap: &corev1.ConfigMapProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: nodeset.SshConfigKey().Name,
									},
									Items: []corev1.KeyToPath{
										{Key: loginbuilder.SshdConfigFile, Path: loginbuilder.SshdConfigFile, Mode: ptr.To[int32](0o600)},
									},
								},
							},
						},
					},
				},
			},
			{
				Name: loginbuilder.SssdConfVolume,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: ptr.To[int32](0o600),
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: nodeset.SssdSecretRef().Name,
									},
									Items: []corev1.KeyToPath{
										{Key: nodeset.SssdSecretRef().Key, Path: loginbuilder.SssdConfFile, Mode: ptr.To[int32](0o600)},
									},
								},
							},
						},
					},
				},
			},
		})
	}

	return out
}

func (b *WorkerBuilder) slurmdContainer(nodeset *slinkyv1beta1.NodeSet, controller *slinkyv1beta1.Controller) corev1.Container {
	merge := nodeset.Spec.Slurmd.Container

	// Base ports always include slurmd
	ports := []corev1.ContainerPort{
		{
			Name:          labels.WorkerApp,
			ContainerPort: common.SlurmdPort,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// Add SSH port if enabled
	if nodeset.Spec.Ssh.Enabled {
		ports = append(ports, corev1.ContainerPort{
			Name:          "ssh",
			ContainerPort: common.SshPort,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	// Base volume mounts
	volumeMounts := []corev1.VolumeMount{
		{Name: common.SlurmEtcVolume, MountPath: common.SlurmEtcDir, ReadOnly: true},
		{Name: common.SlurmLogFileVolume, MountPath: common.SlurmLogFileDir},
	}

	// Add SSH host key mounts if enabled
	if nodeset.Spec.Ssh.Enabled {
		volumeMounts = structutils.MergeList(volumeMounts, []corev1.VolumeMount{
			{Name: loginbuilder.SshConfigVolume, MountPath: loginbuilder.SshdConfigFilePath, SubPath: loginbuilder.SshdConfigFile, ReadOnly: true},
			{Name: loginbuilder.SssdConfVolume, MountPath: loginbuilder.SssdConfFilePath, SubPath: loginbuilder.SssdConfFile, ReadOnly: true},
		})
	}

	opts := common.ContainerOpts{
		Base: corev1.Container{
			Name: labels.WorkerApp,
			Args: slurmdArgs(nodeset, controller),
			Env: []corev1.EnvVar{
				{
					Name: "POD_TOPOLOGY",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: fmt.Sprintf("metadata.annotations['%s']", slinkyv1beta1.AnnotationNodeTopologyLine),
						},
					},
				},
			},
			Ports: ports,
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
			VolumeMounts: volumeMounts,
		},
		Merge: merge,
	}

	return b.CommonBuilder.BuildContainer(opts)
}

func slurmdArgs(nodeset *slinkyv1beta1.NodeSet, controller *slinkyv1beta1.Controller) []string {
	args := []string{"-Z"}
	args = append(args, common.ConfiglessArgs(controller)...)
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

func (b *WorkerBuilder) getWorkerHashes(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) (map[string]string, error) {
	sshConfig := &corev1.ConfigMap{}
	sshConfigKey := nodeset.SshConfigKey()
	if err := b.client.Get(ctx, sshConfigKey, sshConfig); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(sshConfig), err)
		}
	}

	sssdSecret := &corev1.Secret{}
	sssdSecretKey := nodeset.SssdSecretKey()
	if err := b.client.Get(ctx, sssdSecretKey, sssdSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(sssdSecret), err)
		}
	}
	sssdConfRefKey := nodeset.SssdSecretRef().Key

	hashMap := map[string]string{
		common.AnnotationSshdConfHash: crypto.CheckSum([]byte(sshConfig.Data[loginbuilder.SshdConfigFile])),
		common.AnnotationSssdConfHash: crypto.CheckSum([]byte(sssdSecret.StringData[sssdConfRefKey])),
	}

	return hashMap, nil
}
