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
	"k8s.io/utils/ptr"
)

func newController(name, namespace string) *Controller {
	return &Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestControllerKeys(t *testing.T) {
	c := newController("slurm", "slinky")

	tests := []struct {
		name string
		got  types.NamespacedName
		want types.NamespacedName
	}{
		{
			name: "Key",
			got:  c.Key(),
			want: types.NamespacedName{Name: "slurm-controller", Namespace: "slinky"},
		},
		{
			name: "ServiceKey",
			got:  c.ServiceKey(),
			want: types.NamespacedName{Name: "slurm-controller", Namespace: "slinky"},
		},
		{
			name: "ConfigKey",
			got:  c.ConfigKey(),
			want: types.NamespacedName{Name: "slurm-config", Namespace: "slinky"},
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

func TestControllerServiceFQDN(t *testing.T) {
	c := newController("slurm", "slinky")

	if got, want := c.ServiceFQDNShort(), "slurm-controller.slinky"; got != want {
		t.Errorf("ServiceFQDNShort() = %q, want %q", got, want)
	}

	// ServiceFQDN appends the cluster domain resolved from /etc/resolv.conf at
	// runtime, so only the stable prefix is asserted here.
	if got, prefix := c.ServiceFQDN(), "slurm-controller.slinky.svc."; !strings.HasPrefix(got, prefix) {
		t.Errorf("ServiceFQDN() = %q, want prefix %q", got, prefix)
	}
}

func TestControllerClusterName(t *testing.T) {
	t.Run("falls back to namespace_name when unset", func(t *testing.T) {
		c := newController("slurm", "slinky")
		if got, want := c.ClusterName(), "slinky_slurm"; got != want {
			t.Errorf("ClusterName() = %q, want %q", got, want)
		}
	})

	t.Run("uses explicit ClusterName when set", func(t *testing.T) {
		c := newController("slurm", "slinky")
		c.Spec.ClusterName = "my-cluster"
		if got, want := c.ClusterName(), "my-cluster"; got != want {
			t.Errorf("ClusterName() = %q, want %q", got, want)
		}
	})
}

func TestControllerPrimaryName(t *testing.T) {
	t.Run("derives ordinal-0 name when not external", func(t *testing.T) {
		c := newController("slurm", "slinky")
		if got, want := c.PrimaryName(), "slurm-controller-0"; got != want {
			t.Errorf("PrimaryName() = %q, want %q", got, want)
		}
	})

	t.Run("uses external host when external", func(t *testing.T) {
		c := newController("slurm", "slinky")
		c.Spec.External = true
		c.Spec.ExternalConfig = ExternalConfig{Host: "slurmctld.example.com", Port: 6817}
		if got, want := c.PrimaryName(), "slurmctld.example.com"; got != want {
			t.Errorf("PrimaryName() = %q, want %q", got, want)
		}
	})
}

func TestControllerPrimaryFQDN(t *testing.T) {
	c := newController("slurm", "slinky")
	if got, want := c.PrimaryFQDN(), "slurm-controller-0.slurm-controller-internal.slinky"; got != want {
		t.Errorf("PrimaryFQDN() = %q, want %q", got, want)
	}
}

func TestControllerAuthSlurm(t *testing.T) {
	c := newController("slurm", "slinky")
	c.Spec.SlurmKeyRef = corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "slurm-key"},
		Key:                  "slurm.key",
	}

	wantKey := types.NamespacedName{Name: "slurm-key", Namespace: "slinky"}
	if got := c.AuthSlurmKey(); got != wantKey {
		t.Errorf("AuthSlurmKey() = %v, want %v", got, wantKey)
	}
	if got := c.AuthSlurmRef(); !reflect.DeepEqual(got, c.Spec.SlurmKeyRef) {
		t.Errorf("AuthSlurmRef() = %v, want %v", got, c.Spec.SlurmKeyRef)
	}
}

func TestControllerAuthJwtRef(t *testing.T) {
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
			c := newController("slurm", "slinky")
			c.Spec.JwtKeyRef = tc.jwtRef
			c.Spec.JwtHs256KeyRef = tc.hs256

			if got := c.AuthJwtRef(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("AuthJwtRef() = %v, want %v", got, tc.want)
			}

			wantKey := types.NamespacedName{Name: tc.want.Name, Namespace: "slinky"}
			if got := c.AuthJwtKey(); got != wantKey {
				t.Errorf("AuthJwtKey() = %v, want %v", got, wantKey)
			}

			// Deprecated aliases must delegate to the non-deprecated methods.
			if got := c.AuthJwtHs256Ref(); !reflect.DeepEqual(got, c.AuthJwtRef()) {
				t.Errorf("AuthJwtHs256Ref() = %v, want %v", got, c.AuthJwtRef())
			}
			if got := c.AuthJwtHs256Key(); got != c.AuthJwtKey() {
				t.Errorf("AuthJwtHs256Key() = %v, want %v", got, c.AuthJwtKey())
			}
		})
	}
}

