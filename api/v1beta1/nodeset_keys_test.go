// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newNodeSet(name, namespace string) *NodeSet {
	return &NodeSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestNodeSetKeys(t *testing.T) {
	ns := newNodeSet("gpu", "slinky")

	tests := []struct {
		name string
		got  types.NamespacedName
		want types.NamespacedName
	}{
		{
			name: "Key",
			got:  ns.Key(),
			want: types.NamespacedName{Name: "gpu", Namespace: "slinky"},
		},
		{
			name: "HeadlessServiceKey",
			got:  ns.HeadlessServiceKey(),
			want: types.NamespacedName{Name: "gpu-headless", Namespace: "slinky"},
		},
		{
			name: "SshConfigKey",
			got:  ns.SshConfigKey(),
			want: types.NamespacedName{Name: "gpu-ssh-config", Namespace: "slinky"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("got %v, want %v", tc.got, tc.want)
			}
		})
	}
}

func TestNodeSetSssdSecret(t *testing.T) {
	ns := newNodeSet("gpu", "slinky")
	ns.Spec.Ssh.SssdConfRef = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "sssd-conf"},
		Key:                  "sssd.conf",
	}

	wantKey := types.NamespacedName{Name: "sssd-conf", Namespace: "slinky"}
	if got := ns.SssdSecretKey(); got != wantKey {
		t.Errorf("SssdSecretKey() = %v, want %v", got, wantKey)
	}

	wantRef := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "sssd-conf"},
		Key:                  "sssd.conf",
	}
	if got := ns.SssdSecretRef(); !reflect.DeepEqual(got, wantRef) {
		t.Errorf("SssdSecretRef() = %v, want %v", got, wantRef)
	}
}
