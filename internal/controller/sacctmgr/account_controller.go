// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package sacctmgr

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/controller/sacctmgr/slurmcontrol"
)

const (
	AccountControllerName = "account-controller"
	AccountFinalizer      = "slinky.slurm.net/account-finalizer"
	ConditionReady        = "Ready"
)

// AccountReconciler reconciles an Account object
type AccountReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	control       slurmcontrol.AccountingControlInterface
	eventRecorder events.EventRecorder
}

// conditionEventType maps a Ready condition to the Kubernetes event type used
// when emitting an event for a condition transition.
func conditionEventType(cond metav1.Condition) string {
	if cond.Status == metav1.ConditionTrue {
		return corev1.EventTypeNormal
	}
	switch cond.Reason {
	case "SyncFailed", "NoClient":
		return corev1.EventTypeWarning
	default:
		return corev1.EventTypeNormal
	}
}

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=accounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=accounts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=accounts/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AccountReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started syncing Account", "request", req)

	err := r.Sync(ctx, req)
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *AccountReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.eventRecorder = mgr.GetEventRecorder(AccountControllerName)
	return ctrl.NewControllerManagedBy(mgr).
		For(&slinkyv1beta1.Account{}).
		Complete(r)
}

func NewAccountReconciler(c client.Client, cm *clientmap.ClientMap) *AccountReconciler {
	return &AccountReconciler{
		Client:        c,
		Scheme:        c.Scheme(),
		control:       slurmcontrol.NewAccountingControl(cm),
		eventRecorder: events.NewFakeRecorder(100),
	}
}
