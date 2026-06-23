// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package sacctmgr

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/controller/sacctmgr/slurmcontrol"
)

const (
	UserControllerName = "user-controller"
	UserFinalizer      = "slinky.slurm.net/user-finalizer"
	// requeue interval used when waiting for referenced accounts to exist.
	userAccountRetry = 10 * time.Second
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	control slurmcontrol.AccountingControlInterface
}

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=users/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started syncing User", "request", req)

	res, err := r.Sync(ctx, req)
	return res, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&slinkyv1beta1.User{}).
		Complete(r)
}

func NewUserReconciler(c client.Client, cm *clientmap.ClientMap) *UserReconciler {
	return &UserReconciler{
		Client:  c,
		Scheme:  c.Scheme(),
		control: slurmcontrol.NewAccountingControl(cm),
	}
}
