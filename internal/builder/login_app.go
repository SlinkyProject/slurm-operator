// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

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

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
)

const (
	LoginPort = 22

	sackdVolume = "sackd-dir"
	sackdDir    = "/run/slurm"

	sackdSocket     = "sack.socket"
	sackdSocketPath = sackdDir + "/" + sackdSocket

	sshConfigVolume    = "ssh-config"
	sshdConfigFile     = "sshd_config"
	sshdConfigFilePath = sshDir + "/" + sshdConfigFile

	sshHostKeysVolume = "ssh-host-keys"

	sshDir = "/etc/ssh"

	sshHostRsaKeyFile        = "ssh_host_rsa_key"
	sshHostRsaKeyFilePath    = sshDir + "/" + sshHostRsaKeyFile
	sshHostRsaPubKeyFile     = sshHostRsaKeyFile + ".pub"
	sshHostRsaKeyPubFilePath = sshDir + "/" + sshHostRsaPubKeyFile

	sshHostEd25519KeyFile        = "ssh_host_ed25519_key"
	sshHostEd25519KeyFilePath    = sshDir + "/" + sshHostEd25519KeyFile
	sshHostEd25519PubKeyFile     = sshHostEd25519KeyFile + ".pub"
	sshHostEd25519PubKeyFilePath = sshDir + "/" + sshHostEd25519PubKeyFile

	sshHostEcdsaKeyFile        = "ssh_host_ecdsa_key"
	sshHostEcdsaKeyFilePath    = sshDir + "/" + sshHostEcdsaKeyFile
	sshHostEcdsaPubKeyFile     = sshHostEcdsaKeyFile + ".pub"
	sshHostEcdsaPubKeyFilePath = sshDir + "/" + sshHostEcdsaPubKeyFile

	sssdConfVolume   = "sssd-conf"
	sssdConfFile     = "sssd.conf"
	sssdConfDir      = "/etc/sssd"
	sssdConfFilePath = sssdConfDir + "/" + sssdConfFile

	authorizedKeysVolume = "authorized-keys"
	authorizedKeysFile   = "authorized_keys"

	rootAuthorizedKeysFilePath = "/root/.ssh/" + authorizedKeysFile
)

func (b *Builder) BuildLogin(loginset *slinkyv1alpha1.LoginSet) (*appsv1.Deployment, error) {
	key := loginset.Key()

	selectorLabels := labels.NewBuilder().
		WithLoginSelectorLabels(loginset).
		Build()
	objectMeta := metadata.NewBuilder(key).
		WithMetadata(loginset.Spec.Template.PodMetadata).
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

func (b *Builder) loginPodTemplate(loginset *slinkyv1alpha1.LoginSet) (corev1.PodTemplateSpec, error) {
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
		WithMetadata(loginset.Spec.Template.PodMetadata).
		WithLabels(labels.NewBuilder().WithLoginLabels(loginset).Build()).
		WithAnnotations(hashMap).
		WithAnnotations(map[string]string{
			annotationDefaultContainer: labels.LoginApp,
		}).
		Build()

	spec := loginset.Spec
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
				b.loginContainer(spec.Login.Container, controller),
			},
			Hostname:          template.Hostname,
			Subdomain:         b.getSubdomainForLoginHostnameServices(loginset),
			DNSConfig:         b.getDNSConfigForLoginHostnameServices(loginset),
			ImagePullSecrets:  template.ImagePullSecrets,
			NodeSelector:      template.NodeSelector,
			PriorityClassName: template.PriorityClassName,
			Tolerations:       template.Tolerations,
			Volumes:           loginVolumes(loginset, controller),
		},
		merge: template.PodSpec,
	}

	o := b.buildPodTemplate(opts)

	return o, nil
}

