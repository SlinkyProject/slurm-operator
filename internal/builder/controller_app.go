// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	_ "embed"
	"fmt"
	"path"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
)

const (
	SlurmctldPort = 6817

	slurmctldLogFile     = "slurmctld.log"
	slurmctldLogFilePath = slurmLogFileDir + "/" + slurmctldLogFile

	slurmAuthSocketVolume  = "slurm-authsocket"
	slurmctldAuthSocketDir = "/run/slurmctld"

	slurmctldStateSaveVolume = "statesave"

	slurmctldSpoolDir = "/var/spool/slurmctld"
)

func (b *Builder) BuildController(controller *slinkyv1beta1.Controller) (*appsv1.StatefulSet, error) {
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
			Name: slurmctldStateSaveVolume,
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
				Name:      slurmctldStateSaveVolume,
				Namespace: key.Namespace,
			},
			Spec: persistence.PersistentVolumeClaimSpec,
		}
		o.Spec.VolumeClaimTemplates = append(o.Spec.VolumeClaimTemplates, volumeClaimTemplate)
	default:
		volume := corev1.Volume{
			Name: slurmctldStateSaveVolume,
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

func (b *Builder) controllerPodTemplate(controller *slinkyv1beta1.Controller) (corev1.PodTemplateSpec, error) {
	ctx := context.TODO()
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

	// Build annotations with SSSD hash if configured
	annotations := map[string]string{
		annotationDefaultContainer: labels.ControllerApp,
	}
	if controller.Spec.SssdConfRef.Name != "" {
		sssdSecret := &corev1.Secret{}
		sssdSecretKey := controller.SssdSecretKey()
		if err := b.client.Get(ctx, sssdSecretKey, sssdSecret); err != nil {
			if !apierrors.IsNotFound(err) {
				return corev1.PodTemplateSpec{}, fmt.Errorf("failed to get object (%s): %w", klog.KObj(sssdSecret), err)
			}
		}
		sssdConfRefKey := controller.SssdSecretRef().Key
		sssdConfHash := crypto.CheckSum([]byte(sssdSecret.StringData[sssdConfRefKey]))
		annotations[annotationSssdConfHash] = sssdConfHash
	}

	objectMeta := metadata.NewBuilder(key).
		WithAnnotations(controller.Annotations).
		WithLabels(controller.Labels).
		WithMetadata(controller.Spec.Template.Metadata).
		WithLabels(labels.NewBuilder().WithControllerLabels(controller).Build()).
		WithAnnotations(annotations).
		Build()

	spec := controller.Spec
	template := spec.Template.PodSpecWrapper

	opts := PodTemplateOpts{
		Key: key,
		Metadata: slinkyv1beta1.Metadata{
			Annotations: objectMeta.Annotations,
			Labels:      objectMeta.Labels,
		},
		base: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup: ptr.To[int64](401),
			},
			Containers: []corev1.Container{
				b.slurmctldContainer(spec.Slurmctld.Container, controller),
			},
			InitContainers: []corev1.Container{
				b.reconfigureContainer(spec.Reconfigure),
				b.logfileContainer(spec.LogFile, slurmctldLogFilePath),
			},
			Volumes: controllerVolumes(controller, extraConfigMapNames),
		},
		merge: template.PodSpec,
	}

	return b.buildPodTemplate(opts), nil
}

func controllerVolumes(controller *slinkyv1beta1.Controller, extra []string) []corev1.Volume {
	out := []corev1.Volume{
		{
			Name: slurmEtcVolume,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0o640),
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
									{Key: controller.AuthSlurmRef().Key, Path: slurmKeyFile, Mode: ptr.To[int32](0o600)},
								},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controller.AuthJwtHs256Ref().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: controller.AuthJwtHs256Ref().Key, Path: JwtHs256KeyFile, Mode: ptr.To[int32](0o600)},
								},
							},
						},
					},
				},
			},
		},
		logFileVolume(),
		pidfileVolume(),
		{
			Name: slurmAuthSocketVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
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
	// Add SSSD volume if configured (optional)
	if controller.Spec.SssdConfRef.Name != "" {
		sssdVolume := corev1.Volume{
			Name: sssdConfVolume,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0o600),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controller.SssdSecretRef().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: controller.SssdSecretRef().Key, Path: sssdConfFile, Mode: ptr.To[int32](0o600)},
								},
							},
						},
					},
				},
			},
		}
		out = append(out, sssdVolume)
	}
	return out
}

func clusterSpoolDir(clustername string) string {
	return path.Join(slurmctldSpoolDir, clustername)
}

func (b *Builder) slurmctldContainer(merge corev1.Container, controller *slinkyv1beta1.Controller) corev1.Container {
	clusterName := controller.ClusterName()
	volumeMounts := []corev1.VolumeMount{
		{Name: slurmEtcVolume, MountPath: slurmEtcDir, ReadOnly: true},
		{Name: slurmPidFileVolume, MountPath: slurmPidFileDir},
		{Name: slurmctldStateSaveVolume, MountPath: clusterSpoolDir(clusterName)},
		{Name: slurmAuthSocketVolume, MountPath: slurmctldAuthSocketDir},
		{Name: slurmLogFileVolume, MountPath: slurmLogFileDir},
	}
	// Add SSSD mount if configured (optional)
	// Mount to staging dir (not /etc/sssd/) so entrypoint can copy with correct permissions
	if controller.Spec.SssdConfRef.Name != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      sssdConfVolume,
			MountPath: "/run/sssd-mounted/sssd.conf",
			SubPath:   sssdConfFile,
			ReadOnly:  true,
		})
	}

	opts := ContainerOpts{
		base: corev1.Container{
			Name: labels.ControllerApp,
			Ports: []corev1.ContainerPort{
				{
					Name:          labels.ControllerApp,
					ContainerPort: SlurmctldPort,
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
			VolumeMounts: volumeMounts,
		},
		merge: merge,
	}

	return b.BuildContainer(opts)
}

//go:embed scripts/reconfigure.sh
var reconfigureScript string

func (b *Builder) reconfigureContainer(container slinkyv1beta1.ContainerWrapper) corev1.Container {
	opts := ContainerOpts{
		base: corev1.Container{
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
				{Name: slurmEtcVolume, MountPath: slurmEtcDir, ReadOnly: true},
				{Name: slurmAuthSocketVolume, MountPath: slurmctldAuthSocketDir, ReadOnly: true},
			},
		},
		merge: container.Container,
	}

	return b.BuildContainer(opts)
}
