// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"errors"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	api "github.com/SlinkyProject/slurm-client/api/v0044"
	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/client/fake"
	"github.com/SlinkyProject/slurm-client/pkg/types"
	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/stretchr/testify/require"
)

func Test_tolerateError(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Nil",
			args: args{
				err: nil,
			},
			want: true,
		},
		{
			name: "Empty",
			args: args{
				err: errors.New(""),
			},
			want: false,
		},
		{
			name: "NotFound",
			args: args{
				err: errors.New(http.StatusText(http.StatusNotFound)),
			},
			want: true,
		},
		{
			name: "NoContent",
			args: args{
				err: errors.New(http.StatusText(http.StatusNoContent)),
			},
			want: true,
		},
		{
			name: "Forbidden",
			args: args{
				err: errors.New(http.StatusText(http.StatusForbidden)),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tolerateError(tt.args.err); got != tt.want {
				t.Errorf("tolerateError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newSlurmClientMap(controllerName string, client client.Client) *clientmap.ClientMap {
	cm := clientmap.NewClientMap()
	key := k8stypes.NamespacedName{
		Namespace: corev1.NamespaceDefault,
		Name:      controllerName,
	}
	cm.Add(key, client)
	return cm
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
			sclient: fake.NewClientBuilder().
				WithLists(&types.V0044ControllerPingList{
					Items: []types.V0044ControllerPing{
						{V0044ControllerPing: newPing("controller-0", true, true)},
						{V0044ControllerPing: newPing("controller-1", false, true)},
					},
				}).
				Build(),
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
			sclient: fake.NewClientBuilder().
				WithLists(&types.V0044ControllerPingList{
					Items: []types.V0044ControllerPing{
						{V0044ControllerPing: newPing("controller-0", true, false)},
						{V0044ControllerPing: newPing("controller-1", false, true)},
					},
				}).
				Build(),
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
			sclient: fake.NewClientBuilder().
				WithLists(&types.V0044ControllerPingList{
					Items: []types.V0044ControllerPing{
						{V0044ControllerPing: newPing("controller-0", true, true)},
						{V0044ControllerPing: newPing("controller-1", false, false)},
					},
				}).
				Build(),
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
			sclient: fake.NewClientBuilder().
				WithLists(&types.V0044ControllerPingList{
					Items: []types.V0044ControllerPing{
						{V0044ControllerPing: newPing("controller-0", true, false)},
						{V0044ControllerPing: newPing("controller-1", false, false)},
					},
				}).
				Build(),
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
			r := NewSlurmControl(newSlurmClientMap(controllerName, tt.sclient))
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
