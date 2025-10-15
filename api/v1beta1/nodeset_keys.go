// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (o *NodeSet) Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Name,
		Namespace: o.Namespace,
	}
}

func (o *NodeSet) HeadlessServiceKey() types.NamespacedName {
	key := o.Key()
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-headless", key.Name),
		Namespace: o.Namespace,
	}
}

func (o *NodeSet) SssdSecretKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Spec.Ssh.SssdConfRef.Name,
		Namespace: o.Namespace,
	}
}

func (o *NodeSet) SssdSecretRef() *corev1.SecretKeySelector {
	key := o.SssdSecretKey()
	return &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: key.Name,
		},
		Key: o.Spec.Ssh.SssdConfRef.Key,
	}
}

func (o *NodeSet) SshConfigKey() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-ssh-config", o.Name),
		Namespace: o.Namespace,
	}
}
