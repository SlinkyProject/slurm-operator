// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/history"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/mathutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podcontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

const (
	burstReplicas = 250
)

// Sync implements control logic for synchronizing a NodeSet and its derived Pods.
func (r *NodeSetReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	nodeset := &slinkyv1alpha1.NodeSet{}
	if err := r.Get(ctx, req.NamespacedName, nodeset); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(3).Info("NodeSet has been deleted.", "request", req)
			r.expectations.DeleteExpectations(logger, req.String())
			return nil
		}
		return err
	}

	// Make a copy now to avoid client cache mutation.
	nodeset = nodeset.DeepCopy()
	key := objectutils.KeyFunc(nodeset)

	if err := r.adoptOrphanRevisions(ctx, nodeset); err != nil {
		return err
	}

	revisions, err := r.listRevisions(nodeset)
	if err != nil {
		return err
	}

	currentRevision, updateRevision, collisionCount, err := r.getNodeSetRevisions(nodeset, revisions)
	if err != nil {
		return err
	}
	hash := historycontrol.GetRevision(updateRevision.GetLabels())

	nodesetPods, err := r.getNodeSetPods(ctx, nodeset)
	if err != nil {
		return err
	}

	if !r.expectations.SatisfiedExpectations(logger, key) || nodeset.DeletionTimestamp != nil {
		return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash)
	}

	if err := r.sync(ctx, nodeset, nodesetPods, hash); err != nil {
		return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash, err)
	}

	if r.expectations.SatisfiedExpectations(logger, key) {
		if err := r.syncUpdate(ctx, nodeset, nodesetPods, hash); err != nil {
			return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash, err)
		}
		if err := r.truncateHistory(ctx, nodeset, revisions, currentRevision, updateRevision); err != nil {
			err = fmt.Errorf("failed to clean up revisions of NodeSet(%s): %w", klog.KObj(nodeset), err)
			return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash, err)
		}
	}

	return r.syncStatus(ctx, nodeset, nodesetPods, currentRevision, updateRevision, collisionCount, hash)
}

// adoptOrphanRevisions adopts any orphaned ControllerRevisions that match nodeset's Selector. If all adoptions are
// successful the returned error is nil.
func (r *NodeSetReconciler) adoptOrphanRevisions(ctx context.Context, nodeset *slinkyv1alpha1.NodeSet) error {
	revisions, err := r.listRevisions(nodeset)
	if err != nil {
		return err
	}
	orphanRevisions := make([]*appsv1.ControllerRevision, 0)
	for i := range revisions {
		if metav1.GetControllerOf(revisions[i]) == nil {
			orphanRevisions = append(orphanRevisions, revisions[i])
		}
		// Add the unique label if it iss not already added to the revision.
		// We use the revision name instead of computing hash, so that we do not
		// need to worry about hash collision
		if _, ok := revisions[i].Labels[history.ControllerRevisionHashLabel]; !ok {
			toUpdate := revisions[i].DeepCopy()
			toUpdate.Labels[history.ControllerRevisionHashLabel] = toUpdate.Name
			if err := r.Update(ctx, toUpdate); err != nil {
				return err
			}
		}
	}
	if len(orphanRevisions) > 0 {
		canAdoptErr := r.canAdoptFunc(nodeset)(ctx)
		if canAdoptErr != nil {
			return fmt.Errorf("cannot adopt ControllerRevisions: %w", canAdoptErr)
		}
		return r.doAdoptOrphanRevisions(nodeset, orphanRevisions)
	}
	return nil
}

func (r *NodeSetReconciler) doAdoptOrphanRevisions(
	nodeset *slinkyv1alpha1.NodeSet,
	revisions []*appsv1.ControllerRevision,
) error {
	for i := range revisions {
		adopted, err := r.historyControl.AdoptControllerRevision(nodeset, slinkyv1alpha1.NodeSetGVK, revisions[i])
		if err != nil {
			return err
		}
		revisions[i] = adopted
	}
	return nil
}

