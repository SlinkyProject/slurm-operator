// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package podcontrol

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podcontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podutils"
)

const (
	eventCreate = "Create"
	eventDelete = "Delete"
	eventUpdate = "Update"
)

var (
	// podGVK contains the schema.GroupVersionKind for pods.
	podGVK = corev1.SchemeGroupVersion.WithKind("Pod")
)

type PodControlInterface interface {
	CreateNodeSetPod(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error
	DeleteNodeSetPod(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error
	UpdateNodeSetPod(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error

	PodPVCsMatchRetentionPolicy(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error)
	UpdatePodPVCsForRetentionPolicy(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error
	IsPodPVCsStale(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error)
}

// RealPodControl is the default implementation of PodControlInterface.
type realPodControl struct {
	client.Client
	recorder   record.EventRecorder
	podControl podcontrol.PodControlInterface
}

// CreateNodeSetPod implements PodControlInterface.
func (r *realPodControl) CreateNodeSetPod(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error {
	// Create the Pod's PVCs prior to creating the Pod
	if err := r.createPersistentVolumeClaims(ctx, nodeset, pod); err != nil {
		r.recordPodEvent(eventCreate, nodeset, pod, err)
		return err
	}
	// If we created the PVCs attempt to create the Pod
	err := r.podControl.CreateThisPod(ctx, pod, nodeset)
	if apierrors.IsAlreadyExists(err) {
		return err
	}
	// Set PVC policy as much as is possible at this point.
	if err := r.UpdatePodPVCsForRetentionPolicy(ctx, nodeset, pod); err != nil {
		r.recordPodEvent(eventUpdate, nodeset, pod, err)
		return err
	}
	r.recordPodEvent(eventCreate, nodeset, pod, err)
	return err
}

// DeleteNodeSetPod implements PodControlInterface.
func (r *realPodControl) DeleteNodeSetPod(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error {
	err := r.podControl.DeletePod(ctx, pod.GetNamespace(), pod.GetName(), nodeset)
	r.recordPodEvent(eventDelete, nodeset, pod, err)
	return err
}

// UpdatePod implements PodControlInterface.
func (r *realPodControl) UpdateNodeSetPod(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error {
	attemptedUpdate := false
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// assume the Pod is consistent
		consistent := true
		// if the Pod does not conform to its identity, update the identity and dirty the Pod
		if !nodesetutils.IsIdentityMatch(nodeset, pod) {
			nodesetutils.UpdateIdentity(nodeset, pod)
			consistent = false
		}
		// if the Pod does not conform to the NodeSet's storage requirements, update the Pod's PVC's,
		// dirty the Pod, and create any missing PVCs
		if !nodesetutils.IsStorageMatch(nodeset, pod) {
			nodesetutils.UpdateStorage(nodeset, pod)
			consistent = false
			if err := r.createPersistentVolumeClaims(ctx, nodeset, pod); err != nil {
				r.recordPodEvent(eventUpdate, nodeset, pod, err)
				return err
			}
		}
		// if the Pod's PVCs are not consistent with the NodeSet's PVC deletion policy, update the PVC
		// and dirty the pod.
		if match, err := r.PodPVCsMatchRetentionPolicy(ctx, nodeset, pod); err != nil {
			r.recordPodEvent(eventUpdate, nodeset, pod, err)
			return err
		} else if !match {
			if err := r.UpdatePodPVCsForRetentionPolicy(ctx, nodeset, pod); err != nil {
				r.recordPodEvent(eventUpdate, nodeset, pod, err)
				return err
			}
			consistent = false
		}

		// if the Pod is not dirty, do nothing
		if consistent {
			return nil
		}

		attemptedUpdate = true
		// commit the update, retrying on conflicts

		updateErr := r.Update(ctx, pod)
		if updateErr == nil {
			return nil
		}

		podId := types.NamespacedName{
			Namespace: nodeset.Namespace,
			Name:      pod.Name,
		}
		updated := &corev1.Pod{}
		if err := r.Get(ctx, podId, updated); err == nil {
			// make a copy so we don't mutate the shared cache
			pod = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Pod %s/%s: %w", nodeset.Namespace, pod.Name, err))
		}

		return updateErr
	})
	if attemptedUpdate {
		r.recordPodEvent(eventUpdate, nodeset, pod, err)
	}
	return err
}

// PodPVCsMatchRetentionPolicy returns false if the PVCs for pod are not consistent with nodeset's PVC deletion policy.
// An error is returned if something is not consistent. This is expected if the pod is being otherwise updated,
// but a problem otherwise (see usage of this method in UpdateNodeSetPod).
func (r *realPodControl) PodPVCsMatchRetentionPolicy(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error) {
	logger := klog.FromContext(ctx)
	ordinal := nodesetutils.GetOrdinal(pod)
	paddedOrdinal := nodesetutils.GetPaddedOrdinal(nodeset, ordinal)
	templates := nodeset.Spec.VolumeClaimTemplates
	for i := range templates {
		claimName := nodesetutils.GetPersistentVolumeClaimName(nodeset, &templates[i], paddedOrdinal)
		claim := &corev1.PersistentVolumeClaim{}
		claimId := types.NamespacedName{
			Namespace: nodeset.Namespace,
			Name:      claimName,
		}
		err := r.Get(ctx, claimId, claim)
		switch {
		case apierrors.IsNotFound(err):
			klog.FromContext(ctx).V(4).Info("Expected claim missing, continuing to pick up in next iteration", "PVC", klog.KObj(claim))
		case err != nil:
			return false, fmt.Errorf("could not retrieve claim %s for %s when checking PVC deletion policy", claimName, pod.Name)
		default:
			if !isClaimOwnerUpToDate(logger, claim, nodeset, pod) {
				return false, nil
			}
		}
	}
	return true, nil
}

// UpdatePodPVCsForRetentionPolicy implements PodControlInterface.
func (r *realPodControl) UpdatePodPVCsForRetentionPolicy(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error {
	logger := klog.FromContext(ctx)
	ordinal := nodesetutils.GetOrdinal(pod)
	paddedOrdinal := nodesetutils.GetPaddedOrdinal(nodeset, ordinal)
	templates := nodeset.Spec.VolumeClaimTemplates
	for i := range templates {
		claimName := nodesetutils.GetPersistentVolumeClaimName(nodeset, &templates[i], paddedOrdinal)
		claimId := types.NamespacedName{
			Namespace: nodeset.Namespace,
			Name:      claimName,
		}
		claim := &corev1.PersistentVolumeClaim{}
		err := r.Get(ctx, claimId, claim)
		switch {
		case apierrors.IsNotFound(err):
			logger.V(4).Info("Expected claim missing, continuing to pick up in next iteration", "PVC", klog.KObj(claim))
		case err != nil:
			return fmt.Errorf("could not retrieve claim %s not found for %s when checking PVC deletion policy: %w", claimName, pod.Name, err)
		default:
			if hasUnexpectedController(claim, nodeset, pod) {
				// Add an event so the user knows they're in a strange configuration. The claim will be cleaned up below.
				msg := fmt.Sprintf("PersistentVolumeClaim %s has a conflicting OwnerReference that acts as a managing controller, the retention policy is ignored for this claim", claimName)
				r.recorder.Event(nodeset, corev1.EventTypeWarning, "ConflictingController", msg)
			}
			if !isClaimOwnerUpToDate(logger, claim, nodeset, pod) {
				claim = claim.DeepCopy() // Make a copy so we don't mutate the shared cache.
				updateClaimOwnerRefForSetAndPod(logger, claim, nodeset, pod)
				if err := r.Update(ctx, claim); err != nil {
					return fmt.Errorf("could not update claim %s for delete policy ownerRefs: %w", claimName, err)
				}
			}
		}
	}
	return nil
}

// IsPodPVCsStale returns true for a stale PVC that should block pod creation. If the scaling
// policy is deletion, and a PVC has an ownerRef that does not match the pod, the PVC is stale. This
// includes pods whose UID has not been created.
func (r *realPodControl) IsPodPVCsStale(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) (bool, error) {
	policy := getPersistentVolumeClaimRetentionPolicy(nodeset)
	if policy.WhenScaled == slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType {
		// PVCs are meant to be reused and so can't be stale.
		return false, nil
	}
	for _, claim := range nodesetutils.GetPersistentVolumeClaims(nodeset, pod) {
		pvc := &corev1.PersistentVolumeClaim{}
		pvcId := types.NamespacedName{
			Namespace: claim.Namespace,
			Name:      claim.Name,
		}
		err := r.Get(ctx, pvcId, pvc)
		switch {
		case apierrors.IsNotFound(err):
			// If the claim doesn't exist yet, it can't be stale.
			continue
		case err != nil:
			return false, err
		default:
			if hasStaleOwnerRef(pvc, pod, podGVK) {
				return true, nil
			}
		}
	}
	return false, nil
}

// recordPodEvent records an event for verb applied to a Pod in a NodeSet. If err is nil the generated event will
// have a reason of corev1.EventTypeNormal. If err is not nil the generated event will have a reason of corev1.EventTypeWarning.
func (r *realPodControl) recordPodEvent(verb string, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, err error) {
	caser := cases.Title(language.English)
	if err == nil {
		reason := fmt.Sprintf("Successful%s", caser.String(verb))
		message := fmt.Sprintf("%s Pod %s in NodeSet %s successful",
			strings.ToLower(verb), pod.Name, nodeset.GetName())
		r.recorder.Event(nodeset, corev1.EventTypeNormal, reason, message)
	} else {
		reason := fmt.Sprintf("Failed%s", caser.String(verb))
		message := fmt.Sprintf("%s Pod %s in NodeSet %s failed error: %s",
			strings.ToLower(verb), pod.Name, nodeset.GetName(), err)
		r.recorder.Event(nodeset, corev1.EventTypeWarning, reason, message)
	}
}

// createPersistentVolumeClaims creates all of the required PersistentVolumeClaims for pod, which must be a member of
// nodeset. If all of the claims for Pod are successfully created, the returned error is nil. If creation fails, this method
// may be called again until no error is returned, indicating the PersistentVolumeClaims for pod are consistent with
// nodeset's Spec.
func (r *realPodControl) createPersistentVolumeClaims(ctx context.Context, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) error {
	var errs []error
	for _, claim := range nodesetutils.GetPersistentVolumeClaims(nodeset, pod) {
		pvcId := types.NamespacedName{
			Namespace: nodeset.Namespace,
			Name:      claim.Name,
		}
		pvc := &corev1.PersistentVolumeClaim{}
		err := r.Get(ctx, pvcId, pvc)
		switch {
		case apierrors.IsNotFound(err):
			if err := r.Create(ctx, &claim); err != nil {
				errs = append(errs, fmt.Errorf("failed to create PVC %s: %w", claim.Name, err))
			}
			if err == nil || !apierrors.IsAlreadyExists(err) {
				r.recordClaimEvent(eventCreate, nodeset, pod, &claim, err)
			}
		case err != nil:
			errs = append(errs, fmt.Errorf("failed to retrieve PVC %s: %w", claim.Name, err))
			r.recordClaimEvent(eventCreate, nodeset, pod, &claim, err)
		default:
			if pvc.DeletionTimestamp != nil {
				errs = append(errs, fmt.Errorf("pvc %s is being deleted", claim.Name))
			}
		}
		// TODO: Check resource requirements and accessmodes, update if necessary
	}
	return errorutils.NewAggregate(errs)
}

// recordClaimEvent records an event for verb applied to the PersistentVolumeClaim of a Pod in a NodeSet. If err is
// nil the generated event will have a reason of corev1.EventTypeNormal. If err is not nil the generated event will have a
// reason of corev1.EventTypeWarning.
func (r *realPodControl) recordClaimEvent(verb string, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod, claim *corev1.PersistentVolumeClaim, err error) {
	caser := cases.Title(language.English)
	if err == nil {
		reason := fmt.Sprintf("Successful%s", caser.String(verb))
		message := fmt.Sprintf("%s Claim %s Pod %s in NodeSet %s successful",
			strings.ToLower(verb), claim.Name, pod.Name, nodeset.Name)
		r.recorder.Event(nodeset, corev1.EventTypeNormal, reason, message)
	} else {
		reason := fmt.Sprintf("Failed%s", caser.String(verb))
		message := fmt.Sprintf("%s Claim %s for Pod %s in NodeSet %s failed error: %s",
			strings.ToLower(verb), claim.Name, pod.Name, nodeset.Name, err)
		r.recorder.Event(nodeset, corev1.EventTypeWarning, reason, message)
	}
}

var _ PodControlInterface = &realPodControl{}

func NewPodControl(client client.Client, recorder record.EventRecorder) PodControlInterface {
	return &realPodControl{
		Client:     client,
		recorder:   recorder,
		podControl: podcontrol.NewPodControl(client, recorder),
	}
}

// isClaimOwnerUpToDate returns false if the ownerRefs of the claim are not nodeset consistently with the
// PVC deletion policy for the NodeSet.
//
// If there are stale references or unexpected controllers, this returns true in order to not touch
// PVCs that have gotten into this unknown state. Otherwise the ownerships are checked to match the
// PVC retention policy:
// - Retain on scaling and nodeset deletion: no owner ref.
// - Retain on scaling and delete on nodeset deletion: owner ref on the nodeset only.
// - Delete on scaling and retain on nodeset deletion: owner ref on the pod only.
// - Delete on scaling and nodeset deletion: owner refs on both nodeset and pod.
func isClaimOwnerUpToDate(logger klog.Logger, claim *corev1.PersistentVolumeClaim, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) bool {
	if hasStaleOwnerRef(claim, nodeset, slinkyv1beta1.NodeSetGVK) || hasStaleOwnerRef(claim, pod, podGVK) {
		// The claim is being managed by previous, presumably deleted, version of the controller. It should not be touched.
		return true
	}

	if hasUnexpectedController(claim, nodeset, pod) {
		if hasOwnerRef(claim, nodeset) || hasOwnerRef(claim, pod) {
			return false // Need to clean up the conflicting controllers
		}
		// The claim refs are good, we don't want to add any controllers on top of the unexpected one.
		return true
	}

	if hasNonControllerOwner(claim, nodeset, pod) {
		// Some resource has an owner ref, but there is no controller. This needs to be updated.
		return false
	}

	policy := getPersistentVolumeClaimRetentionPolicy(nodeset)
	const delete = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
	const retain = slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType
	switch {
	default:
		logger.Error(nil, "Unknown policy, treating as Retain", "policy", nodeset.Spec.PersistentVolumeClaimRetentionPolicy)
		fallthrough
	case policy.WhenDeleted == retain && policy.WhenScaled == retain:
		if hasOwnerRef(claim, nodeset) || hasOwnerRef(claim, pod) {
			return false
		}
	case policy.WhenDeleted == delete && policy.WhenScaled == retain:
		if !hasOwnerRef(claim, nodeset) || hasOwnerRef(claim, pod) {
			return false
		}
	case policy.WhenDeleted == retain && policy.WhenScaled == delete:
		if hasOwnerRef(claim, nodeset) {
			return false
		}
		podScaledDown := podutils.IsPodCordon(pod)
		if podScaledDown != hasOwnerRef(claim, pod) {
			return false
		}
	case policy.WhenDeleted == delete && policy.WhenScaled == delete:
		podScaledDown := podutils.IsPodCordon(pod)
		// If a pod is scaled down, there should be no nodeset ref and a pod ref;
		// if the pod is not scaled down it's the other way around.
		if podScaledDown == hasOwnerRef(claim, nodeset) {
			return false
		}
		if podScaledDown != hasOwnerRef(claim, pod) {
			return false
		}
	}
	return true
}

// hasUnexpectedController returns true if the nodeset has a retention policy and there is a controller
// for the claim that's not the nodeset or pod. Since the retention policy may have been changed, it is
// always valid for the nodeset or pod to be a controller.
func hasUnexpectedController(claim *corev1.PersistentVolumeClaim, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) bool {
	policy := getPersistentVolumeClaimRetentionPolicy(nodeset)
	const retain = slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType
	if policy.WhenScaled == retain && policy.WhenDeleted == retain {
		// On a retain policy, it's not a problem for different controller to be managing the claims.
		return false
	}
	for _, ownerRef := range claim.GetOwnerReferences() {
		if matchesRef(&ownerRef, nodeset, slinkyv1beta1.NodeSetGVK) {
			if ownerRef.UID != nodeset.GetUID() {
				// A UID mismatch means that pods were incorrectly orphaned. Treating this as an unexpected
				// controller means we won't touch the PVCs (eg, leave it to the garbage collector to clean
				// up if appropriate).
				return true
			}
			continue // This is us.
		}

		if matchesRef(&ownerRef, pod, podGVK) {
			if ownerRef.UID != pod.GetUID() {
				// This is the same situation as the nodeset UID mismatch, above.
				return true
			}
			continue // This is us.
		}
		if ownerRef.Controller != nil && *ownerRef.Controller {
			return true // This is another controller.
		}
	}
	return false
}

// hasNonControllerOwner returns true if the pod or nodeset is an owner but not controller of the claim.
func hasNonControllerOwner(claim *corev1.PersistentVolumeClaim, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) bool {
	for _, ownerRef := range claim.GetOwnerReferences() {
		if ownerRef.UID == nodeset.GetUID() || ownerRef.UID == pod.GetUID() {
			if ownerRef.Controller == nil || !*ownerRef.Controller {
				return true
			}
		}
	}
	return false
}

// removeRefs removes any owner refs from the list matching predicate. Returns true if the list was changed and
// the new (or unchanged list).
func removeRefs(refs []metav1.OwnerReference, predicate func(ref *metav1.OwnerReference) bool) []metav1.OwnerReference {
	newRefs := []metav1.OwnerReference{}
	for _, ownerRef := range refs {
		if !predicate(&ownerRef) {
			newRefs = append(newRefs, ownerRef)
		}
	}
	return newRefs
}

// updateClaimOwnerRefForSetAndPod updates the ownerRefs for the claim according to the deletion policy of
// the NodeSet. Returns true if the claim was changed and should be updated and false otherwise.
// isClaimOwnerUpToDate should be called before this to avoid an expensive update operation.
func updateClaimOwnerRefForSetAndPod(logger klog.Logger, claim *corev1.PersistentVolumeClaim, nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) {
	refs := claim.GetOwnerReferences()

	unexpectedController := hasUnexpectedController(claim, nodeset, pod)

	// Scrub any ownerRefs to our nodeset & pod.
	refs = removeRefs(refs, func(ref *metav1.OwnerReference) bool {
		return matchesRef(ref, nodeset, slinkyv1beta1.NodeSetGVK) || matchesRef(ref, pod, podGVK)
	})

	if unexpectedController {
		// Leave ownerRefs to our nodeset & pod scrubed and return without creating new ones.
		claim.SetOwnerReferences(refs)
		return
	}

	policy := getPersistentVolumeClaimRetentionPolicy(nodeset)
	const retain = slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType
	const delete = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
	switch {
	default:
		logger.Error(nil, "Unknown policy, treating as Retain", "policy", nodeset.Spec.PersistentVolumeClaimRetentionPolicy)
		// Nothing to do
	case policy.WhenScaled == retain && policy.WhenDeleted == retain:
		// Nothing to do
	case policy.WhenScaled == retain && policy.WhenDeleted == delete:
		refs = addControllerRef(refs, nodeset, slinkyv1beta1.NodeSetGVK)
	case policy.WhenScaled == delete && policy.WhenDeleted == retain:
		podScaledDown := podutils.IsPodCordon(pod)
		if podScaledDown {
			refs = addControllerRef(refs, pod, podGVK)
		}
	case policy.WhenScaled == delete && policy.WhenDeleted == delete:
		podScaledDown := podutils.IsPodCordon(pod)
		if podScaledDown {
			refs = addControllerRef(refs, pod, podGVK)
		}
		if !podScaledDown {
			refs = addControllerRef(refs, nodeset, slinkyv1beta1.NodeSetGVK)
		}
	}
	claim.SetOwnerReferences(refs)
}

// getPersistentVolumeClaimPolicy returns the PVC policy for a NodeSet, returning a retain policy if the nodeset policy is nil.
func getPersistentVolumeClaimRetentionPolicy(nodeset *slinkyv1beta1.NodeSet) slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy {
	policy := slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
		WhenDeleted: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
		WhenScaled:  slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
	}
	if nodeset.Spec.PersistentVolumeClaimRetentionPolicy != nil {
		policy = *nodeset.Spec.PersistentVolumeClaimRetentionPolicy
	}
	return policy
}

// hasOwnerRef returns true if target has an ownerRef to owner (as its UID).
// This does not check if the owner is a controller.
func hasOwnerRef(target, owner metav1.Object) bool {
	ownerUID := owner.GetUID()
	for _, ownerRef := range target.GetOwnerReferences() {
		if ownerRef.UID == ownerUID {
			return true
		}
	}
	return false
}

// hasStaleOwnerRef returns true if target has a ref to owner that appears to be stale, that is,
// the ref matches the object but not the UID.
func hasStaleOwnerRef(target *corev1.PersistentVolumeClaim, obj metav1.Object, gvk schema.GroupVersionKind) bool {
	for _, ownerRef := range target.GetOwnerReferences() {
		if matchesRef(&ownerRef, obj, gvk) {
			return ownerRef.UID != obj.GetUID()
		}
	}
	return false
}

// matchesRef returns true when the object matches the owner reference, that is the name and GVK are the same.
func matchesRef(ref *metav1.OwnerReference, obj metav1.Object, gvk schema.GroupVersionKind) bool {
	return gvk.GroupVersion().String() == ref.APIVersion && gvk.Kind == ref.Kind && ref.Name == obj.GetName()
}

// addControllerRef returns refs with owner added as a controller, if necessary.
func addControllerRef(refs []metav1.OwnerReference, owner metav1.Object, gvk schema.GroupVersionKind) []metav1.OwnerReference {
	for _, ref := range refs {
		if ref.UID == owner.GetUID() {
			// Already added. Since we scrub our refs before making any changes, we know it's already
			// a controller if appropriate.
			return refs
		}
	}
	return append(refs, *metav1.NewControllerRef(owner, gvk))
}
