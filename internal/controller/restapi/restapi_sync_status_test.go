// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
)

func TestRestapiReconciler_applyAvailableCondition(t *testing.T) {
	restapi := &slinkyv1beta1.RestApi{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  corev1.NamespaceDefault,
			Name:       "slurm",
			Generation: 3,
		},
	}

	newDeployment := func(available int32) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: restapi.Key().Namespace,
				Name:      restapi.Key().Name,
			},
			Status: appsv1.DeploymentStatus{
				AvailableReplicas: available,
			},
		}
	}

	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		wantStatus metav1.ConditionStatus
		wantReason string
	}{
		{
			name:       "Deployment not created yet",
			deployment: nil,
			wantStatus: metav1.ConditionFalse,
			wantReason: "DeploymentNotFound",
		},
		{
			name:       "Deployment has no available replicas",
			deployment: newDeployment(0),
			wantStatus: metav1.ConditionFalse,
			wantReason: "NoReplicasAvailable",
		},
		{
			name:       "Deployment has an available replica",
			deployment: newDeployment(1),
			wantStatus: metav1.ConditionTrue,
			wantReason: "MinimumReplicasAvailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs := []client.Object{}
			if tt.deployment != nil {
				objs = append(objs, tt.deployment)
			}
			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(objs...).
				Build()
			r := &RestapiReconciler{Client: kubeClient}

			conditions := []metav1.Condition{}
			require.NoError(t, r.applyAvailableCondition(t.Context(), restapi, &conditions))

			condition := meta.FindStatusCondition(conditions, slurmconditions.RestApiConditionAvailable)
			require.NotNil(t, condition)
			require.Equal(t, tt.wantStatus, condition.Status)
			require.Equal(t, tt.wantReason, condition.Reason)
			require.Equal(t, restapi.Generation, condition.ObservedGeneration)
		})
	}

	// The RestApi controller reconciles on its own CR, so a condition that is not a pure function
	// of observed state would write status every pass and reconcile itself in a loop.
	t.Run("Repeated application is stable", func(t *testing.T) {
		kubeClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(newDeployment(1)).
			Build()
		r := &RestapiReconciler{Client: kubeClient}

		conditions := []metav1.Condition{}
		require.NoError(t, r.applyAvailableCondition(t.Context(), restapi, &conditions))
		require.Len(t, conditions, 1)
		first := conditions[0]

		require.NoError(t, r.applyAvailableCondition(t.Context(), restapi, &conditions))
		require.Len(t, conditions, 1)
		require.Equal(t, first, conditions[0])
	})
}