// listRevisions returns a array of the ControllerRevisions that represent the revisions of nodeset. If the returned
// error is nil, the returns slice of ControllerRevisions is valid.
func (r *NodeSetReconciler) listRevisions(nodeset *slinkyv1alpha1.NodeSet) ([]*appsv1.ControllerRevision, error) {
	selectorLabels := labels.NewBuilder().WithWorkerSelectorLabels(nodeset).Build()
	selector := k8slabels.SelectorFromSet(k8slabels.Set(selectorLabels))
	return r.historyControl.ListControllerRevisions(nodeset, selector)
}

// getNodeSetPods returns nodeset pods owned by the given nodeset.
// This also reconciles ControllerRef by adopting/orphaning.
// Note that returned histories are pointers to objects in the cache.
// If you want to modify one, you need to deep-copy it first.
func (r *NodeSetReconciler) getNodeSetPods(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
) ([]*corev1.Pod, error) {
	selectorLabels := labels.NewBuilder().WithWorkerSelectorLabels(nodeset).Build()
	selector := k8slabels.SelectorFromSet(k8slabels.Set(selectorLabels))

	// List all pods to include those that do not match the selector anymore but
	// have a ControllerRef pointing to this controller.
	opts := &client.ListOptions{
		Namespace:     nodeset.GetNamespace(),
		LabelSelector: k8slabels.Everything(),
	}
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, opts); err != nil {
		return nil, err
	}
	pods := structutils.ReferenceList(podList.Items)

	filter := func(pod *corev1.Pod) bool {
		// Only claim if it matches our NodeSet name schema. Otherwise release/ignore.
		return nodesetutils.IsPodFromNodeSet(nodeset, pod)
	}

	podControl := podcontrol.NewPodControl(r.Client, r.eventRecorder)

	// Use ControllerRefManager to adopt/orphan as needed.
	cm := kubecontroller.NewPodControllerRefManager(podControl, nodeset, selector, slinkyv1alpha1.NodeSetGVK, r.canAdoptFunc(nodeset))
	return cm.ClaimPods(ctx, pods, filter)
}

// If any adoptions are attempted, we should first recheck for deletion with
// an uncached quorum read sometime after listing Pods/ControllerRevisions.
func (r *NodeSetReconciler) canAdoptFunc(nodeset *slinkyv1alpha1.NodeSet) func(ctx context.Context) error {
	return kubecontroller.RecheckDeletionTimestamp(func(ctx context.Context) (metav1.Object, error) {
		namespacedName := types.NamespacedName{
			Namespace: nodeset.GetNamespace(),
			Name:      nodeset.GetName(),
		}
		fresh := &slinkyv1alpha1.NodeSet{}
		if err := r.Get(ctx, namespacedName, fresh); err != nil {
			return nil, err
		}
		if fresh.UID != nodeset.UID {
			return nil, fmt.Errorf("original NodeSet(%s) is gone: got UID(%v), wanted UID(%v)",
				klog.KObj(nodeset), fresh.UID, nodeset.UID)
		}
		return fresh, nil
	})
}

// sync is the main reconciliation logic.
func (r *NodeSetReconciler) sync(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	// Sync cluster-wide hostname service early to enable hostname resolution
	if err := r.syncClusterWideHostnameService(ctx, nodeset); err != nil {
		return err
	}

	if err := r.syncSlurm(ctx, nodeset, pods); err != nil {
		return err
	}

	if err := r.syncNodeSet(ctx, nodeset, pods, hash); err != nil {
		return err
	}

	return nil
}

