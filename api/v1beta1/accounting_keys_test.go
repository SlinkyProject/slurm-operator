// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newAccounting(name, namespace string) *Accounting {
	return &Accounting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestAccountingKeys(t *testing.T) {
	a := newAccounting("slurm", "slinky")

	tests := []struct {
		name string
		got  types.NamespacedName
		want types.NamespacedName
	}{
		{
			name: "Key",
			got:  a.Key(),
			want: types.NamespacedName{Name: "slurm-accounting", Namespace: "slinky"},
		},
		{
			name: "ConfigKey",
			got:  a.ConfigKey(),
			want: types.NamespacedName{Name: "slurm-accounting", Namespace: "slinky"},
		},
		{
			name: "ServiceKey",
			got:  a.ServiceKey(),
			want: types.NamespacedName{Name: "slurm-accounting", Namespace: "slinky"},
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

func TestAccountingPrimaryName(t *testing.T) {
	a := newAccounting("slurm", "slinky")
	if got, want := a.PrimaryName(), "slurm-accounting-0"; got != want {
		t.Errorf("PrimaryName() = %q, want %q", got, want)
	}
}

func TestAccountingServiceFQDN(t *testing.T) {
	a := newAccounting("slurm", "slinky")

	if got, want := a.ServiceFQDNShort(), "slurm-accounting.slinky"; got != want {
		t.Errorf("ServiceFQDNShort() = %q, want %q", got, want)
	}
	if got, prefix := a.ServiceFQDN(), "slurm-accounting.slinky.svc."; !strings.HasPrefix(got, prefix) {
		t.Errorf("ServiceFQDN() = %q, want prefix %q", got, prefix)
	}
}

func TestAccountingAuthStorage(t *testing.T) {
	a := newAccounting("slurm", "slinky")
	a.Spec.StorageConfig.PasswordKeyRef = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "mariadb-password"},
		Key:                  "password",
	}

	wantKey := types.NamespacedName{Name: "mariadb-password", Namespace: "slinky"}
	if got := a.AuthStorageKey(); got != wantKey {
		t.Errorf("AuthStorageKey() = %v, want %v", got, wantKey)
	}

	wantRef := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "mariadb-password"},
		Key:                  "password",
	}
	if got := a.AuthStorageRef(); !reflect.DeepEqual(got, wantRef) {
		t.Errorf("AuthStorageRef() = %v, want %v", got, wantRef)
	}
}

func TestAccountingAuthSlurm(t *testing.T) {
	a := newAccounting("slurm", "slinky")
	a.Spec.SlurmKeyRef = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "slurm-key"},
		Key:                  "slurm.key",
	}

	wantKey := types.NamespacedName{Name: "slurm-key", Namespace: "slinky"}
	if got := a.AuthSlurmKey(); got != wantKey {
		t.Errorf("AuthSlurmKey() = %v, want %v", got, wantKey)
	}
	if got := a.AuthSlurmRef(); !reflect.DeepEqual(got, a.Spec.SlurmKeyRef) {
		t.Errorf("AuthSlurmRef() = %v, want %v", got, a.Spec.SlurmKeyRef)
	}
}

func TestAccountingAuthJwtRef(t *testing.T) {
	jwt := &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "jwt-key"},
		Key:                  "jwt_hs256.key",
	}
	hs256 := &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "hs256-key"},
		Key:                  "jwt_hs256.key",
	}

	tests := []struct {
		name   string
		jwtRef *corev1.SecretKeySelector
		hs256  *corev1.SecretKeySelector
		want   corev1.SecretKeySelector
	}{
		{
			name: "both unset returns empty selector",
			want: corev1.SecretKeySelector{},
		},
		{
			name:  "only deprecated hs256 set",
			hs256: hs256,
			want:  *hs256,
		},
		{
			name:   "jwtKeyRef takes precedence over hs256",
			jwtRef: jwt,
			hs256:  hs256,
			want:   *jwt,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newAccounting("slurm", "slinky")
			a.Spec.JwtKeyRef = tc.jwtRef
			a.Spec.JwtHs256KeyRef = tc.hs256

			if got := a.AuthJwtRef(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("AuthJwtRef() = %v, want %v", got, tc.want)
			}

			wantKey := types.NamespacedName{Name: tc.want.Name, Namespace: "slinky"}
			if got := a.AuthJwtKey(); got != wantKey {
				t.Errorf("AuthJwtKey() = %v, want %v", got, wantKey)
			}

			// Deprecated aliases must delegate to the non-deprecated methods.
			if got := a.AuthJwtHs256Ref(); !reflect.DeepEqual(got, a.AuthJwtRef()) {
				t.Errorf("AuthJwtHs256Ref() = %v, want %v", got, a.AuthJwtRef())
			}
			if got := a.AuthJwtHs256Key(); got != a.AuthJwtKey() {
				t.Errorf("AuthJwtHs256Key() = %v, want %v", got, a.AuthJwtKey())
			}
		})
	}
}

func TestAccountingAuthJwksKey(t *testing.T) {
	t.Run("nil ref yields empty name", func(t *testing.T) {
		a := newAccounting("slurm", "slinky")
		want := types.NamespacedName{Name: "", Namespace: "slinky"}
		if got := a.AuthJwksKey(); got != want {
			t.Errorf("AuthJwksKey() = %v, want %v", got, want)
		}
	})

	t.Run("uses configmap ref name when set", func(t *testing.T) {
		a := newAccounting("slurm", "slinky")
		a.Spec.JwksKeyRef = &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "jwks-cm"},
			Key:                  "jwks.json",
		}
		want := types.NamespacedName{Name: "jwks-cm", Namespace: "slinky"}
		if got := a.AuthJwksKey(); got != want {
			t.Errorf("AuthJwksKey() = %v, want %v", got, want)
		}
	})
}
