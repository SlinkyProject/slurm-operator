// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newToken(name, namespace string) *Token {
	return &Token{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestTokenKey(t *testing.T) {
	tk := newToken("my-token", "slinky")
	want := types.NamespacedName{Name: "my-token", Namespace: "slinky"}
	if got := tk.Key(); got != want {
		t.Errorf("Key() = %v, want %v", got, want)
	}
}

func TestTokenUsername(t *testing.T) {
	t.Run("defaults to nobody when unset", func(t *testing.T) {
		tk := newToken("my-token", "slinky")
		if got, want := tk.Username(), "nobody"; got != want {
			t.Errorf("Username() = %q, want %q", got, want)
		}
	})

	t.Run("uses explicit username when set", func(t *testing.T) {
		tk := newToken("my-token", "slinky")
		tk.Spec.Username = "alice"
		if got, want := tk.Username(), "alice"; got != want {
			t.Errorf("Username() = %q, want %q", got, want)
		}
	})
}

func TestTokenLifetime(t *testing.T) {
	t.Run("defaults to 15 minutes when unset", func(t *testing.T) {
		tk := newToken("my-token", "slinky")
		if got, want := tk.Lifetime(), 15*time.Minute; got != want {
			t.Errorf("Lifetime() = %v, want %v", got, want)
		}
	})

	t.Run("uses explicit lifetime when set", func(t *testing.T) {
		tk := newToken("my-token", "slinky")
		tk.Spec.Lifetime = &metav1.Duration{Duration: 30 * time.Minute}
		if got, want := tk.Lifetime(), 30*time.Minute; got != want {
			t.Errorf("Lifetime() = %v, want %v", got, want)
		}
	})
}

func TestTokenJwtRef(t *testing.T) {
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
			tk := newToken("my-token", "slinky")
			tk.Spec.JwtKeyRef = tc.jwtRef
			tk.Spec.JwtHs256KeyRef = tc.hs256

			if got := tk.JwtRef(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("JwtRef() = %v, want %v", got, tc.want)
			}

			wantKey := types.NamespacedName{Name: tc.want.Name, Namespace: "slinky"}
			if got := tk.JwtKey(); got != wantKey {
				t.Errorf("JwtKey() = %v, want %v", got, wantKey)
			}

			// Deprecated aliases must delegate to the non-deprecated methods.
			if got := tk.JwtHs256Ref(); !reflect.DeepEqual(got, tk.JwtRef()) {
				t.Errorf("JwtHs256Ref() = %v, want %v", got, tk.JwtRef())
			}
			if got := tk.JwtHs256Key(); got != tk.JwtKey() {
				t.Errorf("JwtHs256Key() = %v, want %v", got, tk.JwtKey())
			}
		})
	}
}

func TestTokenSecretKey(t *testing.T) {
	t.Run("derives name from token name and username when no secretRef", func(t *testing.T) {
		tk := newToken("my-token", "slinky")
		tk.Spec.Username = "alice"
		want := types.NamespacedName{Name: "my-token-jwt-alice", Namespace: "slinky"}
		if got := tk.SecretKey(); got != want {
			t.Errorf("SecretKey() = %v, want %v", got, want)
		}
	})

	t.Run("uses secretRef name when set", func(t *testing.T) {
		tk := newToken("my-token", "slinky")
		tk.Spec.Username = "alice"
		tk.Spec.SecretRef = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "custom-secret"},
			Key:                  "MY_JWT",
		}
		want := types.NamespacedName{Name: "custom-secret", Namespace: "slinky"}
		if got := tk.SecretKey(); got != want {
			t.Errorf("SecretKey() = %v, want %v", got, want)
		}
	})
}

func TestTokenSecretRef(t *testing.T) {
	t.Run("defaults key to SLURM_JWT when no secretRef", func(t *testing.T) {
		tk := newToken("my-token", "slinky")
		tk.Spec.Username = "alice"
		want := corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "my-token-jwt-alice"},
			Key:                  "SLURM_JWT",
		}
		if got := tk.SecretRef(); !reflect.DeepEqual(got, want) {
			t.Errorf("SecretRef() = %v, want %v", got, want)
		}
	})

	t.Run("uses secretRef name and key when set", func(t *testing.T) {
		tk := newToken("my-token", "slinky")
		tk.Spec.Username = "alice"
		tk.Spec.SecretRef = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "custom-secret"},
			Key:                  "MY_JWT",
		}
		want := corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "custom-secret"},
			Key:                  "MY_JWT",
		}
		if got := tk.SecretRef(); !reflect.DeepEqual(got, want) {
			t.Errorf("SecretRef() = %v, want %v", got, want)
		}
	})
}