// syncClusterWideHostnameService manages the cluster-wide hostname service for the Slurm cluster
func (r *NodeSetReconciler) syncClusterWideHostnameService(ctx context.Context, nodeset *slinkyv1alpha1.NodeSet) error {
	logger := log.FromContext(ctx)

	// Get service name from builder
	service, err := r.builder.BuildClusterWideWorkerService(nodeset)
	if err != nil {
		return fmt.Errorf("failed to build cluster-wide service: %w", err)
	}
	serviceName := service.Name
	logger.V(2).Info("Syncing cluster-wide hostname service", "service", serviceName)

	// Create/update the cluster-wide headless service with multi-owner support
	existingService := &corev1.Service{}
	key := client.ObjectKeyFromObject(service)
	err = r.Get(ctx, key, existingService)

	switch {
	case apierrors.IsNotFound(err):
		// Service doesn't exist, create it
		logger.V(2).Info("Creating new cluster-wide service", "service", key)
		if err := r.Create(ctx, service); err != nil {
			return fmt.Errorf("failed to create cluster-wide worker service (%s): %w", klog.KObj(service), err)
		}
	case err != nil:
		return fmt.Errorf("failed to get existing service: %w", err)
	default:
		// Service exists, add this NodeSet as an additional owner if not already present
		// Check if this NodeSet is already an owner
		nodesetUID := nodeset.GetUID()
		alreadyOwner := false
		for _, existingOwner := range existingService.OwnerReferences {
			if existingOwner.UID == nodesetUID {
				logger.V(2).Info("NodeSet is already an owner of the service")
				alreadyOwner = true
				break
			}
		}

		if !alreadyOwner {
			// Create a patch from the current state
			patch := client.MergeFrom(existingService.DeepCopy())

			// Add this NodeSet as a non-controller owner using controllerutil
			if err := controllerutil.SetOwnerReference(nodeset, existingService, r.Scheme, controllerutil.WithBlockOwnerDeletion(true)); err != nil {
				return fmt.Errorf("failed to set NodeSet as owner reference: %w", err)
			}

			// Ensure it's marked as non-controller owner for multi-owner pattern
			for i := range existingService.OwnerReferences {
				if existingService.OwnerReferences[i].UID == nodesetUID {
					existingService.OwnerReferences[i].Controller = ptr.To(false)
					break
				}
			}

			logger.V(2).Info("Adding NodeSet as owner of cluster-wide service",
				"service", client.ObjectKeyFromObject(existingService),
				"nodeset", nodeset.GetName())

			if err := r.Patch(ctx, existingService, patch); err != nil {
				return fmt.Errorf("failed to add NodeSet as owner of cluster-wide service (%s): %w", klog.KObj(service), err)
			}
		}
	}

	logger.V(2).Info("Successfully synced cluster-wide hostname service", "service", klog.KObj(service))
	return nil
}

// syncSlurm will reconcile the Slurm Nodes with the NodeSet Pods.
func (r *NodeSetReconciler) syncSlurm(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
) error {
	nodeDeadlines, err := r.slurmControl.GetNodeDeadlines(ctx, nodeset, pods)
	if err != nil {
		return err
	}

	syncSlurmFn := func(i int) error {
		pod := pods[i]
		slurmNodeName := nodesetutils.GetNodeName(pod)
		deadline := nodeDeadlines.Peek(slurmNodeName)

		toUpdate := pod.DeepCopy()
		if deadline.IsZero() {
			delete(toUpdate.Annotations, slinkyv1alpha1.AnnotationPodDeadline)
		} else {
			toUpdate.Annotations[slinkyv1alpha1.AnnotationPodDeadline] = deadline.Format(time.RFC3339)
		}
		if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
			return err
		}

		if podutils.IsPodCordon(pod) {
			reason := fmt.Sprintf("Pod (%s) is cordoned", klog.KObj(pod))
			if err := r.slurmControl.MakeNodeDrain(ctx, nodeset, pod, reason); err != nil {
				return err
			}
		} else {
			reason := fmt.Sprintf("Pod (%s) is uncordoned", klog.KObj(pod))
			if err := r.slurmControl.MakeNodeUndrain(ctx, nodeset, pod, reason); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, syncSlurmFn); err != nil {
		return err
	}

	return nil
}

// syncNodeSet will reconcile NodeSet pod replica counts.
// Pods will be:
//   - Scaled out when: `replicaCount < replicasWant“
//   - Scaled in when: `replicaCount > replicasWant“
//   - Processed when: `replicaCount == replicasWant“
func (r *NodeSetReconciler) syncNodeSet(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	logger := log.FromContext(ctx)

	// Handle replica scaling by comparing the known pods to the target number of replicas.
	// Create or delete pods as needed to reach the target number.
	replicaCount := int(ptr.Deref(nodeset.Spec.Replicas, 0))
	diff := len(pods) - replicaCount
	if diff < 0 {
		diff = -diff
		logger.V(2).Info("Too few NodeSet pods", "need", replicaCount, "creating", diff)
		return r.doPodScaleOut(ctx, nodeset, pods, diff, hash)
	}

	if diff > 0 {
		logger.V(2).Info("Too many NodeSet pods", "need", replicaCount, "deleting", diff)
		podsToDelete, podsToKeep := nodesetutils.SplitActivePods(pods, diff)
		return r.doPodScaleIn(ctx, nodeset, podsToDelete, podsToKeep)
	}

	logger.V(2).Info("Processing NodeSet pods", "replicas", replicaCount)
	return r.doPodProcessing(ctx, nodeset, pods, hash)
}

