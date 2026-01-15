// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
)

const (
	LoginPort = 22

	SackdVolume = "sackd-dir"
	SackdDir    = "/run/slurm"

	SackdSocket     = "sack.socket"
	SackdSocketPath = SackdDir + "/" + SackdSocket

	SshConfigVolume    = "ssh-config"
	SshdConfigFile     = "sshd_config"
	SshdConfigFilePath = SshDir + "/" + SshdConfigFile

	SshDir = "/etc/ssh"

	SshHostKeysVolume = "ssh-host-keys"

	SshHostRsaKeyFile        = "ssh_host_rsa_key"
	SshHostRsaKeyFilePath    = SshDir + "/" + SshHostRsaKeyFile
	SshHostRsaPubKeyFile     = SshHostRsaKeyFile + ".pub"
	SshHostRsaKeyPubFilePath = SshDir + "/" + SshHostRsaPubKeyFile

	SshHostEd25519KeyFile        = "ssh_host_ed25519_key"
	SshHostEd25519KeyFilePath    = SshDir + "/" + SshHostEd25519KeyFile
	SshHostEd25519PubKeyFile     = SshHostEd25519KeyFile + ".pub"
	SshHostEd25519PubKeyFilePath = SshDir + "/" + SshHostEd25519PubKeyFile

	SshHostEcdsaKeyFile        = "ssh_host_ecdsa_key"
	SshHostEcdsaKeyFilePath    = SshDir + "/" + SshHostEcdsaKeyFile
	SshHostEcdsaPubKeyFile     = SshHostEcdsaKeyFile + ".pub"
	SshHostEcdsaPubKeyFilePath = SshDir + "/" + SshHostEcdsaPubKeyFile

	SssdConfVolume   = "sssd-conf"
	SssdConfFile     = "sssd.conf"
	SssdConfDir      = "/etc/sssd"
	SssdConfFilePath = SssdConfDir + "/" + SssdConfFile

	authorizedKeysVolume = "authorized-keys"
	authorizedKeysFile   = "authorized_keys"

	rootAuthorizedKeysFilePath = "/root/.ssh/" + authorizedKeysFile
)

func (b *LoginBuilder) BuildLogin(loginset *slinkyv1beta1.LoginSet) (*appsv1.Deployment, error) {
	key := loginset.Key()

	selectorLabels := labels.NewBuilder().
		WithLoginSelectorLabels(loginset).
		Build()
	objectMeta := metadata.NewBuilder(key).
		WithAnnotations(loginset.Annotations).
		WithLabels(loginset.Labels).
		WithMetadata(loginset.Spec.Template.Metadata).
		WithLabels(labels.NewBuilder().WithLoginLabels(loginset).Build()).
		Build()

	podTemplate, err := b.loginPodTemplate(loginset)
	if err != nil {
		return nil, fmt.Errorf("failed to build pod template: %w", err)
	}

	o := &appsv1.Deployment{
		ObjectMeta: objectMeta,
		Spec: appsv1.DeploymentSpec{
			Replicas:             loginset.Spec.Replicas,
			RevisionHistoryLimit: ptr.To[int32](0),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: podTemplate,
		},
	}

	if err := controllerutil.SetControllerReference(loginset, o, b.client.Scheme()); err != nil {
		return nil, fmt.Errorf("failed to set owner controller: %w", err)
	}

	return o, nil
}

func (b *LoginBuilder) loginPodTemplate(loginset *slinkyv1beta1.LoginSet) (corev1.PodTemplateSpec, error) {
	ctx := context.TODO()
	key := loginset.Key()

	controller, err := b.refResolver.GetController(ctx, loginset.Spec.ControllerRef)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	hashMap, err := b.getLoginHashes(ctx, loginset)
	if err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	objectMeta := metadata.NewBuilder(key).
		WithAnnotations(loginset.Annotations).
		WithLabels(loginset.Labels).
		WithMetadata(loginset.Spec.Template.Metadata).
		WithLabels(labels.NewBuilder().WithLoginLabels(loginset).Build()).
		WithAnnotations(hashMap).
		WithAnnotations(map[string]string{
			annotationDefaultContainer: labels.LoginApp,
		}).
		Build()

	spec := loginset.Spec
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
				b.loginContainer(spec.Login.Container, controller),
			},
			InitContainers: []corev1.Container{
				b.CommonBuilder.InitconfContainer(spec.InitConf),
			},
			DNSConfig: &corev1.PodDNSConfig{
				Searches: []string{
					common.SlurmClusterWorkerService(spec.ControllerRef.Name, loginset.Namespace),
				},
			},
			Volumes: loginVolumes(loginset, controller),
		},
		Merge: template.PodSpec,
	}

	return b.CommonBuilder.BuildPodTemplate(opts), nil
}

