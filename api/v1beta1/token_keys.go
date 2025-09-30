// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
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

func (o *Token) JwtHs256Key() types.NamespacedName {
	namespace := o.Spec.JwtHs256KeyRef.Namespace
	if namespace == "" {
		namespace = o.Namespace
	}
	return types.NamespacedName{
		Name:      o.Spec.JwtHs256KeyRef.Name,
		Namespace: namespace,
	}
}

func (o *Token) JwtHs256Ref() *JwtSecretKeySelector {
	ref := o.Spec.JwtHs256KeyRef
	if ref.Namespace == "" {
		ref.Namespace = o.Namespace
	}
	return &ref
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

func (o *Token) SecretRef() *corev1.SecretKeySelector {
	name := o.SecretKey().Name
	key := "SLURM_JWT"
	if o.Spec.SecretRef != nil {
		key = o.Spec.SecretRef.Key
	}
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name,
		},
		Key: key,
	}
}