// doPodScaleOut handles scaling-out NodeSet pods.
// NodeSet pods should be uncordoned and undrained, and new pods created.
func (r *NodeSetReconciler) doPodScaleOut(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	numCreate int,
	hash string,
) error {
	logger := log.FromContext(ctx)
	key := objectutils.KeyFunc(nodeset)

	uncordonFn := func(i int) error {
		pod := pods[i]
		return r.makePodUncordonAndUndrain(ctx, nodeset, pod)
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, uncordonFn); err != nil {
		return err
	}

	numCreate = mathutils.Clamp(numCreate, 0, burstReplicas)

	usedOrdinals := set.New[int]()
	for _, pod := range pods {
		usedOrdinals.Insert(nodesetutils.GetOrdinal(pod))
	}

	podsToCreate := make([]*corev1.Pod, numCreate)
	ordinal := 0
	for i := range numCreate {
		for usedOrdinals.Has(ordinal) {
			ordinal++
		}
		pod, err := r.newNodeSetPod(ctx, nodeset, ordinal, hash)
		if err != nil {
			return err
		}
		usedOrdinals.Insert(ordinal)
		podsToCreate[i] = pod
	}

	// TODO: Track UIDs of creates just like deletes. The problem currently
	// is we'd need to wait on the result of a create to record the pod's
	// UID, which would require locking *across* the create, which will turn
	// into a performance bottleneck. We should generate a UID for the pod
	// beforehand and store it via ExpectCreations.
	if err := r.expectations.ExpectCreations(logger, key, numCreate); err != nil {
		return err
	}

	// Batch the pod creates. Batch sizes start at SlowStartInitialBatchSize
	// and double with each successful iteration in a kind of "slow start".
	// This handles attempts to start large numbers of pods that would
	// likely all fail with the same error. For example a project with a
	// low quota that attempts to create a large number of pods will be
	// prevented from spamming the API service with the pod create requests
	// after one of its pods fails. Conveniently, this also prevents the
	// event spam that those failures would generate.
	successfulCreations, err := utils.SlowStartBatch(numCreate, utils.SlowStartInitialBatchSize, func(index int) error {
		pod := podsToCreate[index]
		if err := r.podControl.CreateNodeSetPod(ctx, nodeset, pod); err != nil {
			if apierrors.HasStatusCause(err, corev1.NamespaceTerminatingCause) {
				// if the namespace is being terminated, we don't have to do
				// anything because any creation will fail
				return nil
			}
			return err
		}
		return nil
	})

	// Any skipped pods that we never attempted to start shouldn't be expected.
	// The skipped pods will be retried later. The next controller resync will
	// retry the slow start process.
	if skippedPods := numCreate - successfulCreations; skippedPods > 0 {
		logger.V(2).Info("Slow-start failure. Skipping creation of pods, decrementing expectations",
			"podsSkipped", skippedPods, "kind", slinkyv1alpha1.NodeSetGVK)
		for range skippedPods {
			// Decrement the expected number of creates because the informer won't observe this pod
			r.expectations.CreationObserved(logger, key)
		}
	}

	return err
}

func (r *NodeSetReconciler) newNodeSetPod(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	ordinal int,
	revisionHash string,
) (*corev1.Pod, error) {
	controller := &slinkyv1alpha1.Controller{}
	key := nodeset.Spec.ControllerRef.NamespacedName()
	if err := r.Get(ctx, key, controller); err != nil {
		return nil, err
	}

	pod := nodesetutils.NewNodeSetPod(nodeset, controller, ordinal, revisionHash)

	return pod, nil
}

