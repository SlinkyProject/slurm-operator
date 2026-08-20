// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/controller/controller/slurmcontrol"
)

type fakeSlurmControl struct {
	pings []slurmcontrol.ControllerPing
	err   error
}

func (f fakeSlurmControl) GetActiveHAController(context.Context, *slinkyv1beta1.Controller) ([]slurmcontrol.ControllerPing, error) {
	return f.pings, f.err
}

func TestControllerReconciler_syncHAStatus(t *testing.T) {
	newController := func(external bool) *slinkyv1beta1.Controller {
		return &slinkyv1beta1.Controller{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: corev1.NamespaceDefault,
				Name:      "slurm",
			},
			Spec: slinkyv1beta1.ControllerSpec{
				External: external,
				HighAvailability: slinkyv1beta1.ControllerHighAvailability{
					Enabled: true,
				},
			},
		}
	}
	newPod := func(controller *slinkyv1beta1.Controller, ordinal int, active bool) *corev1.Pod {
		podLabels := labels.NewBuilder().WithControllerSelectorLabels(controller).Build()
		if active {
			podLabels[slinkyv1beta1.LabelControllerActive] = "true"
		}
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: controller.Namespace,
				Name:      controller.PodName(ordinal),
				Labels:    podLabels,
			},
		}
	}
	activeOf := func(pods ...*corev1.Pod) map[string]bool {
		got := make(map[string]bool, len(pods))
		for _, pod := range pods {
			got[pod.Name] = pod.Labels[slinkyv1beta1.LabelControllerActive] == "true"
		}
		return got
	}

	tests := []struct {
		name       string
		controller *slinkyv1beta1.Controller
		pods       []*corev1.Pod
		pings      []slurmcontrol.ControllerPing
		pingErr    error
		wantActive map[string]bool
		wantErr    bool
	}{
		{
			name:       "External skips labeling",
			controller: newController(true),
			pods: []*corev1.Pod{
				newPod(newController(false), 0, true),
				newPod(newController(false), 1, false),
			},
			wantActive: map[string]bool{
				"slurm-controller-0": true,
				"slurm-controller-1": false,
			},
		},
		{
			name:       "Active backup gets label, primary loses it",
			controller: newController(false),
			pods: []*corev1.Pod{
				newPod(newController(false), 0, true),
				newPod(newController(false), 1, false),
			},
			pings: []slurmcontrol.ControllerPing{
				{Name: "node-a", Active: false},
				{Name: "node-b", Active: true},
			},
			wantActive: map[string]bool{
				"slurm-controller-0": false,
				"slurm-controller-1": true,
			},
		},
		{
			name:       "No active ping defaults to primary",
			controller: newController(false),
			pods: []*corev1.Pod{
				newPod(newController(false), 0, false),
				newPod(newController(false), 1, true),
			},
			pings: []slurmcontrol.ControllerPing{
				{Name: "node-a", Active: false},
				{Name: "node-b", Active: false},
			},
			wantActive: map[string]bool{
				"slurm-controller-0": true,
				"slurm-controller-1": false,
			},
		},
		{
			name:       "ErrNoSlurmClient defaults to primary",
			controller: newController(false),
			pods: []*corev1.Pod{
				newPod(newController(false), 0, false),
				newPod(newController(false), 1, true),
			},
			pingErr: slurmcontrol.ErrNoSlurmClient,
			wantActive: map[string]bool{
				"slurm-controller-0": true,
				"slurm-controller-1": false,
			},
		},
		{
			name:       "Slurm error is returned",
			controller: newController(false),
			pods: []*corev1.Pod{
				newPod(newController(false), 0, true),
			},
			pingErr: errors.New("internal error"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := make([]client.Object, 0, len(tt.pods))
			for _, pod := range tt.pods {
				objs = append(objs, pod.DeepCopy())
			}
			kubeClient := fake.NewClientBuilder().WithObjects(objs...).Build()
			r := &ControllerReconciler{
				Client: kubeClient,
				slurmControl: fakeSlurmControl{
					pings: tt.pings,
					err:   tt.pingErr,
				},
			}

			err := r.syncHAStatus(t.Context(), tt.controller)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			got := make([]*corev1.Pod, 0, len(tt.pods))
			for _, pod := range tt.pods {
				updated := &corev1.Pod{}
				require.NoError(t, kubeClient.Get(t.Context(), client.ObjectKeyFromObject(pod), updated))
				got = append(got, updated)
			}
			require.Equal(t, tt.wantActive, activeOf(got...))
		})
	}
}
