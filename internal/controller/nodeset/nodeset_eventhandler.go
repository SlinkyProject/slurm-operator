// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podinfo"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
)

var _ handler.EventHandler = &podEventHandler{}

type podEventHandler struct {
	client.Reader
	expectations *kubecontroller.UIDTrackingControllerExpectations
}

func (e *podEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	pod, ok := evt.Object.(*corev1.Pod)
	if !ok {
		return
	}
	e.createPod(ctx, pod, q)
}

func (e *podEventHandler) createPod(
	ctx context.Context,
	pod *corev1.Pod,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	if pod.DeletionTimestamp != nil {
		// on a restart of the controller manager, it's possible a new pod shows up in a state that
		// is already pending deletion. Prevent the pod from being a creation observation.
		e.deletePod(ctx, pod, q)
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(pod); controllerRef != nil {
		nodeset := e.resolveControllerRef(ctx, pod.Namespace, controllerRef)
		if nodeset == nil {
			return
		}
		nodesetKey, err := kubecontroller.KeyFunc(nodeset)
		if err != nil {
			return
		}
		logger.V(4).Info("Pod created", "pod", klog.KObj(pod), "detail", pod)
		e.expectations.CreationObserved(logger, nodesetKey)
		objectutils.EnqueueRequest(q, nodeset)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching NodeSets and sync
	// them to see if anyone wants to adopt it.
	// DO NOT observe creation because no controller should be waiting for an
	// orphan.
	nodesetList := e.getPodNodeSets(ctx, pod)
	if len(nodesetList) == 0 {
		return
	}
	logger.V(4).Info("Orphan Pod created", "pod", klog.KObj(pod), "detail", pod)
	for _, nodeset := range nodesetList {
		objectutils.EnqueueRequest(q, nodeset)
	}
}

func (e *podEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.updatePod(ctx, evt.ObjectNew, evt.ObjectOld, q)
}

// When a pod is updated, figure out what replica nodeset/s manage it and wake them
// up. If the labels of the pod have changed we need to awaken both the old
// and new replica nodeset. old and cur must be *corev1.Pod types.
func (e *podEventHandler) updatePod(
	ctx context.Context,
	cur, old any,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)
	curPod, ok := cur.(*corev1.Pod)
	if !ok {
		return
	}
	oldPod, ok := old.(*corev1.Pod)
	if !ok {
		return
	}

	if curPod.ResourceVersion == oldPod.ResourceVersion {
		// Periodic resync will send update events for all known pods.
		// Two different versions of the same pod will always have different RVs.
		return
	}

	labelChanged := !reflect.DeepEqual(curPod.Labels, oldPod.Labels)
	if curPod.DeletionTimestamp != nil {
		// when a pod is deleted gracefully it's deletion timestamp is first modified to reflect a grace period,
		// and after such time has passed, the kubelet actually deletes it from the store. We receive an update
		// for modification of the deletion timestamp and expect a nodeset to create more replicas asap, not wait
		// until the kubelet actually deletes the pod. This is different from the Phase of a pod changing, because
		// a nodeset never initiates a phase change, and so is never asleep waiting for the same.
		e.deletePod(ctx, curPod, q)
		if labelChanged {
			// we don't need to check the oldPod.DeletionTimestamp because DeletionTimestamp cannot be unset.
			e.deletePod(ctx, oldPod, q)
		}
		return
	}

	curControllerRef := metav1.GetControllerOf(curPod)
	oldControllerRef := metav1.GetControllerOf(oldPod)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller, if any.
		if nodeset := e.resolveControllerRef(ctx, oldPod.Namespace, oldControllerRef); nodeset != nil {
			objectutils.EnqueueRequest(q, nodeset)
		}
	}

	// If it has a ControllerRef, that's all that matters.
	if curControllerRef != nil {
		nodeset := e.resolveControllerRef(ctx, curPod.Namespace, curControllerRef)
		if nodeset == nil {
			return
		}
		logger.V(4).Info("Pod objectMeta updated.", "pod", klog.KObj(oldPod), "oldObjectMeta", oldPod.ObjectMeta, "curObjectMeta", curPod.ObjectMeta)
		objectutils.EnqueueRequest(q, nodeset)
		// TODO: MinReadySeconds in the Pod will generate an Available condition to be added in
		// the Pod status which in turn will trigger a requeue of the owning nodeset thus
		// having its status updated with the newly available replica. For now, we can fake the
		// update by resyncing the controller MinReadySeconds after the it is requeued because
		// a Pod transitioned to Ready.
		// Note that this still suffers from #29229, we are just moving the problem one level
		// "closer" to kubelet (from the deployment to the replica nodeset controller).
		if !podutil.IsPodReady(oldPod) && podutil.IsPodReady(curPod) && nodeset.Spec.MinReadySeconds > 0 {
			logger.V(2).Info("pod will be enqueued after a while for availability check", "duration", nodeset.Spec.MinReadySeconds, "kind", slinkyv1beta1.NodeSetGVK, "pod", klog.KObj(oldPod))
			requeueDuration := (time.Duration(nodeset.Spec.MinReadySeconds) * time.Second) + time.Second
			objectutils.EnqueueRequestAfter(q, nodeset, requeueDuration)
		}
		return
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now.
	if labelChanged || controllerRefChanged {
		nodesetList := e.getPodNodeSets(ctx, curPod)
		if len(nodesetList) == 0 {
			return
		}
		logger.V(4).Info("Orphan Pod objectMeta updated.", "pod", klog.KObj(oldPod), "oldObjectMeta", oldPod.ObjectMeta, "curObjectMeta", curPod.ObjectMeta)
		for _, nodeset := range nodesetList {
			objectutils.EnqueueRequest(q, nodeset)
		}
	}
}

func (e *podEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.deletePod(ctx, evt.Object, q)
}

// When a pod is deleted, enqueue the replica nodeset that manages the pod and update its expectations.
// obj could be an *corev1.Pod, or a DeletionFinalStateUnknown marker item.
func (e *podEventHandler) deletePod(
	ctx context.Context,
	obj any,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)
	pod, ok := obj.(*corev1.Pod)

	// When a delete is dropped, the relist will notice a pod in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the pod
	// changed labels the new ReplicaSet will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		pod, ok = tombstone.Obj.(*corev1.Pod)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a pod %#v", obj))
			return
		}
	}

	controllerRef := metav1.GetControllerOf(pod)
	if controllerRef == nil {
		// No controller should care about orphans being deleted.
		return
	}
	nodeset := e.resolveControllerRef(ctx, pod.Namespace, controllerRef)
	if nodeset == nil {
		return
	}
	nodesetKey, err := kubecontroller.KeyFunc(nodeset)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %w", nodeset, err))
		return
	}
	logger.V(4).Info("Pod deleted", "delete_by", utilruntime.GetCaller(), "deletion_timestamp", pod.DeletionTimestamp, "pod", klog.KObj(pod))
	e.expectations.DeletionObserved(logger, nodesetKey, kubecontroller.PodKey(pod))
	objectutils.EnqueueRequest(q, nodeset)
}

