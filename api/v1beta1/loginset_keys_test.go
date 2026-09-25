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

func newLoginSet(name, namespace string) *LoginSet {
	return &LoginSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestLoginSetKeys(t *testing.T) {
	ls := newLoginSet("login", "slinky")

	tests := []struct {
		name string
		got  types.NamespacedName
		want types.NamespacedName
	}{
		{
			name: "Key",
			got:  ls.Key(),
			want: types.NamespacedName{Name: "login", Namespace: "slinky"},
		},
		{
			name: "ServiceKey",
			got:  ls.ServiceKey(),
			want: types.NamespacedName{Name: "login", Namespace: "slinky"},
		},
		{
			name: "SshConfigKey",
			got:  ls.SshConfigKey(),
			want: types.NamespacedName{Name: "login-ssh-config", Namespace: "slinky"},
		},
		{
			name: "SshHostKeys",
			got:  ls.SshHostKeys(),
			want: types.NamespacedName{Name: "login-ssh-host-keys", Namespace: "slinky"},
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

func TestLoginSetServiceFQDNShort(t *testing.T) {
	ls := newLoginSet("login", "slinky")
	if got, want := ls.ServiceFQDNShort(), "login.slinky"; got != want {
		t.Errorf("ServiceFQDNShort() = %q, want %q", got, want)
	}
}

func TestLoginSetSssdSecret(t *testing.T) {
	ls := newLoginSet("login", "slinky")
	ls.Spec.SssdConfRef = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "sssd-conf"},
		Key:                  "sssd.conf",
	}

	wantKey := types.NamespacedName{Name: "sssd-conf", Namespace: "slinky"}
	if got := ls.SssdSecretKey(); got != wantKey {
		t.Errorf("SssdSecretKey() = %v, want %v", got, wantKey)
	}

	wantRef := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "sssd-conf"},
		Key:                  "sssd.conf",
	}
	if got := ls.SssdSecretRef(); !reflect.DeepEqual(got, wantRef) {
		t.Errorf("SssdSecretRef() = %v, want %v", got, wantRef)
	}
}
