// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/SlinkyProject/slurm-operator/internal/utils/domainname"
)

func (o *LoginSet) Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Name,
		Namespace: o.Namespace,
	}
}

func (o *LoginSet) ServiceKey() types.NamespacedName {
	key := o.Key()
	return types.NamespacedName{
		Name:      key.Name,
		Namespace: o.Namespace,
	}
}

func (o *LoginSet) ServiceFQDN() string {
	s := o.ServiceKey()
	return domainname.Fqdn(s.Name, s.Namespace)
}

func (o *LoginSet) ServiceFQDNShort() string {
	s := o.ServiceKey()
	return domainname.FqdnShort(s.Name, s.Namespace)
}

func (o *LoginSet) SssdSecretKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Spec.SssdConfRef.Name,
		Namespace: o.Namespace,
	}
}

func (o *LoginSet) SssdSecretRef() *corev1.SecretKeySelector {
	key := o.SssdSecretKey()
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: key.Name,
		},
		Key: o.Spec.SssdConfRef.Key,
	}
}

func (o *LoginSet) SshConfigKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-ssh-config", o.Name),
		Namespace: o.Namespace,
	}
}

func (o *LoginSet) SshHostKeys() types.NamespacedName {
	key := o.Key()
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-ssh-host-keys", key.Name),
		Namespace: o.Namespace,
	}
}