func (e *podEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	pod, ok := evt.Object.(*corev1.Pod)
	if !ok {
		return
	}
	namespacedName := types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}
	if err := e.Get(ctx, namespacedName, pod); err != nil {
		return
	}

	nodesetList := e.getPodNodeSets(ctx, pod)
	for _, nodeset := range nodesetList {
		if !nodesetutils.IsPodFromNodeSet(nodeset, pod) {
			continue
		}
		objectutils.EnqueueRequest(q, nodeset)
	}
}

func (e *podEventHandler) resolveControllerRef(
	ctx context.Context,
	namespace string,
	controllerRef *metav1.OwnerReference,
) *slinkyv1beta1.NodeSet {
	if controllerRef.Kind != slinkyv1beta1.NodeSetKind || controllerRef.APIVersion != slinkyv1beta1.NodeSetAPIVersion {
		return nil
	}

	nodeset := &slinkyv1beta1.NodeSet{}
	key := types.NamespacedName{Namespace: namespace, Name: controllerRef.Name}
	if err := e.Get(ctx, key, nodeset); err != nil {
		return nil
	}
	if nodeset.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return nodeset
}

func (e *podEventHandler) getPodNodeSets(ctx context.Context, pod *corev1.Pod) []*slinkyv1beta1.NodeSet {
	logger := log.FromContext(ctx)
	nodesetList := slinkyv1beta1.NodeSetList{}
	if err := e.List(ctx, &nodesetList, client.InNamespace(pod.Namespace)); err != nil {
		return nil
	}

	var nsMatched []*slinkyv1beta1.NodeSet
	for i := range nodesetList.Items {
		nodeset := &nodesetList.Items[i]
		selectorLabels := labels.NewBuilder().WithWorkerSelectorLabels(nodeset).Build()
		selector := k8slabels.SelectorFromSet(k8slabels.Set(selectorLabels))
		if selector.Empty() || !selector.Matches(k8slabels.Set(pod.Labels)) {
			continue
		}
		nsMatched = append(nsMatched, nodeset)
	}

	if len(nsMatched) > 1 {
		// ControllerRef will ensure we do not do anything crazy, but more than one
		// item in this list nevertheless constitutes user error.
		logger.Info("More than one NodeSet is selecting Pod",
			"Pod", klog.KObj(pod), "NodeSets", nsMatched)
	}
	return nsMatched
}

