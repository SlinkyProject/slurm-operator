// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/SlinkyProject/slurm-operator/internal/utils/domainname"
)

func (o *Controller) ClusterName() string {
	if o.Spec.ClusterName != "" {
		return o.Spec.ClusterName
	}
	return fmt.Sprintf("%s_%s", o.Namespace, o.Name)
}

func (o *Controller) Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-controller", o.Name),
		Namespace: o.Namespace,
	}
}

func (o *Controller) PrimaryName() string {
	if o.Spec.External {
		return o.Spec.ExternalConfig.Host
	}
	key := o.Key()
	return fmt.Sprintf("%s-0", key.Name)
}

func (o *Controller) PrimaryFQDN() string {
	key := o.PrimaryName()
	svc := o.ServiceFQDNShort()
	return fmt.Sprintf("%s.%s", key, svc)
}

func (o *Controller) ServiceKey() types.NamespacedName {
	key := o.Key()
	return types.NamespacedName{
		Name:      key.Name,
		Namespace: o.Namespace,
	}
}

func (o *Controller) ServiceFQDN() string {
	s := o.ServiceKey()
	return domainname.Fqdn(s.Name, s.Namespace)
}

func (o *Controller) ServiceFQDNShort() string {
	s := o.ServiceKey()
	return domainname.FqdnShort(s.Name, s.Namespace)
}

func (o *Controller) AuthSlurmKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Spec.SlurmKeyRef.Name,
		Namespace: o.Namespace,
	}
}

func (o *Controller) AuthSlurmRef() *corev1.SecretKeySelector {
	ref := o.Spec.SlurmKeyRef
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: ref.Name,
		},
		Key: ref.Key,
	}
}

func (o *Controller) AuthJwtHs256Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Spec.JwtHs256KeyRef.Name,
		Namespace: o.Namespace,
	}
}

func (o *Controller) AuthJwtHs256Ref() *corev1.SecretKeySelector {
	ref := o.Spec.JwtHs256KeyRef
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: ref.Name,
		},
		Key: ref.Key,
	}
}

func (o *Controller) ConfigKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-config", o.Name),
		Namespace: o.Namespace,
	}
}

func (o *Controller) SssdSecretKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Spec.SssdConfRef.Name,
		Namespace: o.Namespace,
	}
}

func (o *Controller) SssdSecretRef() *corev1.SecretKeySelector {
	key := o.SssdSecretKey()
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: key.Name,
		},
		Key: o.Spec.SssdConfRef.Key,
	}
}
