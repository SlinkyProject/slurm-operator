// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
)

func TestAccountingReconciler_applyAvailableCondition(t *testing.T) {
	newAccounting := func(external bool) *slinkyv1beta1.Accounting {
		return &slinkyv1beta1.Accounting{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:  corev1.NamespaceDefault,
				Name:       "slurm",
				Generation: 4,
			},
			Spec: slinkyv1beta1.AccountingSpec{
				External: external,
			},
		}
	}

	newStatefulSet := func(accounting *slinkyv1beta1.Accounting, ready int32) *appsv1.StatefulSet {
		return &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: accounting.Key().Namespace,
				Name:      accounting.Key().Name,
			},
			Status: appsv1.StatefulSetStatus{
				ReadyReplicas: ready,
			},
		}
	}

	tests := []struct {
		name          string
		external      bool
		readyReplicas *int32
		wantCondition bool
		wantStatus    metav1.ConditionStatus
		wantReason    string
	}{
		{
			name:          "StatefulSet not created yet",
			wantCondition: true,
			wantStatus:    metav1.ConditionFalse,
			wantReason:    "StatefulSetNotFound",
		},
		{
			name:          "StatefulSet has no ready replicas",
			readyReplicas: ptr.To(int32(0)),
			wantCondition: true,
			wantStatus:    metav1.ConditionFalse,
			wantReason:    "NoReplicasAvailable",
		},
		{
			name:          "StatefulSet has a ready replica",
			readyReplicas: ptr.To(int32(1)),
			wantCondition: true,
			wantStatus:    metav1.ConditionTrue,
			wantReason:    "MinimumReplicasAvailable",
		},
		{
			name:          "External accounting has no condition",
			external:      true,
			wantCondition: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accounting := newAccounting(tt.external)

			objs := []client.Object{}
			if tt.readyReplicas != nil {
				objs = append(objs, newStatefulSet(accounting, *tt.readyReplicas))
			}
			kubeClient := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(objs...).
				Build()
			r := &AccountingReconciler{Client: kubeClient}

			conditions := []metav1.Condition{}
			require.NoError(t, r.applyAvailableCondition(t.Context(), accounting, &conditions))

			condition := meta.FindStatusCondition(conditions, slurmconditions.AccountingConditionAvailable)
			if !tt.wantCondition {
				require.Nil(t, condition)
				return
			}
			require.NotNil(t, condition)
			require.Equal(t, tt.wantStatus, condition.Status)
			require.Equal(t, tt.wantReason, condition.Reason)
			require.Equal(t, accounting.Generation, condition.ObservedGeneration)
		})
	}

	// The Accounting controller reconciles on its own CR, so a condition that is not a pure
	// function of observed state would write status every pass and reconcile itself in a loop.
	t.Run("Repeated application is stable", func(t *testing.T) {
		accounting := newAccounting(false)
		kubeClient := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(newStatefulSet(accounting, 1)).
			Build()
		r := &AccountingReconciler{Client: kubeClient}

		conditions := []metav1.Condition{}
		require.NoError(t, r.applyAvailableCondition(t.Context(), accounting, &conditions))
		require.Len(t, conditions, 1)
		first := conditions[0]

		require.NoError(t, r.applyAvailableCondition(t.Context(), accounting, &conditions))
		require.Len(t, conditions, 1)
		require.Equal(t, first, conditions[0])
	})
}

func TestAccountingReconciler_applyAvailableCondition_ExternalRemovesStaleCondition(t *testing.T) {
	accounting := &slinkyv1beta1.Accounting{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "slurm",
		},
		Spec: slinkyv1beta1.AccountingSpec{External: true},
	}

	conditions := []metav1.Condition{
		{
			Type:               slurmconditions.AccountingConditionAvailable,
			Status:             metav1.ConditionTrue,
			Reason:             "MinimumReplicasAvailable",
			LastTransitionTime: metav1.Now(),
		},
	}

	kubeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	r := &AccountingReconciler{Client: kubeClient}

	require.NoError(t, r.applyAvailableCondition(t.Context(), accounting, &conditions))
	require.Nil(t, meta.FindStatusCondition(conditions, slurmconditions.AccountingConditionAvailable))
}