var _ handler.EventHandler = &controllerEventHandler{}

type controllerEventHandler struct {
	client.Reader
	refResolver *refresolver.RefResolver
}

func (e *controllerEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *controllerEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *controllerEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *controllerEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *controllerEventHandler) enqueueRequest(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	controller, ok := obj.(*slinkyv1beta1.Controller)
	if !ok {
		return
	}

	list, err := e.refResolver.GetNodeSetsForController(ctx, controller)
	if err != nil {
		logger.Error(err, "failed to list NodeSets referencing Controller")
		return
	}

	for _, item := range list.Items {
		objectutils.EnqueueRequest(q, &item)
	}
}

// SetEventHandler is a helper function to make slurm node updates propagate to
// the nodeset controller via configured event channel.
func SetEventHandler(client slurmclient.Client, eventCh chan event.GenericEvent) {
	informer := client.GetInformer(slurmtypes.ObjectTypeV0043Node)
	informer.SetEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			node, ok := obj.(*slurmtypes.V0043Node)
			if !ok {
				return
			}
			podInfo := podinfo.PodInfo{}
			_ = podinfo.ParseIntoPodInfo(node.Comment, &podInfo)
			eventCh <- podEvent(podInfo)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNode, ok := oldObj.(*slurmtypes.V0043Node)
			if !ok {
				return
			}
			newNode, ok := newObj.(*slurmtypes.V0043Node)
			if !ok {
				return
			}
			if apiequality.Semantic.DeepEqual(newNode.State, oldNode.State) {
				return
			}
			podInfo := podinfo.PodInfo{}
			_ = podinfo.ParseIntoPodInfo(newNode.Comment, &podInfo)
			eventCh <- podEvent(podInfo)
		},
		DeleteFunc: func(obj interface{}) {
			node, ok := obj.(*slurmtypes.V0043Node)
			if !ok {
				return
			}
			podInfo := podinfo.PodInfo{}
			_ = podinfo.ParseIntoPodInfo(node.Comment, &podInfo)
			eventCh <- podEvent(podInfo)
		},
	})
}

func podEvent(podInfo podinfo.PodInfo) event.GenericEvent {
	return event.GenericEvent{
		Object: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: podInfo.Namespace,
				Name:      podInfo.PodName,
			},
		},
	}
}

var _ handler.EventHandler = &secretEventHandler{}

type secretEventHandler struct {
	client.Reader
	refResolver *refresolver.RefResolver
}

func (e *secretEventHandler) Create(
	ctx context.Context,
	evt event.CreateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *secretEventHandler) Update(
	ctx context.Context,
	evt event.UpdateEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.ObjectNew, q)
}

func (e *secretEventHandler) Delete(
	ctx context.Context,
	evt event.DeleteEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	e.enqueueRequest(ctx, evt.Object, q)
}

func (e *secretEventHandler) Generic(
	ctx context.Context,
	evt event.GenericEvent,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	// Intentionally blank
}

func (e *secretEventHandler) enqueueRequest(
	ctx context.Context,
	obj client.Object,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
) {
	logger := log.FromContext(ctx)

	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return
	}
	secretKey := client.ObjectKeyFromObject(secret)

	controllerList := &slinkyv1beta1.ControllerList{}
	if err := e.List(ctx, controllerList); err != nil {
		logger.Error(err, "failed to list controller CRs")
	}

	for _, controller := range controllerList.Items {
		slurmKeyKey := controller.AuthSlurmKey()
		jwtHs256KeyKey := controller.AuthJwtHs256Key()
		if secretKey.String() != slurmKeyKey.String() &&
			secretKey.String() != jwtHs256KeyKey.String() {
			continue
		}

		nodesetList, err := e.refResolver.GetNodeSetsForController(ctx, &controller)
		if err != nil {
			logger.Error(err, "failed to list NodeSet CRs")
		}

		for _, nodeset := range nodesetList.Items {
			key := client.ObjectKeyFromObject(&controller)
			if nodeset.Spec.ControllerRef.IsMatch(key) {
				objectutils.EnqueueRequest(q, &nodeset)
			}
		}
	}
}
