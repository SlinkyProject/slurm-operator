// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/features"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/annotations"
	"github.com/SlinkyProject/slurm-operator/internal/errors"
	"github.com/SlinkyProject/slurm-operator/internal/resources"
)

// NodeSetPodControlObjectManager abstracts the manipulation of Pods and PVCs. The real controller implements this
// with a clientset for writes and listers for reads; for tests we provide stubs.
type NodeSetPodControlObjectManager interface {
	CreatePod(ctx context.Context, pod *corev1.Pod) error
	GetPod(ctx context.Context, namespace, podName string) (*corev1.Pod, error)
	UpdatePod(ctx context.Context, pod *corev1.Pod) error
	DeletePod(ctx context.Context, pod *corev1.Pod) error
	CreateClaim(ctx context.Context, claim *corev1.PersistentVolumeClaim) error
	GetClaim(ctx context.Context, namespace, claimName string) (*corev1.PersistentVolumeClaim, error)
	UpdateClaim(ctx context.Context, claim *corev1.PersistentVolumeClaim) error
}

// NodeSetPodControl defines the interface that StatefulSetController uses to create, update, and delete Pods,
// and to update the Status of a NodeSet. It follows the design paradigms used for PodControl, but its
// implementation provides for PVC creation, ordered Pod creation, ordered Pod termination, and Pod identity enforcement.
// Manipulation of objects is provided through objectMgr, which allows the k8s API to be mocked out for testing.
type NodeSetPodControl struct {
	client.Client
	objectMgr     NodeSetPodControlObjectManager
	recorder      record.EventRecorder
	slurmClusters *resources.Clusters
}

// NewNodeSetPodControl constructs a NodeSetPodControl using a realNodeSetPodControlObjectManager with the given
// clientset, listers and EventRecorder.
func NewNodeSetPodControl(
	client client.Client,
	recorder record.EventRecorder,
	slurmClusters *resources.Clusters,
) *NodeSetPodControl {
	return &NodeSetPodControl{
		Client: client,
		objectMgr: &realNodeSetPodControlObjectManager{
			Client: client,
		},
		recorder:      recorder,
		slurmClusters: slurmClusters,
	}
}

// NewNodeSetPodControlFromManager creates a NodeSetPodControl using the given NodeSetPodControlObjectManager and recorder.
func NewNodeSetPodControlFromManager(
	client client.Client,
	om NodeSetPodControlObjectManager,
	recorder record.EventRecorder,
	slurmClusters *resources.Clusters,
) *NodeSetPodControl {
	return &NodeSetPodControl{
		Client:        client,
		objectMgr:     om,
		recorder:      recorder,
		slurmClusters: slurmClusters,
	}
}

// realNodeSetPodControlObjectManager uses a clientset.Interface and listers.
type realNodeSetPodControlObjectManager struct {
	client.Client
}

func (om *realNodeSetPodControlObjectManager) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	return om.Create(ctx, pod)
}

func (om *realNodeSetPodControlObjectManager) GetPod(ctx context.Context, namespace, podName string) (*corev1.Pod, error) {
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      podName,
	}
	pod := &corev1.Pod{}
	err := om.Get(ctx, namespacedName, pod)
	return pod, err
}

func (om *realNodeSetPodControlObjectManager) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	return om.Update(ctx, pod)
}

func (om *realNodeSetPodControlObjectManager) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	return om.Delete(ctx, pod)
}

func (om *realNodeSetPodControlObjectManager) CreateClaim(ctx context.Context, claim *corev1.PersistentVolumeClaim) error {
	return om.Create(ctx, claim)
}

func (om *realNodeSetPodControlObjectManager) GetClaim(ctx context.Context, namespace, claimName string) (*corev1.PersistentVolumeClaim, error) {
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      claimName,
	}
	claim := &corev1.PersistentVolumeClaim{}
	err := om.Get(ctx, namespacedName, claim)
	return claim, err
}

func (om *realNodeSetPodControlObjectManager) UpdateClaim(ctx context.Context, claim *corev1.PersistentVolumeClaim) error {
	return om.Update(ctx, claim)
}

func (spc *NodeSetPodControl) CreateNodeSetPod(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	// Create the Pod's PVCs prior to creating the Pod
	if err := spc.createPersistentVolumeClaims(ctx, set, pod); err != nil {
		spc.recordPodEvent("create", set, pod, err)
		return err
	}
	// If we created the PVCs, attempt to create the Pod
	err := spc.objectMgr.CreatePod(ctx, pod)
	// sink already exists errors
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	// Set PVC policy as much as is possible at this point.
	if err := spc.UpdatePodClaimForRetentionPolicy(ctx, set, pod); err != nil {
		spc.recordPodEvent("update", set, pod, err)
		return err
	}
	spc.recordPodEvent("create", set, pod, err)
	return err
}