// doPodScaleIn handles scaling-in NodeSet pods.
// Ceratain NodeSet pods should be cordoned and drained, and defunct pods
// deleted after being fulled drained.
func (r *NodeSetReconciler) doPodScaleIn(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	podsToDelete, podsToKeep []*corev1.Pod,
) error {
	logger := log.FromContext(ctx)
	key := objectutils.KeyFunc(nodeset)

	uncordonFn := func(i int) error {
		pod := podsToKeep[i]
		return r.makePodUncordonAndUndrain(ctx, nodeset, pod)
	}
	if _, err := utils.SlowStartBatch(len(podsToKeep), utils.SlowStartInitialBatchSize, uncordonFn); err != nil {
		return err
	}

	fixPodPVCsFn := func(i int) error {
		pod := podsToDelete[i]
		if matchPolicy, err := r.podControl.PodPVCsMatchRetentionPolicy(ctx, nodeset, pod); err != nil {
			return err
		} else if !matchPolicy {
			if err := r.podControl.UpdatePodPVCsForRetentionPolicy(ctx, nodeset, pod); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := utils.SlowStartBatch(len(podsToDelete), utils.SlowStartInitialBatchSize, fixPodPVCsFn); err != nil {
		return err
	}

	numDelete := mathutils.Clamp(len(podsToDelete), 0, burstReplicas)

	// Snapshot the UIDs (namespace/name) of the pods we're expecting to see
	// deleted, so we know to record their expectations exactly once either
	// when we see it as an update of the deletion timestamp, or as a delete.
	// Note that if the labels on a pod/nodeset change in a way that the pod gets
	// orphaned, the nodeset will only wake up after the expectations have
	// expired even if other pods are deleted.
	if err := r.expectations.ExpectDeletions(logger, key, getPodKeys(podsToDelete)); err != nil {
		return err
	}
	_, err := utils.SlowStartBatch(numDelete, utils.SlowStartInitialBatchSize, func(index int) error {
		pod := podsToDelete[index]
		podKey := kubecontroller.PodKey(pod)
		if err := r.processCondemned(ctx, nodeset, podsToDelete, index); err != nil {
			// Decrement the expected number of deletes because the informer won't observe this deletion
			r.expectations.DeletionObserved(logger, key, podKey)
			if !apierrors.IsNotFound(err) {
				logger.V(2).Info("Failed to delete pod, decremented expectations",
					"pod", podKey, "kind", slinkyv1alpha1.NodeSetGVK)
				return err
			}
		}
		if isDrained, err := r.slurmControl.IsNodeDrained(ctx, nodeset, pod); !isDrained || err != nil {
			// Decrement expectations and requeue reconcile because the Slurm node is not drained yet.
			// We must wait until fully drained to terminate the pod.
			r.expectations.DeletionObserved(logger, key, podKey)
			if err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func getPodKeys(pods []*corev1.Pod) []string {
	podKeys := make([]string, 0, len(pods))
	for _, pod := range pods {
		podKeys = append(podKeys, kubecontroller.PodKey(pod))
	}
	return podKeys
}

// processCondemned will gracefully terminate the condemned NodeSet pod.
// NOTE: intended to be used by utils.SlowStartBatch().
func (r *NodeSetReconciler) processCondemned(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	condemned []*corev1.Pod,
	i int,
) error {
	logger := klog.FromContext(ctx)
	pod := condemned[i]
	key := objectutils.KeyFunc(pod)

	podKey := client.ObjectKeyFromObject(pod)
	if err := r.Get(ctx, podKey, pod); err != nil {
		return err
	}

	if podutils.IsTerminating(pod) {
		logger.V(3).Info("NodeSet Pod is terminating, skipping further processing",
			"pod", klog.KObj(pod))
		return nil
	}

	isDrained, err := r.slurmControl.IsNodeDrained(ctx, nodeset, pod)
	if err != nil {
		return err
	}

	if podutils.IsRunning(pod) && !isDrained {
		logger.V(2).Info("NodeSet Pod is draining, pending termination for scale-in",
			"pod", klog.KObj(pod))
		// Decrement expectations and requeue reconcile because the Slurm node is not drained yet.
		// We must wait until fully drained to terminate the pod.
		durationStore.Push(key, 30*time.Second)
		return r.makePodCordonAndDrain(ctx, nodeset, pod)
	}

	logger.V(2).Info("NodeSet Pod is terminating for scale-in",
		"pod", klog.KObj(pod))
	if err := r.podControl.DeleteNodeSetPod(ctx, nodeset, pod); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// doPodProcessing handles batch processing of NodeSet pods.
func (r *NodeSetReconciler) doPodProcessing(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	logger := log.FromContext(ctx)
	key := objectutils.KeyFunc(nodeset)

	// NOTE: we must respect the uncordon and undrain nodes in accordance with updateStrategy
	// to not fight it given the statefulness of how we cordon and terminate nodeset pods.
	_, podsToKeep := r.splitUpdatePods(ctx, nodeset, pods, hash)
	uncordonFn := func(i int) error {
		pod := podsToKeep[i]
		return r.makePodUncordonAndUndrain(ctx, nodeset, pod)
	}
	if _, err := utils.SlowStartBatch(len(podsToKeep), utils.SlowStartInitialBatchSize, uncordonFn); err != nil {
		return err
	}

	if err := r.expectations.SetExpectations(logger, key, 0, 0); err != nil {
		return err
	}

	processReplicaFn := func(i int) error {
		pod := pods[i]
		return r.processReplica(ctx, nodeset, pod)
	}
	if _, err := utils.SlowStartBatch(len(pods), utils.SlowStartInitialBatchSize, processReplicaFn); err != nil {
		return err
	}

	return nil
}

// processReplica will ensure the NodeSet pod replica can be scheduled and cleanup errant pods.
// NOTE: intended to be used by utils.SlowStartBatch().
func (r *NodeSetReconciler) processReplica(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	// Note that pods with phase Succeeded will also trigger this event. This is
	// because final pod phase of evicted or otherwise forcibly stopped pods
	// (e.g. terminated on node reboot) is determined by the exit code of the
	// container, not by the reason for pod termination. We should restart the
	// pod regardless of the exit code.
	if podutils.IsFailed(pod) || podutils.IsSucceeded(pod) {
		if !podutils.IsTerminating(pod) {
			if err := r.podControl.DeleteNodeSetPod(ctx, nodeset, pod); err != nil {
				return err
			}
		}
		// New pod should be generated on the next sync after the current pod is removed from etcd.
		return nil
	}

	return r.podControl.UpdateNodeSetPod(ctx, nodeset, pod)
}

// makePodCordonAndDrain will cordon the pod and drain the corresponding Slurm node.
func (r *NodeSetReconciler) makePodCordonAndDrain(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	if err := r.makePodCordon(ctx, pod); err != nil {
		return err
	}

	reason := fmt.Sprintf("Pod (%s) has been cordoned", klog.KObj(pod))
	if err := r.slurmControl.MakeNodeDrain(ctx, nodeset, pod, reason); err != nil {
		return err
	}

	return nil
}

// makePodCordon will cordon the pod.
func (r *NodeSetReconciler) makePodCordon(
	ctx context.Context,
	pod *corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	if podutils.IsPodCordon(pod) {
		return nil
	}

	toUpdate := pod.DeepCopy()
	logger.Info("Cordon Pod, pending deletion", "Pod", klog.KObj(toUpdate))
	if toUpdate.Annotations == nil {
		toUpdate.Annotations = make(map[string]string)
	}
	toUpdate.Annotations[slinkyv1alpha1.AnnotationPodCordon] = "true"
	if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
		return err
	}

	return nil
}

// makePodUncordonAndUndrain will uncordon the pod and undrain the corresponding Slurm node.
func (r *NodeSetReconciler) makePodUncordonAndUndrain(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	if err := r.makePodUncordon(ctx, pod); err != nil {
		return err
	}

	reason := fmt.Sprintf("Pod (%s) has been uncordoned", klog.KObj(pod))
	if err := r.slurmControl.MakeNodeUndrain(ctx, nodeset, pod, reason); err != nil {
		return err
	}

	return nil
}

// makePodUncordonAndUndrain will uncordon the pod.
func (r *NodeSetReconciler) makePodUncordon(ctx context.Context, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	if !podutils.IsPodCordon(pod) {
		return nil
	}

	toUpdate := pod.DeepCopy()
	logger.Info("Uncordon Pod", "Pod", klog.KObj(toUpdate))
	delete(toUpdate.Annotations, slinkyv1alpha1.AnnotationPodCordon)
	if err := r.Patch(ctx, toUpdate, client.StrategicMergeFrom(pod)); err != nil {
		return err
	}

	return nil
}

// syncUpdate will synchronize NodeSet pod version updates based on update type.
func (r *NodeSetReconciler) syncUpdate(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	switch nodeset.Spec.UpdateStrategy.Type {
	case slinkyv1alpha1.OnDeleteNodeSetStrategyType:
		// r.syncNodeSet() will handled it on the next reconcile
		return nil
	case slinkyv1alpha1.RollingUpdateNodeSetStrategyType:
		return r.syncRollingUpdate(ctx, nodeset, pods, hash)
	default:
		return nil
	}
}

// syncRollingUpdate will synchronize rolling updates for NodeSet pods.
func (r *NodeSetReconciler) syncRollingUpdate(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) error {
	logger := log.FromContext(ctx)

	_, oldPods := findUpdatedPods(pods, hash)

	unhealthyPods, healthyPods := nodesetutils.SplitUnhealthyPods(oldPods)
	if len(unhealthyPods) > 0 {
		logger.Info("Delete unhealthy pods for Rolling Update",
			"unhealthyPods", len(unhealthyPods))
		if err := r.doPodScaleIn(ctx, nodeset, unhealthyPods, nil); err != nil {
			return err
		}
	}

	podsToDelete, _ := r.splitUpdatePods(ctx, nodeset, healthyPods, hash)
	if len(podsToDelete) > 0 {
		logger.Info("Scale-in pods for Rolling Update",
			"delete", len(podsToDelete))
		if err := r.doPodScaleIn(ctx, nodeset, podsToDelete, nil); err != nil {
			return err
		}
	}

	return nil
}

// splitUpdatePods returns two pod lists based on UpdateStrategy type.
func (r *NodeSetReconciler) splitUpdatePods(
	ctx context.Context,
	nodeset *slinkyv1alpha1.NodeSet,
	pods []*corev1.Pod,
	hash string,
) (podsToDelete, podsToKeep []*corev1.Pod) {
	logger := log.FromContext(ctx)

	switch nodeset.Spec.UpdateStrategy.Type {
	case slinkyv1alpha1.OnDeleteNodeSetStrategyType:
		return nil, nil
	case slinkyv1alpha1.RollingUpdateNodeSetStrategyType:
		newPods, oldPods := findUpdatedPods(pods, hash)

		var numUnavailable int
		now := metav1.Now()
		for _, pod := range newPods {
			if !podutil.IsPodAvailable(pod, nodeset.Spec.MinReadySeconds, now) {
				numUnavailable++
			}
		}

		total := int(ptr.Deref(nodeset.Spec.Replicas, 0))
		maxUnavailable := mathutils.GetScaledValueFromIntOrPercent(nodeset.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable, total, true, 1)
		remainingUnavailable := mathutils.Clamp((maxUnavailable - numUnavailable), 0, maxUnavailable)
		podsToDelete, remainingOldPods := nodesetutils.SplitActivePods(oldPods, remainingUnavailable)

		remainingPods := make([]*corev1.Pod, len(newPods))
		copy(remainingPods, newPods)
		remainingPods = append(remainingPods, remainingOldPods...)

		logger.V(1).Info("calculated pod lists for update",
			"maxUnavailable", maxUnavailable,
			"updatePods", len(podsToDelete),
			"remainingPods", len(remainingPods))
		return podsToDelete, remainingPods
	default:
		return nil, nil
	}
}

// findUpdatedPods looks at non-deleted pods and returns two lists, new and old pods, given the hash.
func findUpdatedPods(pods []*corev1.Pod, hash string) (newPods, oldPods []*corev1.Pod) {
	for _, pod := range pods {
		if podutils.IsTerminating(pod) {
			continue
		}
		if historycontrol.GetRevision(pod.GetLabels()) == hash {
			newPods = append(newPods, pod)
		} else {
			oldPods = append(oldPods, pod)
		}
	}
	return newPods, oldPods
}
