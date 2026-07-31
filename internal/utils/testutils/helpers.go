// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"context"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

const Timeout = 30 * time.Second
const Interval = 2 * time.Second

// ExpectNotRecreated asserts that a controller does not recreate obj, deleting
// it again whenever it reappears until it has stayed absent for several polls.
//
// A reconcile that read the owner before its deletionTimestamp was visible can
// still be in flight and recreate obj once, and how long that straggler takes
// varies by runner speed, so a fixed window is inherently flaky. Draining
// stragglers instead still fails on a genuine regression, where obj is
// recreated for as long as the timeout allows.
func ExpectNotRecreated(ctx context.Context, c client.Client, obj client.Object) {
	ginkgo.GinkgoHelper()
	const absencesRequired = 5
	key := client.ObjectKeyFromObject(obj)
	absences := 0
	gomega.Eventually(func(g gomega.Gomega) {
		err := c.Get(ctx, key, obj)
		if err == nil {
			g.Expect(c.Delete(ctx, obj)).To(gomega.Succeed())
			absences = 0
		} else {
			g.Expect(client.IgnoreNotFound(err)).To(gomega.Succeed())
			absences++
		}
		g.Expect(absences).To(gomega.BeNumerically(">=", absencesRequired), "%s keeps being recreated", key)
	}, Timeout, Interval).Should(gomega.Succeed())
}

func NewController(name string, slurmKeyRef, jwtKeyRef corev1.SecretKeySelector, accounting *slinkyv1beta1.Accounting) *slinkyv1beta1.Controller {
	var accountingRef *corev1.LocalObjectReference
	if accounting != nil {
		accountingRef = new(corev1.LocalObjectReference{
			Name: accounting.Name,
		})
	}
	return &slinkyv1beta1.Controller{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1beta1.ControllerAPIVersion,
			Kind:       slinkyv1beta1.ControllerKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1beta1.ControllerSpec{
			SlurmKeyRef:   slurmKeyRef,
			JwtKeyRef:     &jwtKeyRef,
			AccountingRef: accountingRef,
			Slurmctld: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmctld",
				},
			},
			Reconfigure: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmctld",
				},
			},
			LogFile: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "alpine",
				},
			},
			Persistence: slinkyv1beta1.ControllerPersistence{
				Enabled: ptr.To(false),
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

func NewJwtKeyRef(name string) corev1.SecretKeySelector {
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name + "-jwtkey",
		},
		Key: "jwt.key",
	}
}

func NewJwtKeySecret(ref corev1.SecretKeySelector) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ref.Name,
			Namespace: corev1.NamespaceDefault,
		},
		Data: map[string][]byte{
			ref.Key: []byte("jwt.key"),
		},
	}
}

func NewAccounting(name string, slurmKeyRef, jwtKeyRef corev1.SecretKeySelector, passwordRef corev1.SecretKeySelector) *slinkyv1beta1.Accounting {
	return &slinkyv1beta1.Accounting{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1beta1.AccountingAPIVersion,
			Kind:       slinkyv1beta1.AccountingKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1beta1.AccountingSpec{
			SlurmKeyRef: slurmKeyRef,
			JwtKeyRef:   &jwtKeyRef,
			StorageConfig: slinkyv1beta1.StorageConfig{
				Host:           "mariadb",
				PasswordKeyRef: passwordRef,
			},
			Slurmdbd: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmdbd",
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

func NewNodeset(name string, controller *slinkyv1beta1.Controller, replicas int32) *slinkyv1beta1.NodeSet {
	var controllerRef corev1.LocalObjectReference
	if controller != nil {
		controllerRef = corev1.LocalObjectReference{
			Name: controller.Name,
		}
	}
	return &slinkyv1beta1.NodeSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1beta1.NodeSetAPIVersion,
			Kind:       slinkyv1beta1.NodeSetKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1beta1.NodeSetSpec{
			ControllerRef: controllerRef,
			Replicas:      ptr.To(replicas),
			Slurmd: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmd",
				},
			},
		},
	}
}

func NewLoginset(name string, controller *slinkyv1beta1.Controller, sssdConfRef corev1.SecretKeySelector) *slinkyv1beta1.LoginSet {
	var controllerRef corev1.LocalObjectReference
	if controller != nil {
		controllerRef = corev1.LocalObjectReference{
			Name: controller.Name,
		}
	}
	return &slinkyv1beta1.LoginSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1beta1.LoginSetAPIVersion,
			Kind:       slinkyv1beta1.LoginSetKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1beta1.LoginSetSpec{
			ControllerRef: controllerRef,
			Login: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "login",
				},
			},
			InitConf: slinkyv1beta1.ContainerWrapper{
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

func NewRestapi(name string, controller *slinkyv1beta1.Controller) *slinkyv1beta1.RestApi {
	var controllerRef corev1.LocalObjectReference
	if controller != nil {
		controllerRef = corev1.LocalObjectReference{
			Name: controller.Name,
		}
	}
	return &slinkyv1beta1.RestApi{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1beta1.RestApiAPIVersion,
			Kind:       slinkyv1beta1.RestApiKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1beta1.RestApiSpec{
			ControllerRef: controllerRef,
			Slurmrestd: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmrestd",
				},
			},
		},
	}
}

func NewToken(name string, jwtKeySecret *corev1.Secret) *slinkyv1beta1.Token {
	return &slinkyv1beta1.Token{
		TypeMeta: metav1.TypeMeta{
			APIVersion: slinkyv1beta1.TokenAPIVersion,
			Kind:       slinkyv1beta1.TokenKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: slinkyv1beta1.TokenSpec{
			Username: "slurm",
			JwtKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: jwtKeySecret.Name,
				},
				Key: "jwt.key",
			},
		},
	}
}
