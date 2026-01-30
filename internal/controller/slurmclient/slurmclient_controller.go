// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmclient

import (
	"context"
	"flag"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/clientmap"
	"github.com/togethercomputer/slurm-operator/internal/utils/durationstore"
	"github.com/togethercomputer/slurm-operator/internal/utils/refresolver"
)

const (
	ControllerName = "slurmclient-controller"

	// BackoffGCInterval is the time that has to pass before next iteration of backoff GC is run
	BackoffGCInterval = 1 * time.Minute
)

func init() {
	flag.IntVar(&maxConcurrentReconciles, "slurmclient-workers", maxConcurrentReconciles, "Max concurrent workers for SlurmClient controller.")
}

var (
	maxConcurrentReconciles = 1

	// this is a short cut for any sub-functions to notify the reconcile how long to wait to requeue
	durationStore = durationstore.NewDurationStore(durationstore.Greater)

	onceBackoffGC     sync.Once
	failedPodsBackoff = flowcontrol.NewBackOff(1*time.Second, 15*time.Minute)
)

// SlurmClientReconciler reconciles a SlurmClient object
type SlurmClientReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ClientMap *clientmap.ClientMap
	EventCh   chan event.GenericEvent

	refResolver   *refresolver.RefResolver
	eventRecorder record.EventRecorderLogger
}

// +kubebuilder:rbac:groups=slinky.slurm.net,resources=controllers,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *SlurmClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, retErr error) {
	logger := log.FromContext(ctx)
	logger.Info("Started syncing SlurmClient", "request", req)

	onceBackoffGC.Do(func() {
		go wait.Until(failedPodsBackoff.GC, BackoffGCInterval, ctx.Done())
	})

	startTime := time.Now()
	defer func() {
		if retErr == nil {
			if res.RequeueAfter > 0 {
				logger.Info("Finished syncing SlurmClient", "duration", time.Since(startTime), "result", res)
			} else {
				logger.Info("Finished syncing SlurmClient", "duration", time.Since(startTime))
			}
		} else {
			logger.Info("Finished syncing SlurmClient", "duration", time.Since(startTime), "error", retErr)
		}
		// clean the duration store
		_ = durationStore.Pop(req.Namespace)
	}()

	retErr = r.Sync(ctx, req)
	res = reconcile.Result{
		RequeueAfter: durationStore.Pop(req.String()),
	}
	return res, retErr
}

// SetupWithManager sets up the controller with the Manager.
func (r *SlurmClientReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&slinkyv1beta1.Controller{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		}).
		Complete(r)
}

func NewReconciler(c client.Client, cm *clientmap.ClientMap, ec chan event.GenericEvent) *SlurmClientReconciler {
	s := c.Scheme()
	es := corev1.EventSource{Component: ControllerName}
	if cm == nil {
		panic("ClientMap cannot be nil")
	}
	if ec == nil {
		panic("EventCh cannot be nil")
	}
	return &SlurmClientReconciler{
		Client: c,
		Scheme: s,

		ClientMap: cm,
		EventCh:   ec,

		refResolver:   refresolver.New(c),
		eventRecorder: record.NewBroadcaster().NewRecorder(s, es),
	}
}
