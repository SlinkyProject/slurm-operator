// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/controller/controller/slurmcontrol"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
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
			name:       "No client defaults to primary",
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
			// The active label is in the controller Service selector, so a failed ping must not
			// move traffic off the pod that is actually active.
			name:       "Ping failure preserves the existing active label",
			controller: newController(false),
			pods: []*corev1.Pod{
				newPod(newController(false), 0, false),
				newPod(newController(false), 1, true),
			},
			pingErr: errors.New("connection refused"),
			wantActive: map[string]bool{
				"slurm-controller-0": false,
				"slurm-controller-1": true,
			},
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
			}

			err := r.syncHAStatus(t.Context(), tt.controller, tt.pings, tt.pingErr)
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

func newStatusController() *slinkyv1beta1.Controller {
	return &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  corev1.NamespaceDefault,
			Name:       "slurm",
			Generation: 2,
		},
	}
}

func newCountingClient(statusWrites *int, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(objs...).
		WithStatusSubresource(&slinkyv1beta1.Controller{}).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(
				ctx context.Context,
				c client.Client,
				subResourceName string,
				obj client.Object,
				opts ...client.SubResourceUpdateOption,
			) error {
				*statusWrites++
				return c.SubResource(subResourceName).Update(ctx, obj, opts...)
			},
		}).
		Build()
}

func TestControllerReconciler_syncControllerStatus(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		wantStatus metav1.ConditionStatus
		wantReason string
	}{
		{
			name:       "Slurm answered",
			pingErr:    nil,
			wantStatus: metav1.ConditionTrue,
			wantReason: "Reachable",
		},
		{
			name:       "No client registered yet",
			pingErr:    slurmcontrol.ErrNoSlurmClient,
			wantStatus: metav1.ConditionFalse,
			wantReason: "NoSlurmClient",
		},
		{
			name:       "Slurm did not respond",
			pingErr:    errors.New("connection refused"),
			wantStatus: metav1.ConditionFalse,
			wantReason: "Unreachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := newStatusController()
			var statusWrites int
			kubeClient := newCountingClient(&statusWrites, controller.DeepCopy())
			r := &ControllerReconciler{Client: kubeClient}

			require.NoError(t, r.syncControllerStatus(t.Context(), controller, tt.pingErr))

			require.Equal(t, 1, statusWrites, "expected exactly one status write")

			updated := &slinkyv1beta1.Controller{}
			require.NoError(t, kubeClient.Get(t.Context(), client.ObjectKeyFromObject(controller), updated))
			condition := meta.FindStatusCondition(updated.Status.Conditions,
				slurmconditions.ControllerConditionSlurmReachable)
			require.NotNil(t, condition, "SlurmReachable condition should be set")
			require.Equal(t, tt.wantStatus, condition.Status)
			require.Equal(t, tt.wantReason, condition.Reason)
			require.Equal(t, controller.Generation, condition.ObservedGeneration)
		})
	}
}

func TestControllerReconciler_syncControllerStatus_UnchangedStateDoesNotWrite(t *testing.T) {
	tests := []struct {
		name       string
		firstPing  error
		secondPing error
	}{
		{
			name: "still reachable",
		},
		{
			// Distinct error values must still collapse to one write. A reason-derived message
			// keeps status quiescent; interpolating the error text would rewrite status, and so
			// wake every NodeSet referencing this Controller, on every reconcile.
			name:       "still unreachable, different errors",
			firstPing:  errors.New("connection refused"),
			secondPing: errors.New("i/o timeout"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := newStatusController()
			var statusWrites int
			kubeClient := newCountingClient(&statusWrites, controller.DeepCopy())
			r := &ControllerReconciler{Client: kubeClient}

			require.NoError(t, r.syncControllerStatus(t.Context(), controller, tt.firstPing))
			require.Equal(t, 1, statusWrites, "first pass should publish the condition")

			persisted := &slinkyv1beta1.Controller{}
			require.NoError(t, kubeClient.Get(t.Context(), client.ObjectKeyFromObject(controller), persisted))

			require.NoError(t, r.syncControllerStatus(t.Context(), persisted, tt.secondPing))
			require.Equal(t, 1, statusWrites, "an unchanged condition should not rewrite status")
		})
	}
}

func TestControllerReconciler_syncControllerStatus_RecoveryEmitsStatusWrite(t *testing.T) {
	controller := newStatusController()
	var statusWrites int
	kubeClient := newCountingClient(&statusWrites, controller.DeepCopy())
	r := &ControllerReconciler{Client: kubeClient}

	require.NoError(t, r.syncControllerStatus(t.Context(), controller, errors.New("connection refused")))
	unreachable := &slinkyv1beta1.Controller{}
	require.NoError(t, kubeClient.Get(t.Context(), client.ObjectKeyFromObject(controller), unreachable))
	require.Equal(t, metav1.ConditionFalse, meta.FindStatusCondition(unreachable.Status.Conditions,
		slurmconditions.ControllerConditionSlurmReachable).Status)

	require.NoError(t, r.syncControllerStatus(t.Context(), unreachable, nil))

	require.Equal(t, 2, statusWrites,
		"becoming reachable must write status so NodeSets watching this Controller are enqueued")

	recovered := &slinkyv1beta1.Controller{}
	require.NoError(t, kubeClient.Get(t.Context(), client.ObjectKeyFromObject(controller), recovered))
	require.Equal(t, metav1.ConditionTrue, meta.FindStatusCondition(recovered.Status.Conditions,
		slurmconditions.ControllerConditionSlurmReachable).Status)
}

// An external Slurm is the case most prone to the startup race this condition exists for, since
// the operator does not control when it comes up. syncStatus therefore pings before syncHAStatus
// short-circuits on External, which is a deliberate extra call for external deployments.
func TestControllerReconciler_syncStatus_ExternalStillReportsReachability(t *testing.T) {
	controller := newStatusController()
	controller.Spec.External = true

	var statusWrites int
	kubeClient := newCountingClient(&statusWrites, controller.DeepCopy())
	r := &ControllerReconciler{
		Client:       kubeClient,
		slurmControl: fakeSlurmControl{},
	}

	require.NoError(t, r.syncStatus(t.Context(), controller))

	updated := &slinkyv1beta1.Controller{}
	require.NoError(t, kubeClient.Get(t.Context(), client.ObjectKeyFromObject(controller), updated))
	condition := meta.FindStatusCondition(updated.Status.Conditions,
		slurmconditions.ControllerConditionSlurmReachable)
	require.NotNil(t, condition, "external Controllers must still report reachability")
	require.Equal(t, metav1.ConditionTrue, condition.Status)
}

func TestControllerReconciler_syncStatus_PingFailureIsNotFatal(t *testing.T) {
	controller := newStatusController()
	var statusWrites int
	kubeClient := newCountingClient(&statusWrites, controller.DeepCopy())
	r := &ControllerReconciler{
		Client:       kubeClient,
		slurmControl: fakeSlurmControl{err: errors.New("connection refused")},
	}

	require.NoError(t, r.syncStatus(t.Context(), controller))
	require.Equal(t, 1, statusWrites, "the condition must still be published")

	updated := &slinkyv1beta1.Controller{}
	require.NoError(t, kubeClient.Get(t.Context(), client.ObjectKeyFromObject(controller), updated))
	condition := meta.FindStatusCondition(updated.Status.Conditions,
		slurmconditions.ControllerConditionSlurmReachable)
	require.NotNil(t, condition)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Equal(t, "Unreachable", condition.Reason)
}