func TestControllerReplicas(t *testing.T) {
	tests := []struct {
		name     string
		external bool
		haEnable bool
		backups  *int32
		want     int32
	}{
		{
			name: "no HA defaults to 1",
			want: 1,
		},
		{
			name:     "external is always 1",
			external: true,
			haEnable: true,
			backups:  ptr.To[int32](5),
			want:     1,
		},
		{
			name:     "HA enabled with nil backups defaults to 1 backup",
			haEnable: true,
			want:     2,
		},
		{
			name:     "HA enabled with explicit backups",
			haEnable: true,
			backups:  ptr.To[int32](3),
			want:     4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newController("slurm", "slinky")
			c.Spec.External = tc.external
			c.Spec.HighAvailability.Enabled = tc.haEnable
			c.Spec.HighAvailability.Backups = tc.backups
			if got := c.Replicas(); got != tc.want {
				t.Errorf("Replicas() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestControllerPodName(t *testing.T) {
	t.Run("derives ordinal name when not external", func(t *testing.T) {
		c := newController("slurm", "slinky")
		if got, want := c.PodName(2), "slurm-controller-2"; got != want {
			t.Errorf("PodName(2) = %q, want %q", got, want)
		}
	})

	t.Run("uses external host regardless of ordinal", func(t *testing.T) {
		c := newController("slurm", "slinky")
		c.Spec.External = true
		c.Spec.ExternalConfig = ExternalConfig{Host: "slurmctld.example.com", Port: 6817}
		if got, want := c.PodName(3), "slurmctld.example.com"; got != want {
			t.Errorf("PodName(3) = %q, want %q", got, want)
		}
	})
}

func TestControllerPodFQDNShort(t *testing.T) {
	c := newController("slurm", "slinky")
	if got, want := c.PodFQDNShort(0), "slurm-controller-0.slurm-controller.slinky"; got != want {
		t.Errorf("PodFQDNShort(0) = %q, want %q", got, want)
	}
}

func TestControllerServiceInternalFQDN(t *testing.T) {
	c := newController("slurm", "slinky")

	if got, want := c.ServiceInternalFQDNShort(), "slurm-controller-internal.slinky"; got != want {
		t.Errorf("ServiceInternalFQDNShort() = %q, want %q", got, want)
	}

	// ServiceInternalFQDN appends the cluster domain resolved from
	// /etc/resolv.conf at runtime, so only the stable prefix is asserted here.
	if got, prefix := c.ServiceInternalFQDN(), "slurm-controller-internal.slinky.svc."; !strings.HasPrefix(got, prefix) {
		t.Errorf("ServiceInternalFQDN() = %q, want prefix %q", got, prefix)
	}
}

func TestControllerPodInternalFQDNShort(t *testing.T) {
	c := newController("slurm", "slinky")
	if got, want := c.PodInternalFQDNShort(0), "slurm-controller-0.slurm-controller-internal.slinky"; got != want {
		t.Errorf("PodInternalFQDNShort(0) = %q, want %q", got, want)
	}
}

func TestControllerServiceInternalKey(t *testing.T) {
	c := newController("slurm", "slinky")
	want := types.NamespacedName{Name: "slurm-controller-internal", Namespace: "slinky"}
	if got := c.ServiceInternalKey(); got != want {
		t.Errorf("ServiceInternalKey() = %v, want %v", got, want)
	}
}

func TestControllerAuthJwksKey(t *testing.T) {
	t.Run("nil ref yields empty name", func(t *testing.T) {
		c := newController("slurm", "slinky")
		want := types.NamespacedName{Name: "", Namespace: "slinky"}
		if got := c.AuthJwksKey(); got != want {
			t.Errorf("AuthJwksKey() = %v, want %v", got, want)
		}
	})

	t.Run("uses configmap ref name when set", func(t *testing.T) {
		c := newController("slurm", "slinky")
		c.Spec.JwksKeyRef = &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "jwks-cm"},
			Key:                  "jwks.json",
		}
		want := types.NamespacedName{Name: "jwks-cm", Namespace: "slinky"}
		if got := c.AuthJwksKey(); got != want {
			t.Errorf("AuthJwksKey() = %v, want %v", got, want)
		}
	})
}
