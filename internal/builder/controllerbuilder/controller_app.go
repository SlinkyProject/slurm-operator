// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	_ "embed"
	"fmt"
	"path"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
)

func (b *ControllerBuilder) BuildController(controller *slinkyv1beta1.Controller) (*appsv1.StatefulSet, error) {
	key := controller.Key()
	serviceKey := controller.ServiceKey()
	selectorLabels := labels.NewBuilder().
		WithControllerSelectorLabels(controller).
		Build()
	objectMeta := metadata.NewBuilder(key).
		WithAnnotations(controller.Annotations).
		WithLabels(controller.Labels).
		WithMetadata(controller.Spec.Template.Metadata).
		WithLabels(labels.NewBuilder().WithControllerLabels(controller).Build()).
		Build()

	persistence := controller.Spec.Persistence

	podTemplate, err := b.controllerPodTemplate(controller)
	if err != nil {
		return nil, fmt.Errorf("failed to build pod template: %w", err)
	}

	o := &appsv1.StatefulSet{
		ObjectMeta: objectMeta,
		Spec: appsv1.StatefulSetSpec{
			PodManagementPolicy:  appsv1.ParallelPodManagement,
			Replicas:             ptr.To[int32](1),
			RevisionHistoryLimit: ptr.To[int32](0),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			ServiceName: serviceKey.Name,
			Template:    podTemplate,
		},
	}

	switch {
	case persistence.Enabled && persistence.ExistingClaim != "":
		volume := corev1.Volume{
			Name: common.SlurmctldStateSaveVolume,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: persistence.ExistingClaim,
				},
			},
		}
		o.Spec.Template.Spec.Volumes = append(o.Spec.Template.Spec.Volumes, volume)
	case persistence.Enabled:
		volumeClaimTemplate := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.SlurmctldStateSaveVolume,
				Namespace: key.Namespace,
			},
			Spec: persistence.PersistentVolumeClaimSpec,
		}
		o.Spec.VolumeClaimTemplates = append(o.Spec.VolumeClaimTemplates, volumeClaimTemplate)
	default:
		volume := corev1.Volume{
			Name: common.SlurmctldStateSaveVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		o.Spec.Template.Spec.Volumes = append(o.Spec.Template.Spec.Volumes, volume)
	}

	if err := controllerutil.SetControllerReference(controller, o, b.client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to set owner controller: %w", err)
	}

	return o, nil
}

func (b *ControllerBuilder) controllerPodTemplate(controller *slinkyv1beta1.Controller) (corev1.PodTemplateSpec, error) {
	key := controller.Key()

	size := len(controller.Spec.ConfigFileRefs) + len(controller.Spec.PrologScriptRefs) + len(controller.Spec.EpilogScriptRefs) + len(controller.Spec.PrologSlurmctldScriptRefs) + len(controller.Spec.EpilogSlurmctldScriptRefs)
	extraConfigMapNames := make([]string, 0, size)
	for _, ref := range controller.Spec.ConfigFileRefs {
		extraConfigMapNames = append(extraConfigMapNames, ref.Name)
	}
	for _, ref := range controller.Spec.PrologScriptRefs {
		extraConfigMapNames = append(extraConfigMapNames, ref.Name)
	}
	for _, ref := range controller.Spec.EpilogScriptRefs {
		extraConfigMapNames = append(extraConfigMapNames, ref.Name)
	}
	for _, ref := range controller.Spec.PrologSlurmctldScriptRefs {
		extraConfigMapNames = append(extraConfigMapNames, ref.Name)
	}
	for _, ref := range controller.Spec.EpilogSlurmctldScriptRefs {
		extraConfigMapNames = append(extraConfigMapNames, ref.Name)
	}

	objectMeta := metadata.NewBuilder(key).
		WithAnnotations(controller.Annotations).
		WithLabels(controller.Labels).
		WithMetadata(controller.Spec.Template.Metadata).
		WithLabels(labels.NewBuilder().WithControllerLabels(controller).Build()).
		WithAnnotations(map[string]string{
			annotationDefaultContainer: labels.ControllerApp,
		}).
		Build()

	spec := controller.Spec
	template := spec.Template.PodSpecWrapper

	opts := common.PodTemplateOpts{
		Key: key,
		Metadata: slinkyv1beta1.Metadata{
			Annotations: objectMeta.Annotations,
			Labels:      objectMeta.Labels,
		},
		Base: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			Containers: []corev1.Container{
				b.slurmctldContainer(spec.Slurmctld.Container, controller.ClusterName()),
			},
			InitContainers: []corev1.Container{
				b.reconfigureContainer(spec.Reconfigure),
				b.CommonBuilder.LogfileContainer(spec.LogFile, common.SlurmctldLogFilePath),
			},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    ptr.To(common.SlurmUserUid),
				RunAsGroup:   ptr.To(common.SlurmUserGid),
				FSGroup:      ptr.To(common.SlurmUserGid),
			},
			Volumes: controllerVolumes(controller, extraConfigMapNames),
		},
		Merge: template.PodSpec,
	}

	return b.CommonBuilder.BuildPodTemplate(opts), nil
}

