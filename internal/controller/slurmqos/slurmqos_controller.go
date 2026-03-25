// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmqos

import (
	"context"
	"flag"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
)

const (
	ControllerName = "slurmqos-controller"

	finalizerName = "slinky.slurm.net/slurmqos-finalizer"
)

func init() {
	flag.IntVar(&maxConcurrentReconciles, "slurmqos-workers", maxConcurrentReconciles, "Max concurrent workers for SlurmQos controller.")
}

var (
	maxConcurrentReconciles = 1
)

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=slurmqos,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=slurmqos/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=slurmqos/finalizers,verbs=update
// +kubebuilder:rbac:groups=slinky.slurm.net,resources=controllers,verbs=get;list;watch

// SlurmQosReconciler reconciles a SlurmQos object
type SlurmQosReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ClientMap *clientmap.ClientMap
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SlurmQosReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, retErr error) {
	logger := log.FromContext(ctx)

	logger.Info("Started syncing SlurmQos", "request", req)

	startTime := time.Now()
	defer func() {
		if retErr == nil {
			logger.Info("Finished syncing SlurmQos", "duration", time.Since(startTime))
		} else {
			logger.Info("Finished syncing SlurmQos", "duration", time.Since(startTime), "error", retErr)
		}
	}()

	retErr = r.Sync(ctx, req)
	if retErr != nil {
		logger.Error(retErr, "encountered an error while reconciling request", "request", req)
	}
	return res, retErr
}

// SetupWithManager sets up the controller with the Manager.
func (r *SlurmQosReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&slinkyv1beta1.SlurmQos{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		}).
		Complete(r)
}

func NewReconciler(c client.Client, cm *clientmap.ClientMap) *SlurmQosReconciler {
	s := c.Scheme()
	if cm == nil {
		panic("ClientMap cannot be nil")
	}
	return &SlurmQosReconciler{
		Client: c,
		Scheme: s,

		ClientMap: cm,
	}
}