func loginVolumes(loginset *slinkyv1alpha1.LoginSet, controller *slinkyv1alpha1.Controller) []corev1.Volume {
	out := []corev1.Volume{
		{
			Name: sackdVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		},
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
		{
			Name: sshHostKeysVolume,
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
									{Key: sshHostRsaKeyFile, Path: sshHostRsaKeyFile, Mode: ptr.To[int32](0o600)},
									{Key: sshHostRsaPubKeyFile, Path: sshHostRsaPubKeyFile, Mode: ptr.To[int32](0o644)},
									{Key: sshHostEd25519KeyFile, Path: sshHostEd25519KeyFile, Mode: ptr.To[int32](0o600)},
									{Key: sshHostEd25519PubKeyFile, Path: sshHostEd25519PubKeyFile, Mode: ptr.To[int32](0o644)},
									{Key: sshHostEcdsaKeyFile, Path: sshHostEcdsaKeyFile, Mode: ptr.To[int32](0o600)},
									{Key: sshHostEcdsaPubKeyFile, Path: sshHostEcdsaPubKeyFile, Mode: ptr.To[int32](0o644)},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: sshConfigVolume,
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
									{Key: sshdConfigFile, Path: sshdConfigFile, Mode: ptr.To[int32](0o600)},
									{Key: authorizedKeysFile, Path: authorizedKeysFile, Mode: ptr.To[int32](0o600)},
								},
							},
						},
					},
				},
			},
		},
		{
			Name: sssdConfVolume,
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
									{Key: loginset.SssdSecretRef().Key, Path: sssdConfFile, Mode: ptr.To[int32](0o600)},
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

func (b *Builder) loginContainer(merge corev1.Container, controller *slinkyv1alpha1.Controller) corev1.Container {
	opts := ContainerOpts{
		base: corev1.Container{
			Name: labels.LoginApp,
			Env:  loginEnv(merge, controller),
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
							sackdSocketPath,
						},
					},
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: slurmEtcVolume, MountPath: slurmEtcDir, ReadOnly: true},
				{Name: sackdVolume, MountPath: sackdDir},
				{Name: sshHostKeysVolume, MountPath: sshHostRsaKeyFilePath, SubPath: sshHostRsaKeyFile, ReadOnly: true},
				{Name: sshHostKeysVolume, MountPath: sshHostRsaKeyPubFilePath, SubPath: sshHostRsaPubKeyFile, ReadOnly: true},
				{Name: sshHostKeysVolume, MountPath: sshHostEd25519KeyFilePath, SubPath: sshHostEd25519KeyFile, ReadOnly: true},
				{Name: sshHostKeysVolume, MountPath: sshHostEd25519PubKeyFilePath, SubPath: sshHostEd25519PubKeyFile, ReadOnly: true},
				{Name: sshHostKeysVolume, MountPath: sshHostEcdsaKeyFilePath, SubPath: sshHostEcdsaKeyFile, ReadOnly: true},
				{Name: sshHostKeysVolume, MountPath: sshHostEcdsaPubKeyFilePath, SubPath: sshHostEcdsaPubKeyFile, ReadOnly: true},
				{Name: sshConfigVolume, MountPath: sshdConfigFilePath, SubPath: sshdConfigFile, ReadOnly: true},
				{Name: sshConfigVolume, MountPath: rootAuthorizedKeysFilePath, SubPath: authorizedKeysFile, ReadOnly: true},
				{Name: sssdConfVolume, MountPath: sssdConfFilePath, SubPath: sssdConfFile, ReadOnly: true},
			},
		},
		merge: merge,
	}

	return b.BuildContainer(opts)
}

func loginEnv(container corev1.Container, controller *slinkyv1alpha1.Controller) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  "SACKD_OPTIONS",
			Value: strings.Join(configlessArgs(controller), " "),
		},
	}
	return mergeEnvVar(env, container.Env, " ")
}

const (
	annotationSshdConfHash    = slinkyv1alpha1.LoginSetPrefix + "sshd-conf-hash"
	annotationSssdConfHash    = slinkyv1alpha1.LoginSetPrefix + "sssd-conf-hash"
	annotationSshHostKeysHash = slinkyv1alpha1.LoginSetPrefix + "ssh-host-keys-hash"
)

func (b *Builder) getLoginHashes(ctx context.Context, loginset *slinkyv1alpha1.LoginSet) (map[string]string, error) {
	sshConfig := &corev1.ConfigMap{}
	sshConfigKey := loginset.SshConfigKey()
	if err := b.client.Get(ctx, sshConfigKey, sshConfig); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(sshConfig), err)
		}
	}
	sshdConfigHash := crypto.CheckSum([]byte(sshConfig.Data[sshdConfigFile]))

	sshHostKeys := &corev1.Secret{}
	sshHostKeysKey := loginset.SshHostKeys()
	if err := b.client.Get(ctx, sshHostKeysKey, sshHostKeys); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(sshHostKeys), err)
		}
	}
	sshHostKeysHash := crypto.CheckSumFromMap(sshHostKeys.Data)

	sssdSecret := &corev1.Secret{}
	sssdSecretKey := loginset.SssdSecretKey()
	if err := b.client.Get(ctx, sssdSecretKey, sssdSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get object (%s): %w", klog.KObj(sssdSecret), err)
		}
	}
	sssdConfRefKey := loginset.SssdSecretRef().Key
	SssdConfHash := crypto.CheckSum([]byte(sssdSecret.StringData[sssdConfRefKey]))

	hashMap := map[string]string{
		annotationSshHostKeysHash: sshHostKeysHash,
		annotationSshdConfHash:    sshdConfigHash,
		annotationSssdConfHash:    SssdConfHash,
	}

	return hashMap, nil
}

// getDNSConfigForLoginHostnameServices returns DNS configuration for hostname services
func (b *Builder) getDNSConfigForLoginHostnameServices(loginset *slinkyv1alpha1.LoginSet) *corev1.PodDNSConfig {
	serviceName := slurmClusterWorkerServiceName(loginset.Spec.ControllerRef.Name)
	searches := []string{
		fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, loginset.Namespace),
		fmt.Sprintf("%s.svc.cluster.local", loginset.Namespace),
		"svc.cluster.local",
		"cluster.local",
	}

	return &corev1.PodDNSConfig{
		Searches: searches,
	}
}

// getSubdomainForLoginHostnameServices returns subdomain for hostname services
func (b *Builder) getSubdomainForLoginHostnameServices(loginset *slinkyv1alpha1.LoginSet) string {
	return slurmClusterWorkerServiceName(loginset.Spec.ControllerRef.Name)
}