func controllerVolumes(controller *slinkyv1beta1.Controller, extra []string) []corev1.Volume {
	out := []corev1.Volume{
		{
			Name: common.SlurmEtcVolume,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0o610),
					Sources: []corev1.VolumeProjection{
						{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controller.ConfigKey().Name,
								},
							},
						},
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
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controller.AuthJwtHs256Ref().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: controller.AuthJwtHs256Ref().Key, Path: common.JwtHs256KeyFile},
								},
							},
						},
					},
				},
			},
		},
		common.LogFileVolume(),
		common.PidfileVolume(),
		{
			Name: common.SlurmAuthSocketVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	slices.Sort(extra)
	for _, name := range extra {
		volumeProjection := corev1.VolumeProjection{
			ConfigMap: &corev1.ConfigMapProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: name,
				},
			},
		}
		out[0].Projected.Sources = append(out[0].Projected.Sources, volumeProjection)
	}
	return out
}

func clusterSpoolDir(clustername string) string {
	return path.Join(common.SlurmctldSpoolDir, clustername)
}

func (b *ControllerBuilder) slurmctldContainer(merge corev1.Container, clusterName string) corev1.Container {
	opts := common.ContainerOpts{
		Base: corev1.Container{
			Name: labels.ControllerApp,
			Ports: []corev1.ContainerPort{
				{
					Name:          labels.ControllerApp,
					ContainerPort: common.SlurmctldPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			StartupProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/livez",
						Port: intstr.FromString(labels.ControllerApp),
					},
				},
				FailureThreshold: 6,
				PeriodSeconds:    10,
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readyz",
						Port: intstr.FromString(labels.ControllerApp),
					},
				},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/livez",
						Port: intstr.FromString(labels.ControllerApp),
					},
				},
				FailureThreshold: 6,
				PeriodSeconds:    10,
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    ptr.To(common.SlurmUserUid),
				RunAsGroup:   ptr.To(common.SlurmUserGid),
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: common.SlurmEtcVolume, MountPath: common.SlurmEtcDir, ReadOnly: true},
				{Name: common.SlurmPidFileVolume, MountPath: common.SlurmPidFileDir},
				{Name: common.SlurmctldStateSaveVolume, MountPath: clusterSpoolDir(clusterName)},
				{Name: common.SlurmAuthSocketVolume, MountPath: common.SlurmctldAuthSocketDir},
				{Name: common.SlurmLogFileVolume, MountPath: common.SlurmLogFileDir},
			},
		},
		Merge: merge,
	}

	return b.CommonBuilder.BuildContainer(opts)
}

//go:embed scripts/reconfigure.sh
var reconfigureScript string

func (b *ControllerBuilder) reconfigureContainer(container slinkyv1beta1.ContainerWrapper) corev1.Container {
	opts := common.ContainerOpts{
		Base: corev1.Container{
			Name: "reconfigure",
			Command: []string{
				"tini",
				"-g",
				"--",
				"bash",
				"-c",
				reconfigureScript,
			},
			RestartPolicy: ptr.To(corev1.ContainerRestartPolicyAlways),
			VolumeMounts: []corev1.VolumeMount{
				{Name: common.SlurmEtcVolume, MountPath: common.SlurmEtcDir, ReadOnly: true},
				{Name: common.SlurmAuthSocketVolume, MountPath: common.SlurmctldAuthSocketDir, ReadOnly: true},
			},
		},
		Merge: container.Container,
	}

	return b.CommonBuilder.BuildContainer(opts)
}
