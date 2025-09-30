// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/SlinkyProject/slurm-operator/internal/utils/domainname"
)

func (o *Accounting) Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-accounting", o.Name),
		Namespace: o.Namespace,
	}
}

func (o *Accounting) PrimaryName() string {
	key := o.Key()
	return fmt.Sprintf("%s-0", key.Name)
}

func (o *Accounting) ServiceKey() types.NamespacedName {
	key := o.Key()
	return types.NamespacedName{
		Name:      key.Name,
		Namespace: o.Namespace,
	}
}

func (o *Accounting) ServiceFQDN() string {
	s := o.ServiceKey()
	return domainname.Fqdn(s.Name, s.Namespace)
}

func (o *Accounting) ServiceFQDNShort() string {
	s := o.ServiceKey()
	return domainname.FqdnShort(s.Name, s.Namespace)
}

func (o *Accounting) AuthStorageKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Spec.StorageConfig.PasswordKeyRef.Name,
		Namespace: o.Namespace,
	}
}

func (o *Accounting) AuthStorageRef() *corev1.SecretKeySelector {
	authKey := o.AuthStorageKey()
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: authKey.Name,
		},
		Key: o.Spec.StorageConfig.PasswordKeyRef.Key,
	}
}

func (o *Accounting) AuthSlurmKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Spec.SlurmKeyRef.Name,
		Namespace: o.Namespace,
	}
}

func (o *Accounting) AuthSlurmRef() *corev1.SecretKeySelector {
	ref := o.Spec.SlurmKeyRef
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: ref.Name,
		},
		Key: ref.Key,
	}
}

func (o *Accounting) AuthJwtHs256Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Spec.JwtHs256KeyRef.Name,
		Namespace: o.Namespace,
	}
}

func (o *Accounting) AuthJwtHs256Ref() *corev1.SecretKeySelector {
	ref := o.Spec.JwtHs256KeyRef
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: ref.Name,
		},
		Key: ref.Key,
	}
}

func (o *Accounting) ConfigKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-accounting", o.Name),
		Namespace: o.Namespace,
	}
}
