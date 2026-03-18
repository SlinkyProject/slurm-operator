// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func (o *Token) Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Name,
		Namespace: o.Namespace,
	}
}

func (o *Token) Username() string {
	username := "nobody"
	if o.Spec.Username != "" {
		username = o.Spec.Username
	}
	return username
}

func (o *Token) Lifetime() time.Duration {
	lifetime := 15 * time.Minute
	if o.Spec.Lifetime != nil {
		lifetime = o.Spec.Lifetime.Duration
	}
	return lifetime
}

// Deprecated: use JwtKey() instead.
func (o *Token) JwtHs256Key() types.NamespacedName {
	return o.JwtKey()
}

// Deprecated: use JwtRef() instead.
func (o *Token) JwtHs256Ref() JwtSecretKeySelector {
	return o.JwtRef()
}

func (o *Token) JwtKey() types.NamespacedName {
	ref := o.JwtRef()
	return types.NamespacedName{
		Name:      ref.Name,
		Namespace: ref.Namespace,
	}
}

// NOTE: Return non-nil because this field is effectively required.
func (o *Token) JwtRef() JwtSecretKeySelector {
	var refPtr *JwtSecretKeySelector
	switch {
	case o.Spec.JwtKeyRef != nil:
		refPtr = o.Spec.JwtKeyRef
	case o.Spec.JwtHs256KeyRef != nil:
		refPtr = o.Spec.JwtHs256KeyRef
	}
	ref := ptr.Deref(refPtr, JwtSecretKeySelector{})
	if ref.Namespace == "" {
		ref.Namespace = o.Namespace
	}
	return ref
}

func (o *Token) SecretKey() types.NamespacedName {
	name := fmt.Sprintf("%s-jwt-%s", o.Name, o.Spec.Username)
	if o.Spec.SecretRef != nil {
		name = o.Spec.SecretRef.Name
	}
	return types.NamespacedName{
		Name:      name,
		Namespace: o.Namespace,
	}
}

func (o *Token) SecretRef() corev1.SecretKeySelector {
	name := o.SecretKey().Name
	key := "SLURM_JWT"
	if o.Spec.SecretRef != nil {
		key = o.Spec.SecretRef.Key
	}
	return corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name,
		},
		Key: key,
	}
}
