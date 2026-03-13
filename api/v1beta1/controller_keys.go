// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

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
	return &o.Spec.SlurmKeyRef
}

// Deprecated: use AuthJwtKey() instead.
func (o *Controller) AuthJwtHs256Key() types.NamespacedName {
	return o.AuthJwtKey()
}

// Deprecated: use AuthJwtRef() instead.
func (o *Controller) AuthJwtHs256Ref() *corev1.SecretKeySelector {
	return o.AuthJwtRef()
}

func (o *Controller) AuthJwtKey() types.NamespacedName {
	refPtr := o.Spec.JwtHs256KeyRef
	if o.Spec.JwtKeyRef != nil {
		refPtr = o.Spec.JwtKeyRef
	}
	ref := ptr.Deref(refPtr, corev1.SecretKeySelector{})
	return types.NamespacedName{
		Name:      ref.Name,
		Namespace: o.Namespace,
	}
}

func (o *Controller) AuthJwtRef() *corev1.SecretKeySelector {
	refPtr := o.Spec.JwtHs256KeyRef
	if o.Spec.JwtKeyRef != nil {
		refPtr = o.Spec.JwtKeyRef
	}
	ref := ptr.Deref(refPtr, corev1.SecretKeySelector{})
	return &ref
}

func (o *Controller) AuthJwksKey() types.NamespacedName {
	ref := ptr.Deref(o.Spec.JwksKeyRef, corev1.ConfigMapKeySelector{})
	return types.NamespacedName{
		Name:      ref.Name,
		Namespace: o.Namespace,
	}
}

func (o *Controller) AuthJwksRef() *corev1.ConfigMapKeySelector {
	ref := ptr.Deref(o.Spec.JwksKeyRef, corev1.ConfigMapKeySelector{})
	return &ref
}

func (o *Controller) ConfigKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-config", o.Name),
		Namespace: o.Namespace,
	}
}
