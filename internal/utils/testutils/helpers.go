// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
)

const Timeout = 30 * time.Second

func NewObjectRef(obj client.Object) slinkyv1alpha1.ObjectReference {
	return slinkyv1alpha1.ObjectReference{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
}

func NewController(name string, slurmKeyRef, jwtHs256KeyRef corev1.SecretKeySelector, accounting *slinkyv1alpha1.Accounting) *slinkyv1alpha1.Controller {
	accountingRef := slinkyv1alpha1.ObjectReference{}
	if accounting != nil {
		accountingRef = NewObjectRef(accounting)
	}
	return &slinkyv1alpha1.Controller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1alpha1.ControllerAPIVersion,
			Kind:       slinkyv1alpha1.ControllerKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1alpha1.ControllerSpec{
			SlurmKeyRef:    slurmKeyRef,
			JwtHs256KeyRef: jwtHs256KeyRef,
			AccountingRef:  accountingRef,
			Slurmctld: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmctld",
				},
			},
			Reconfigure: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmctld",
				},
			},
			LogFile: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "alpine",
				},
			},
		},
	}
}

func NewSlurmKeyRef(name string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name + "-slurmkey",
		},
		Key: "slurm.key",
	}
}

func NewSlurmKeySecret(ref corev1.SecretKeySelector) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			ref.Key: []byte("slurm.key"),
		},
	}
}

func NewJwtHs256KeyRef(name string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name + "-jwths256key",
		},
		Key: "jwt_hs256.key",
	}
}

func NewJwtHs256KeySecret(ref corev1.SecretKeySelector) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			ref.Key: []byte("jwt_hs256.key"),
		},
	}
}

func NewAccounting(name string, slurmKeyRef, jwtHs256KeyRef corev1.SecretKeySelector, passwordRef corev1.SecretKeySelector) *slinkyv1alpha1.Accounting {
	return &slinkyv1alpha1.Accounting{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1alpha1.AccountingAPIVersion,
			Kind:       slinkyv1alpha1.AccountingKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1alpha1.AccountingSpec{
			SlurmKeyRef:    slurmKeyRef,
			JwtHs256KeyRef: jwtHs256KeyRef,
			StorageConfig: slinkyv1alpha1.StorageConfig{
				Host:           "mariadb",
				PasswordKeyRef: passwordRef,
			},
			Slurmdbd: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmdbd",
				},
			},
			InitConf: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "sackd",
				},
			},
		},
	}
}

func NewPasswordRef(name string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name + "-password",
		},
		Key: "password",
	}
}

func NewPasswordSecret(ref corev1.SecretKeySelector) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			ref.Key: []byte("password"),
		},
	}
}

func NewNodeset(name string, controller *slinkyv1alpha1.Controller, replicas int32) *slinkyv1alpha1.NodeSet {
	controllerRef := slinkyv1alpha1.ObjectReference{}
	if controller != nil {
		controllerRef = NewObjectRef(controller)
	}
	return &slinkyv1alpha1.NodeSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1alpha1.NodeSetAPIVersion,
			Kind:       slinkyv1alpha1.NodeSetKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1alpha1.NodeSetSpec{
			ControllerRef: controllerRef,
			Replicas:      ptr.To(replicas),
			Slurmd: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmd",
				},
			},
			LogFile: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "alpine",
				},
			},
		},
	}
}

func NewLoginset(name string, controller *slinkyv1alpha1.Controller, sssdConfRef corev1.SecretKeySelector) *slinkyv1alpha1.LoginSet {
	controllerRef := slinkyv1alpha1.ObjectReference{}
	if controller != nil {
		controllerRef = NewObjectRef(controller)
	}
	return &slinkyv1alpha1.LoginSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1alpha1.LoginSetAPIVersion,
			Kind:       slinkyv1alpha1.LoginSetKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1alpha1.LoginSetSpec{
			ControllerRef: controllerRef,
			Login: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "login",
				},
			},
			SssdConfRef: sssdConfRef,
		},
	}
}

func NewSssdConfRef(name string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name + "-sssdconf",
		},
		Key: "sssd.conf",
	}
}

func NewSssdConfSecret(ref corev1.SecretKeySelector) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			ref.Key: []byte("sssd.conf"),
		},
	}
}

func NewRestapi(name string, controller *slinkyv1alpha1.Controller) *slinkyv1alpha1.RestApi {
	controllerRef := slinkyv1alpha1.ObjectReference{}
	if controller != nil {
		controllerRef = NewObjectRef(controller)
	}
	return &slinkyv1alpha1.RestApi{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1alpha1.RestApiAPIVersion,
			Kind:       slinkyv1alpha1.RestApiKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1alpha1.RestApiSpec{
			ControllerRef: controllerRef,
			Slurmrestd: slinkyv1alpha1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmrestd",
				},
			},
		},
	}
}

func NewToken(name string, jwtHs256KeySecret *corev1.Secret) *slinkyv1alpha1.Token {
	return &slinkyv1alpha1.Token{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1alpha1.TokenAPIVersion,
			Kind:       slinkyv1alpha1.TokenKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1alpha1.TokenSpec{
			Username: "slurm",
			JwtHs256KeyRef: slinkyv1alpha1.JwtSecretKeySelector{
				SecretKeySelector: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: jwtHs256KeySecret.Name,
					},
					Key: "jwt_hs256.key",
				},
			},
		},
	}
}