func (spc *NodeSetPodControl) isNodeSetPodDrain(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) bool {
	clusterName := types.NamespacedName{
		Namespace: set.GetNamespace(),
		Name:      set.Spec.ClusterName,
	}
	slurmClient := spc.slurmClusters.Get(clusterName)
	if slurmClient == nil {
		return false
	}

	objectKey := object.ObjectKey(pod.Spec.Hostname)
	slurmNode := &slurmtypes.Node{}
	err := slurmClient.Get(ctx, objectKey, slurmNode)
	if err != nil {
		return false
	}

	return slurmNode.State.Has(slurmtypes.NodeStateDRAIN)
}

func (spc *NodeSetPodControl) updateSlurmNode(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	if isNodeSetPodCordon(pod) {
		logger.Info("Uncordon Pod", "Pod", pod)
		delete(pod.Annotations, annotations.PodCordon)
		err := spc.Update(ctx, pod)
		spc.recordPodEvent("uncordon", set, pod, err)
		return err
	}

	return nil
}

func (spc *NodeSetPodControl) UpdateNodeSetPod(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	if err := spc.updateSlurmNode(ctx, set, pod); err != nil {
		return err
	}

	attemptedUpdate := false
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// assume the Pod is consistent
		consistent := true
		// if the Pod does not conform to its identity, update the identity and dirty the Pod
		if !identityMatches(set, pod) {
			updateIdentity(set, pod)
			consistent = false
		}
		// if the Pod does not conform to the NodeSet's storage requirements, update the Pod's PVC's,
		// dirty the Pod, and create any missing PVCs
		if !storageMatches(set, pod) {
			updateStorage(set, pod)
			consistent = false
			if err := spc.createPersistentVolumeClaims(ctx, set, pod); err != nil {
				spc.recordPodEvent("update", set, pod, err)
				return err
			}
		}
		if utilfeature.DefaultFeatureGate.Enabled(features.StatefulSetAutoDeletePVC) {
			// if the Pod's PVCs are not consistent with the NodeSet's PVC deletion policy, update the PVC
			// and dirty the pod.
			if match, err := spc.ClaimsMatchRetentionPolicy(ctx, set, pod); err != nil {
				spc.recordPodEvent("update", set, pod, err)
				return err
			} else if !match {
				if err := spc.UpdatePodClaimForRetentionPolicy(ctx, set, pod); err != nil {
					spc.recordPodEvent("update", set, pod, err)
					return err
				}
				consistent = false
			}
		}

		// if the Pod is not dirty, do nothing
		if consistent {
			return nil
		}

		attemptedUpdate = true
		// commit the update, retrying on conflicts

		updateErr := spc.objectMgr.UpdatePod(ctx, pod)
		if updateErr == nil {
			return nil
		}

		if updated, err := spc.objectMgr.GetPod(ctx, set.Namespace, pod.Name); err == nil {
			// make a copy so we do not mutate the shared cache
			pod = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Pod %s/%s: %w", set.Namespace, pod.Name, err))
		}

		return updateErr
	})
	if attemptedUpdate {
		spc.recordPodEvent("update", set, pod, err)
	}
	return err
}

func (spc *NodeSetPodControl) isNodeSetPodDrained(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) bool {
	clusterName := types.NamespacedName{
		Namespace: set.GetNamespace(),
		Name:      set.Spec.ClusterName,
	}
	slurmClient := spc.slurmClusters.Get(clusterName)
	if slurmClient == nil {
		return false
	}

	objectKey := object.ObjectKey(pod.Spec.Hostname)
	slurmNode := &slurmtypes.Node{}
	opts := &slurmclient.GetOptions{
		RefreshCache: true,
	}
	err := slurmClient.Get(ctx, objectKey, slurmNode, opts)
	if err != nil {
		return false
	}

	return slurmNode.State.Has(slurmtypes.NodeStateDRAIN) &&
		slurmNode.State.HasAny(slurmtypes.NodeStateIDLE, slurmtypes.NodeStateDOWN)
}

