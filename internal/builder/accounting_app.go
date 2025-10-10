// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"context"
	_ "embed"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

const (
	SlurmdbdPort = 6819

	slurmdbdConfFile = "slurmdbd.conf"
)

func (b *Builder) BuildAccounting(accounting *slinkyv1beta1.Accounting) (*appsv1.StatefulSet, error) {
	key := accounting.Key()
	serviceKey := accounting.ServiceKey()

	selectorLabels := labels.NewBuilder().
		WithAccountingSelectorLabels(accounting).
		Build()
	objectMeta := metadata.NewBuilder(key).
		WithMetadata(accounting.Spec.Template.PodMetadata).
		WithLabels(labels.NewBuilder().WithAccountingLabels(accounting).Build()).
		Build()

	podTemplate, err := b.accountingPodTemplate(accounting)
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

	if err := controllerutil.SetControllerReference(accounting, o, b.client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to set owner controller: %w", err)
	}

	return o, nil
}

func (b *Builder) accountingPodTemplate(accounting *slinkyv1beta1.Accounting) (corev1.PodTemplateSpec, error) {
	ctx := context.TODO()
	key := accounting.Key()

	hashMap, err := b.getAccountingHashes(ctx, accounting)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	objectMeta := metadata.NewBuilder(key).
		WithLabels(labels.NewBuilder().WithAccountingLabels(accounting).Build()).
		WithAnnotations(hashMap).
		WithAnnotations(map[string]string{
			annotationDefaultContainer: labels.AccountingApp,
		}).
		Build()

	spec := accounting.Spec
	template := spec.Template.PodSpecWrapper

	opts := PodTemplateOpts{
		Key: key,
		Metadata: slinkyv1beta1.Metadata{
			Annotations: objectMeta.Annotations,
			Labels:      objectMeta.Labels,
		},
		base: corev1.PodSpec{
			AutomountServiceAccountToken: ptr.To(false),
			Containers: []corev1.Container{
				b.slurmdbdContainer(spec.Slurmdbd.Container),
			},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    ptr.To(slurmUserUid),
				RunAsGroup:   ptr.To(slurmUserGid),
				FSGroup:      ptr.To(slurmUserGid),
			},
			Volumes: accountingVolumes(accounting),
		},
		merge: template.PodSpec,
	}

	return b.buildPodTemplate(opts), nil
}

func accountingVolumes(accounting *slinkyv1beta1.Accounting) []corev1.Volume {
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
									Name: accounting.ConfigKey().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: slurmdbdConfFile, Path: slurmdbdConfFile},
								},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: accounting.AuthSlurmRef().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: accounting.AuthSlurmRef().Key, Path: slurmKeyFile},
								},
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: accounting.AuthJwtHs256Ref().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: accounting.AuthJwtHs256Ref().Key, Path: JwtHs256KeyFile},
								},
							},
						},
					},
				},
			},
		},
		pidfileVolume(),
	}
	return out
}

func (b *Builder) slurmdbdContainer(merge corev1.Container) corev1.Container {
	opts := ContainerOpts{
		base: corev1.Container{
			Name: labels.AccountingApp,
			Ports: []corev1.ContainerPort{
				{
					Name:          labels.AccountingApp,
					ContainerPort: SlurmdbdPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(SlurmdbdPort),
					},
				},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsNonRoot: ptr.To(true),
				RunAsUser:    ptr.To(slurmUserUid),
				RunAsGroup:   ptr.To(slurmUserGid),
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: slurmEtcVolume, MountPath: slurmEtcDir, ReadOnly: true},
				{Name: slurmPidFileVolume, MountPath: slurmPidFileDir},
			},
		},
		merge: merge,
	}

	return b.BuildContainer(opts)
}

const (
	annotationSlurmdbdConfHash = slinkyv1beta1.SlinkyPrefix + "slurmdbd-conf-hash"
)

func (b *Builder) getAccountingHashes(ctx context.Context, accounting *slinkyv1beta1.Accounting) (map[string]string, error) {
	hashMap, err := b.getAuthHashesFromAccounting(ctx, accounting)
	if err != nil {
		return nil, err
	}

	dbdConfig := &corev1.Secret{}
	dbdConfigKey := accounting.ConfigKey()
	if err := b.client.Get(ctx, dbdConfigKey, dbdConfig); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	slurmdbdConfHash := crypto.CheckSumFromMap(dbdConfig.Data)

	hashMap = structutils.MergeMaps(hashMap, map[string]string{
		annotationSlurmdbdConfHash: slurmdbdConfHash,
	})

	return hashMap, nil
}

func (b *Builder) getAuthHashesFromAccounting(ctx context.Context, accounting *slinkyv1beta1.Accounting) (map[string]string, error) {
	authSlurm := &corev1.Secret{}
	authSlurmKey := accounting.AuthSlurmKey()
	if err := b.client.Get(ctx, authSlurmKey, authSlurm); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	authSlurmKeyHash := crypto.CheckSumFromMap(authSlurm.Data)

	authJwtHs256 := &corev1.Secret{}
	authJwtHs256Key := accounting.AuthJwtHs256Key()
	if err := b.client.Get(ctx, authJwtHs256Key, authJwtHs256); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	authJwtHs256KeyHash := crypto.CheckSumFromMap(authJwtHs256.Data)

	hashMap := map[string]string{
		annotationAuthSlurmKeyHash:    authSlurmKeyHash,
		annotationAuthJwtHs256KeyHash: authJwtHs256KeyHash,
	}

	return hashMap, nil
}
