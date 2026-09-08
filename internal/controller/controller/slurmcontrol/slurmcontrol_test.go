// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	api "github.com/SlinkyProject/slurm-client/api/v0044"
	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/client/fake"
	"github.com/SlinkyProject/slurm-client/pkg/client/interceptor"
	"github.com/SlinkyProject/slurm-client/pkg/object"
	"github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

// orderedControllerPingClient returns a fake client whose ControllerPingList List() results
// are sorted by Hostname. GetActiveHAController picks the first *responding* entry in list
// order as Active, mirroring real slurmctld, which returns controllers in a fixed order (the
// HA backup index). The fake client's List() has no such ordering guarantee (backed by a Go
// map), so without this, which entry it treats as "first" is nondeterministic and this test
// flakes. See https://gitlab.com/nvidia/schedmd/slinky/slurm-client/-/merge_requests/188.
func orderedControllerPingClient(items []types.V0044ControllerPing) client.Client {
	base := fake.NewClientBuilder().
		WithLists(&types.V0044ControllerPingList{Items: items}).
		Build()
	return interceptor.NewClient(base, interceptor.Funcs{
		List: func(ctx context.Context, list object.ObjectList, opts ...client.ListOption) error {
			if err := base.List(ctx, list, opts...); err != nil {
				return err
			}
			if l, ok := list.(*types.V0044ControllerPingList); ok {
				sort.Slice(l.Items, func(i, j int) bool {
					return ptr.Deref(l.Items[i].Hostname, "") < ptr.Deref(l.Items[j].Hostname, "")
				})
			}
			return nil
		},
	})
}

func Test_realSlurmControl_GetActiveHAController(t *testing.T) {
	tests := []struct {
		name       string
		sclient    client.Client
		controller *slinkyv1beta1.Controller
		want       []ControllerPing
		wantErr    bool
	}{
		{
			name: "both responding",
			sclient: orderedControllerPingClient([]types.V0044ControllerPing{
				{V0044ControllerPing: newPing("controller-0", true, true)},
				{V0044ControllerPing: newPing("controller-1", false, true)},
			}),
			controller: &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: corev1.NamespaceDefault,
					Name:      "test",
				},
			},
			want: []ControllerPing{
				{Name: "controller-0", Active: true},
				{Name: "controller-1", Active: false},
			},
		},
		{
			name: "primary down",
			sclient: orderedControllerPingClient([]types.V0044ControllerPing{
				{V0044ControllerPing: newPing("controller-0", true, false)},
				{V0044ControllerPing: newPing("controller-1", false, true)},
			}),
			controller: &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: corev1.NamespaceDefault,
					Name:      "test",
				},
			},
			want: []ControllerPing{
				{Name: "controller-0", Active: false},
				{Name: "controller-1", Active: true},
			},
		},
		{
			name: "backup down",
			sclient: orderedControllerPingClient([]types.V0044ControllerPing{
				{V0044ControllerPing: newPing("controller-0", true, true)},
				{V0044ControllerPing: newPing("controller-1", false, false)},
			}),
			controller: &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: corev1.NamespaceDefault,
					Name:      "test",
				},
			},
			want: []ControllerPing{
				{Name: "controller-0", Active: true},
				{Name: "controller-1", Active: false},
			},
		},
		{
			name: "all down",
			sclient: orderedControllerPingClient([]types.V0044ControllerPing{
				{V0044ControllerPing: newPing("controller-0", true, false)},
				{V0044ControllerPing: newPing("controller-1", false, false)},
			}),
			controller: &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: corev1.NamespaceDefault,
					Name:      "test",
				},
			},
			want: []ControllerPing{
				{Name: "controller-0", Active: false},
				{Name: "controller-1", Active: false},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllerName := tt.controller.Name
			r := NewSlurmControl(testutils.NewClientMap(controllerName, tt.controller.Namespace, tt.sclient))
			got, gotErr := r.GetActiveHAController(t.Context(), tt.controller)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetActiveHAController() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetActiveHAController() succeeded unexpectedly")
			}
			require.ElementsMatch(t, tt.want, got)
		})
	}
}

func newPing(hostname string, isPrimary, isResponding bool) api.V0044ControllerPing {
	ping := api.V0044ControllerPing{
		Hostname:   new(hostname),
		Primary:    isPrimary,
		Responding: isResponding,
	}
	if isPrimary {
		ping.Mode = new("primary")
	} else {
		ping.Mode = new("backup")
	}
	if isResponding {
		ping.Pinged = new("UP")
	} else {
		ping.Pinged = new("DOWN")
	}
	return ping
}