func (spc *NodeSetPodControl) deleteSlurmNode(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	logger := log.FromContext(ctx)

	clusterName := types.NamespacedName{
		Namespace: set.GetNamespace(),
		Name:      set.Spec.ClusterName,
	}

	if !isNodeSetPodCordon(pod) && !isNodeSetPodDelete(pod) {
		logger.Info("Cordon Pod before deletion", "Pod", pod)
		pod.Annotations[annotations.PodCordon] = "true"
		err := spc.Update(ctx, pod)
		spc.recordPodEvent("cordon", set, pod, err)
		if err != nil {
			return err
		}
	}

	if !spc.isNodeSetPodDrained(ctx, set, pod) && !isNodeSetPodDelete(pod) {
		slurmClient := spc.slurmClusters.Get(clusterName)
		if slurmClient != nil && !isNodeSetPodDelete(pod) {
			objectKey := object.ObjectKey(pod.Spec.Hostname)
			slurmNode := &slurmtypes.Node{}
			if err := slurmClient.Get(ctx, objectKey, slurmNode); err != nil {
				if err.Error() == http.StatusText(http.StatusNotFound) {
					return nil
				}
				return err
			}

			if !spc.isNodeSetPodDrain(ctx, set, pod) {
				logger.Info("Drain Slurm Node before Pod deletion", "slurmNode", slurmNode, "Pod", pod)
				slurmNode = slurmNode.DeepCopy()
				slurmNode.State.Insert(slurmtypes.NodeStateDRAIN)
				if err := slurmClient.Update(ctx, slurmNode); err != nil {
					if err.Error() == http.StatusText(http.StatusNotFound) {
						return nil
					}
					return err
				}
			} else {
				logger.Info("Waiting for Slurm Node to be drained before Pod can be deleted", "slurmNode", slurmNode, "Pod", pod)
			}

			return errors.New(errors.ErrorReasonNodeNotDrained)
		}
	}

	return nil
}

func (spc *NodeSetPodControl) DeleteNodeSetPod(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) error {
	if err := spc.deleteSlurmNode(ctx, set, pod); err != nil {
		return err
	}

	err := spc.objectMgr.DeletePod(ctx, pod)
	spc.recordPodEvent("delete", set, pod, err)
	return err
}

// ClaimsMatchRetentionPolicy returns false if the PVCs for pod are not consistent with set's PVC deletion policy.
// An error is returned if something is not consistent. This is expected if the pod is being otherwise updated,
// but a problem otherwise (see usage of this method in UpdateNodeSetPod).
func (spc *NodeSetPodControl) ClaimsMatchRetentionPolicy(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) (bool, error) {
	logger := log.FromContext(ctx)
	templates := set.Spec.VolumeClaimTemplates
	for i := range templates {
		claimName := getPersistentVolumeClaimName(set, &templates[i], pod.Spec.Hostname)
		claim, err := spc.objectMgr.GetClaim(ctx, set.Namespace, claimName)
		switch {
		case apierrors.IsNotFound(err):
			logger.V(1).Info("Expected claim missing, continuing to pick up in next iteration", "PVC", claim)
		case err != nil:
			return false, fmt.Errorf("could not retrieve claim %s for %s when checking PVC deletion policy", claimName, pod.Name)
		default:
			if !claimOwnerMatchesSetAndPod(logger, claim, set, pod) {
				return false, nil
			}
		}
	}
	return true, nil
}

// UpdatePodClaimForRetentionPolicy updates the PVCs used by pod to match the PVC deletion policy of set.
func (spc *NodeSetPodControl) UpdatePodClaimForRetentionPolicy(ctx context.Context, set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)
	templates := set.Spec.VolumeClaimTemplates
	for i := range templates {
		claimName := getPersistentVolumeClaimName(set, &templates[i], pod.Spec.Hostname)
		claim, err := spc.objectMgr.GetClaim(ctx, set.Namespace, claimName)
		switch {
		case apierrors.IsNotFound(err):
			logger.V(1).Info("Expected claim missing, continuing to pick up in next iteration", "PVC", claim)
		case err != nil:
			return fmt.Errorf("could not retrieve claim %s not found for %s when checking PVC deletion policy: %w", claimName, pod.Name, err)
		default:
			if !claimOwnerMatchesSetAndPod(logger, claim, set, pod) {
				claim = claim.DeepCopy() // Make a copy so we do not mutate the shared cache.
				needsUpdate := updateClaimOwnerRefForSetAndPod(logger, claim, set, pod)
				if needsUpdate {
					err := spc.objectMgr.UpdateClaim(ctx, claim)
					if err != nil {
						return fmt.Errorf("could not update claim %s for delete policy ownerRefs: %w", claimName, err)
					}
				}
			}
		}
	}
	return nil
}