func loginVolumes(loginset *slinkyv1beta1.LoginSet, controller *slinkyv1beta1.Controller) []corev1.Volume {
	out := []corev1.Volume{
		common.EtcSlurmVolume(),
		{
			Name: SackdVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		},
		{
			Name: common.SlurmConfigVolume,
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
		{
			Name: SshHostKeysVolume,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0o600),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: loginset.SshHostKeys().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: SshHostRsaKeyFile, Path: SshHostRsaKeyFile, Mode: ptr.To[int32](0o600)},
									{Key: SshHostRsaPubKeyFile, Path: SshHostRsaPubKeyFile, Mode: ptr.To[int32](0o644)},
									{Key: SshHostEd25519KeyFile, Path: SshHostEd25519KeyFile, Mode: ptr.To[int32](0o600)},
									{Key: SshHostEd25519PubKeyFile, Path: SshHostEd25519PubKeyFile, Mode: ptr.To[int32](0o644)},
									{Key: SshHostEcdsaKeyFile, Path: SshHostEcdsaKeyFile, Mode: ptr.To[int32](0o600)},
									{Key: SshHostEcdsaPubKeyFile, Path: SshHostEcdsaPubKeyFile, Mode: ptr.To[int32](0o644)},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: SshConfigVolume,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0o600),
					Sources: []corev1.VolumeProjection{
						{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: loginset.SshConfigKey().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: SshdConfigFile, Path: SshdConfigFile, Mode: ptr.To[int32](0o600)},
									{Key: authorizedKeysFile, Path: authorizedKeysFile, Mode: ptr.To[int32](0o600)},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: SssdConfVolume,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](0o600),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: loginset.SssdSecretRef().Name,
								},
								Items: []corev1.KeyToPath{
									{Key: loginset.SssdSecretRef().Key, Path: SssdConfFile, Mode: ptr.To[int32](0o600)},
								},
							},
						},
					},
				},
			},
		},
	}
	return out
}

func (b *LoginBuilder) loginContainer(merge corev1.Container, controller *slinkyv1beta1.Controller) corev1.Container {
	opts := common.ContainerOpts{
		Base: corev1.Container{
			Name: labels.LoginApp,
			Env: []corev1.EnvVar{
				{
					Name:  "SACKD_OPTIONS",
					Value: strings.Join(common.ConfiglessArgs(controller), " "),
				},
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          labels.LoginApp,
					ContainerPort: LoginPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"test",
							"-S",
							SackdSocketPath,
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: common.SlurmEtcVolume, MountPath: common.SlurmEtcDir, ReadOnly: true},
				{Name: SackdVolume, MountPath: SackdDir},
				{Name: SshHostKeysVolume, MountPath: SshHostRsaKeyFilePath, SubPath: SshHostRsaKeyFile, ReadOnly: true},
				{Name: SshHostKeysVolume, MountPath: SshHostRsaKeyPubFilePath, SubPath: SshHostRsaPubKeyFile, ReadOnly: true},
				{Name: SshHostKeysVolume, MountPath: SshHostEd25519KeyFilePath, SubPath: SshHostEd25519KeyFile, ReadOnly: true},
				{Name: SshHostKeysVolume, MountPath: SshHostEd25519PubKeyFilePath, SubPath: SshHostEd25519PubKeyFile, ReadOnly: true},
				{Name: SshHostKeysVolume, MountPath: SshHostEcdsaKeyFilePath, SubPath: SshHostEcdsaKeyFile, ReadOnly: true},
				{Name: SshHostKeysVolume, MountPath: SshHostEcdsaPubKeyFilePath, SubPath: SshHostEcdsaPubKeyFile, ReadOnly: true},
				{Name: SshConfigVolume, MountPath: SshdConfigFilePath, SubPath: SshdConfigFile, ReadOnly: true},
				{Name: SshConfigVolume, MountPath: rootAuthorizedKeysFilePath, SubPath: authorizedKeysFile, ReadOnly: true},
				{Name: SssdConfVolume, MountPath: SssdConfFilePath, SubPath: SssdConfFile, ReadOnly: true},
			},
		},
		Merge: merge,
	}

	return b.CommonBuilder.BuildContainer(opts)
}

func (b *LoginBuilder) getLoginHashes(ctx context.Context, loginset *slinkyv1beta1.LoginSet) (map[string]string, error) {
	SshConfig := &corev1.ConfigMap{}
	SshConfigKey := loginset.SshConfigKey()
	if err := b.client.Get(ctx, SshConfigKey, SshConfig); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(SshConfig), err)
		}
	}

	SshHostKeys := &corev1.Secret{}
	SshHostKeysKey := loginset.SshHostKeys()
	if err := b.client.Get(ctx, SshHostKeysKey, SshHostKeys); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(SshHostKeys), err)
		}
	}

	SssdSecret := &corev1.Secret{}
	SssdSecretKey := loginset.SssdSecretKey()
	if err := b.client.Get(ctx, SssdSecretKey, SssdSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(SssdSecret), err)
		}
	}

	hashMap := map[string]string{
		common.AnnotationSshHostKeysHash: crypto.CheckSumFromMap(SshHostKeys.Data),
		common.AnnotationSshdConfHash:    crypto.CheckSum([]byte(SshConfig.Data[SshdConfigFile])),
		common.AnnotationSssdConfHash:    crypto.CheckSum([]byte(SssdSecret.StringData[loginset.SssdSecretRef().Key])),
	}

	return hashMap, nil
}
