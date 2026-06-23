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

type fakeUserControl struct {
	slurmcontrol.AccountingControlInterface // embed for unimplemented methods
	applied                                 []*slinkyv1beta1.User
	accountReady                            bool
	existsErr                               error
}

func (f *fakeUserControl) ApplyUser(ctx context.Context, u *slinkyv1beta1.User) error {
	f.applied = append(f.applied, u)
	return nil
}

func (f *fakeUserControl) DeleteUser(ctx context.Context, u *slinkyv1beta1.User) error {
	return nil
}

func (f *fakeUserControl) AccountExists(ctx context.Context, u *slinkyv1beta1.User, account string) (bool, error) {
	return f.accountReady, f.existsErr
}

func newTestUser() *slinkyv1beta1.User {
	return &slinkyv1beta1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "alice", Namespace: "default"},
		Spec: slinkyv1beta1.UserSpec{
			ControllerRef:  corev1.LocalObjectReference{Name: "slurm"},
			DefaultAccount: "research",
			Associations:   []slinkyv1beta1.UserAssociation{{Account: "research"}},
		},
	}
}

func TestUserSync_GatesOnMissingAccount(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	_ = slinkyv1beta1.AddToScheme(scheme)

	user := newTestUser()

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(user).
		WithStatusSubresource(user).
		Build()

	fc := &fakeUserControl{accountReady: false}
	r := &UserReconciler{Client: c, Scheme: scheme, control: fc}

	res, err := r.Sync(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "alice"},
	})
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}

	if res.RequeueAfter <= 0 {
		t.Errorf("expected positive RequeueAfter, got %v", res.RequeueAfter)
	}

	if len(fc.applied) != 0 {
		t.Errorf("expected ApplyUser not to be called, got %d", len(fc.applied))
	}

	got := &slinkyv1beta1.User{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "alice"}, got); err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if !meta.IsStatusConditionPresentAndEqual(got.Status.Conditions, ConditionReady, metav1.ConditionFalse) {
		t.Errorf("expected condition %q to be false, got %+v", ConditionReady, got.Status.Conditions)
	}

	cond := meta.FindStatusCondition(got.Status.Conditions, ConditionReady)
	if cond == nil || cond.Reason != "AccountNotReady" {
		t.Errorf("expected reason %q, got %+v", "AccountNotReady", cond)
	}
}

func TestUserSync_AppliesWhenAccountsReady(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	_ = slinkyv1beta1.AddToScheme(scheme)

	user := newTestUser()

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(user).
		WithStatusSubresource(user).
		Build()

	fc := &fakeUserControl{accountReady: true}
	r := &UserReconciler{Client: c, Scheme: scheme, control: fc}

	res, err := r.Sync(ctx, reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "default", Name: "alice"},
	})
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	_ = res

	got := &slinkyv1beta1.User{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "alice"}, got); err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if !controllerutil.ContainsFinalizer(got, UserFinalizer) {
		t.Errorf("expected finalizer %q to be present", UserFinalizer)
	}

	if len(fc.applied) != 1 {
		t.Errorf("expected ApplyUser to be called once, got %d", len(fc.applied))
	}

	if !meta.IsStatusConditionTrue(got.Status.Conditions, ConditionReady) {
		t.Errorf("expected condition %q to be true, got %+v", ConditionReady, got.Status.Conditions)
	}
}