// PodClaimIsStale returns true for a stale PVC that should block pod creation. If the scaling
// policy is deletion, and a PVC has an ownerRef that does not match the pod, the PVC is stale. This
// includes pods whose UID has not been created.
func (spc *NodeSetPodControl) PodClaimIsStale(
	ctx context.Context,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) (bool, error) {
	for _, claim := range getPersistentVolumeClaims(set, pod) {
		pvc, err := spc.objectMgr.GetClaim(ctx, claim.Namespace, claim.Name)
		switch {
		case apierrors.IsNotFound(err):
			// If the claim does not exist yet, it ca not be stale.
			continue
		case err != nil:
			return false, err
		default:
			// A claim is stale if it does not match the pod's UID, including if the pod has no UID.
			if hasStaleOwnerRef(pvc, pod) {
				return true, nil
			}
		}
	}
	return false, nil
}

// recordPodEvent records an event for verb applied to a Pod in a NodeSet. If err is nil the generated event will
// have a reason of corev1.EventTypeNormal. If err is not nil the generated event will have a reason of corev1.EventTypeWarning.
func (spc *NodeSetPodControl) recordPodEvent(
	verb string,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
	err error,
) {
	if err == nil {
		reason := fmt.Sprintf("Successful%s", strings.ToTitle(verb))
		message := fmt.Sprintf("%s Pod %s in NodeSet %s successful",
			strings.ToLower(verb), pod.Name, set.Name)
		spc.recorder.Event(set, corev1.EventTypeNormal, reason, message)
	} else {
		reason := fmt.Sprintf("Failed%s", strings.ToTitle(verb))
		message := fmt.Sprintf("%s Pod %s in NodeSet %s failed error: %s",
			strings.ToLower(verb), pod.Name, set.Name, err)
		spc.recorder.Event(set, corev1.EventTypeWarning, reason, message)
	}
}

// recordClaimEvent records an event for verb applied to the PersistentVolumeClaim of a Pod in a NodeSet. If err is
// nil the generated event will have a reason of corev1.EventTypeNormal. If err is not nil the generated event will have a
// reason of corev1.EventTypeWarning.
func (spc *NodeSetPodControl) recordClaimEvent(verb string, set *slinkyv1alpha1.NodeSet, pod *corev1.Pod, claim *corev1.PersistentVolumeClaim, err error) {
	if err == nil {
		reason := fmt.Sprintf("Successful%s", strings.ToTitle(verb))
		message := fmt.Sprintf("%s Claim %s Pod %s in NodeSet %s success",
			strings.ToLower(verb), claim.Name, pod.Name, set.Name)
		spc.recorder.Event(set, corev1.EventTypeNormal, reason, message)
	} else {
		reason := fmt.Sprintf("Failed%s", strings.ToTitle(verb))
		message := fmt.Sprintf("%s Claim %s for Pod %s in NodeSet %s failed error: %s",
			strings.ToLower(verb), claim.Name, pod.Name, set.Name, err)
		spc.recorder.Event(set, corev1.EventTypeWarning, reason, message)
	}
}

// createMissingPersistentVolumeClaims creates all of the required PersistentVolumeClaims for pod, and updates its retention policy
func (spc *NodeSetPodControl) createMissingPersistentVolumeClaims(ctx context.Context, set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) error {
	if err := spc.createPersistentVolumeClaims(ctx, set, pod); err != nil {
		return err
	}

	// Set PVC policy as much as is possible at this point.
	if err := spc.UpdatePodClaimForRetentionPolicy(ctx, set, pod); err != nil {
		spc.recordPodEvent("update", set, pod, err)
		return err
	}

	return nil
}

// createPersistentVolumeClaims creates all of the required PersistentVolumeClaims for pod, which must be a member of
// set. If all of the claims for Pod are successfully created, the returned error is nil. If creation fails, this method
// may be called again until no error is returned, indicating the PersistentVolumeClaims for pod are consistent with
// set's Spec.
func (spc *NodeSetPodControl) createPersistentVolumeClaims(ctx context.Context, set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) error {
	var errs []error
	for _, claim := range getPersistentVolumeClaims(set, pod) {
		pvc, err := spc.objectMgr.GetClaim(ctx, claim.Namespace, claim.Name)
		switch {
		case apierrors.IsNotFound(err):
			err := spc.objectMgr.CreateClaim(ctx, &claim)
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to create PVC %s: %s", claim.Name, err))
			}
			if err == nil || !apierrors.IsAlreadyExists(err) {
				spc.recordClaimEvent("create", set, pod, &claim, err)
			}
		case err != nil:
			errs = append(errs, fmt.Errorf("failed to retrieve PVC %s: %s", claim.Name, err))
			spc.recordClaimEvent("create", set, pod, &claim, err)
		default:
			if pvc.DeletionTimestamp != nil {
				errs = append(errs, fmt.Errorf("pvc %s is being deleted", claim.Name))
			}
		}
		// TODO: Check resource requirements and accessmodes, update if necessary
	}
	return errorutils.NewAggregate(errs)
}
