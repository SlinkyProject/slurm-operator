// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"flag"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/flowcontrol"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	builder "github.com/SlinkyProject/slurm-operator/internal/builder/workerbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/eventhandler"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/indexes"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/podcontrol"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/slurmcontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/durationstore"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
)

const (
	ControllerName = "nodeset-controller"

	// BackoffGCInterval is the time that has to pass before next iteration of backoff GC is run
	BackoffGCInterval = 1 * time.Minute
)

// Reasons for NodeSet events
const (
	// SelectingAllReason is added to an event when a NodeSet selects all Pods.
	SelectingAllReason = "SelectingAll"
	// FailedPlacementReason is added to an event when a NodeSet cannot schedule a Pod to a specified node.
	FailedPlacementReason = "FailedPlacement"
	// FailedNodeSetPodReason is added to an event when the status of a Pod of a NodeSet is 'Failed'.
	FailedNodeSetPodReason = "FailedNodeSetPod"
	// ScalingUpReason is added to an event when pods are being created to reach the desired replica count.
	ScalingUpReason = "ScalingUp"
	// ScalingDownReason is added to an event when pods are being deleted to reach the desired replica count.
	ScalingDownReason = "ScalingDown"
	// SyncFailedReason is added to an event when a sync sub-step fails.
	SyncFailedReason = "SyncFailed"
	// NodeCordonReason is added to an event when a pod is cordoned due to its Kubernetes node being cordoned.
	NodeCordonReason = "NodeCordon"
	// SlurmNodeNotRegisteredReason is added to an event when a pod is deleted because its Slurm node is not registered.
	SlurmNodeNotRegisteredReason = "SlurmNodeNotRegistered"
	// RollingUpdateReason is added to an event when pods are being replaced during a rolling update.
	RollingUpdateReason = "RollingUpdate"
	// ControllerRefFailedReason is added to an event when the referenced Controller CR cannot be fetched.
	ControllerRefFailedReason = "ControllerRefFailed"
)

func init() {
	flag.IntVar(&maxConcurrentReconciles, "nodeset-workers", maxConcurrentReconciles, "Max concurrent workers for NodeSet controller.")
}

var (
	maxConcurrentReconciles = 1

	// this is a short cut for any sub-functions to notify the reconcile how long to wait to requeue
	durationStore = durationstore.NewDurationStore(durationstore.Greater)

	onceBackoffGC     sync.Once
	failedPodsBackoff = flowcontrol.NewBackOff(1*time.Second, 15*time.Minute)
)

// NodeSetReconciler reconciles a NodeSet object
type NodeSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ClientMap *clientmap.ClientMap

	propagatedNodeConditions []corev1.NodeConditionType

	builder        *builder.WorkerBuilder
	refResolver    *refresolver.RefResolver
	podControl     podcontrol.PodControlInterface
	slurmControl   slurmcontrol.SlurmControlInterface
	historyControl historycontrol.HistoryControlInterface
	eventRecorder  events.EventRecorder
	expectations   *kubecontroller.UIDTrackingControllerExpectations
}

//+kubebuilder:rbac:groups=slinky.slurm.net,resources=nodesets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=slinky.slurm.net,resources=nodesets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=slinky.slurm.net,resources=nodesets/finalizers,verbs=update
//+kubebuilder:rbac:groups=slinky.slurm.net,resources=controllers,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=apps,resources=controllerrevisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *NodeSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, retErr error) {
	logger := log.FromContext(ctx)

	logger.Info("Started syncing NodeSet", "request", req)

	onceBackoffGC.Do(func() {
		go wait.Until(failedPodsBackoff.GC, BackoffGCInterval, ctx.Done())
	})

	startTime := time.Now()
	defer func() {
		if retErr == nil {
			if res.RequeueAfter > 0 {
				logger.Info("Finished syncing NodeSet", "duration", time.Since(startTime), "result", res)
			} else {
				logger.Info("Finished syncing NodeSet", "duration", time.Since(startTime))
			}
		} else {
			logger.Info("Finished syncing NodeSet", "duration", time.Since(startTime), "error", retErr)
		}
		// clean the duration store
		_ = durationStore.Pop(req.String())
	}()

	retErr = r.Sync(ctx, req)
	res = reconcile.Result{
		RequeueAfter: durationStore.Pop(req.String()),
	}
	if retErr != nil {
		logger.Error(retErr, "encountered an error while reconciling request", "request", req)
	}
	return res, retErr
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.eventRecorder = mgr.GetEventRecorder(ControllerName)
	r.podControl = podcontrol.NewPodControl(r.Client, r.eventRecorder)
	podEventHandler := eventhandler.NewPodEventHandler(r.Client, r.expectations)
	if err := indexes.SetupWithManager(mgr); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(ControllerName).
		For(&slinkyv1beta1.NodeSet{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Watches(&corev1.Pod{}, podEventHandler).
		Watches(&corev1.Node{}, eventhandler.NewNodeEventHandler(r.Client)).
		Watches(&slinkyv1beta1.Controller{}, eventhandler.NewControllerEventHandler(r.Client)).
		Watches(&corev1.Secret{}, eventhandler.NewSecretEventHandler(r.Client)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
		}).
		Complete(r)
}

func NewReconciler(c client.Client, cm *clientmap.ClientMap, propagatedNodeConditions []corev1.NodeConditionType) *NodeSetReconciler {
	s := c.Scheme()
	er := events.NewFakeRecorder(100)
	if cm == nil {
		panic("ClientMap cannot be nil")
	}
	return &NodeSetReconciler{
		Client: c,
		Scheme: s,

		ClientMap: cm,

		propagatedNodeConditions: propagatedNodeConditions,

		builder:        builder.New(c),
		refResolver:    refresolver.New(c),
		historyControl: historycontrol.NewHistoryControl(c),
		podControl:     podcontrol.NewPodControl(c, er),
		slurmControl:   slurmcontrol.NewSlurmControl(cm),
		eventRecorder:  er,
		expectations:   kubecontroller.NewUIDTrackingControllerExpectations(kubecontroller.NewControllerExpectations()),
	}
}
