// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package sacctmgr

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/sacctmgr/slurmcontrol"
)

type fakeControl struct {
	slurmcontrol.AccountingControlInterface // embed for unimplemented methods
	applied                                 []*slinkyv1beta1.Account
	deleted                                 []*slinkyv1beta1.Account
	applyErr                                error
}

func (f *fakeControl) ApplyAccount(ctx context.Context, a *slinkyv1beta1.Account) error {
	f.applied = append(f.applied, a)
	return f.applyErr
}

func (f *fakeControl) DeleteAccount(ctx context.Context, a *slinkyv1beta1.Account) error {
	f.deleted = append(f.deleted, a)
	return nil
}

func TestAccountSync_CreatesAndSetsReady(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	_ = slinkyv1beta1.AddToScheme(scheme)

	account := &slinkyv1beta1.Account{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "research",
			Namespace: "default",
		},
		Spec: slinkyv1beta1.AccountSpec{
			ControllerRef: corev1.LocalObjectReference{Name: "slurm"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(account).
		WithStatusSubresource(account).
		Build()

	fc := &fakeControl{}
	r := &AccountReconciler{Client: c, Scheme: scheme, control: fc}

	err := r.Sync(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "research"},
	})
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}

	got := &slinkyv1beta1.Account{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "research"}, got); err != nil {
		t.Fatalf("failed to get account: %v", err)
	}

	if !controllerutil.ContainsFinalizer(got, AccountFinalizer) {
		t.Errorf("expected finalizer %q to be present", AccountFinalizer)
	}

	if len(fc.applied) != 1 {
		t.Errorf("expected ApplyAccount to be called once, got %d", len(fc.applied))
	}

	if !meta.IsStatusConditionTrue(got.Status.Conditions, ConditionReady) {
		t.Errorf("expected condition %q to be true, got %+v", ConditionReady, got.Status.Conditions)
	}
}
