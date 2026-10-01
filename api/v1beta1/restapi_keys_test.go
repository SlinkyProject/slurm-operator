// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newRestApi(name, namespace string) *RestApi {
	return &RestApi{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestRestApiKeys(t *testing.T) {
	r := newRestApi("slurm", "slinky")

	tests := []struct {
		name string
		got  types.NamespacedName
		want types.NamespacedName
	}{
		{
			name: "Key",
			got:  r.Key(),
			want: types.NamespacedName{Name: "slurm-restapi", Namespace: "slinky"},
		},
		{
			name: "ServiceKey",
			got:  r.ServiceKey(),
			want: types.NamespacedName{Name: "slurm-restapi", Namespace: "slinky"},
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

func TestRestApiServiceFQDN(t *testing.T) {
	r := newRestApi("slurm", "slinky")

	if got, want := r.ServiceFQDNShort(), "slurm-restapi.slinky"; got != want {
		t.Errorf("ServiceFQDNShort() = %q, want %q", got, want)
	}
	if got, prefix := r.ServiceFQDN(), "slurm-restapi.slinky.svc."; !strings.HasPrefix(got, prefix) {
		t.Errorf("ServiceFQDN() = %q, want prefix %q", got, prefix)
	}
}
