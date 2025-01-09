// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	v1helper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
	"k8s.io/klog/v2"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/daemon/util"
	"k8s.io/kubernetes/pkg/controller/history"
	"k8s.io/utils/integer"
	"k8s.io/utils/ptr"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/annotations"
	"github.com/SlinkyProject/slurm-operator/internal/utils"
)

// var patchCodec = scheme.Codecs.LegacyCodec(slinkyv1alpha1.SchemeGroupVersion)
var patchCodec = unstructured.UnstructuredJSONScheme

// isPodFromNodeSet returns if the name schema matches
func isPodFromNodeSet(set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) bool {
	found, err := regexp.MatchString(fmt.Sprintf("^%s-", set.Name), pod.Name)
	if err != nil {
		return false
	}
	return found
}

// identityMatches returns true if pod has a valid identity and network identity for a member of set.
func identityMatches(set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) bool {
	return isPodFromNodeSet(set, pod) &&
		pod.Namespace == set.Namespace
}

// storageMatches returns true if pod's Volumes cover the set of PersistentVolumeClaims
func storageMatches(set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) bool {
	volumes := make(map[string]corev1.Volume, len(pod.Spec.Volumes))
	for _, volume := range pod.Spec.Volumes {
		volumes[volume.Name] = volume
	}
	for _, claim := range set.Spec.VolumeClaimTemplates {
		volume, found := volumes[claim.Name]
		if !found ||
			volume.VolumeSource.PersistentVolumeClaim == nil ||
			volume.VolumeSource.PersistentVolumeClaim.ClaimName != getPersistentVolumeClaimName(set, &claim, pod.Spec.Hostname) {
			return false
		}
	}
	return true
}

// getPersistentVolumeClaimPolicy returns the PVC policy for a NodeSet, returning a retain policy if the set policy is nil.
func getPersistentVolumeClaimRetentionPolicy(set *slinkyv1alpha1.NodeSet) slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy {
	policy := slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
		WhenDeleted: slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType,
	}
	if set.Spec.PersistentVolumeClaimRetentionPolicy != nil {
		policy = ptr.Deref(set.Spec.PersistentVolumeClaimRetentionPolicy, slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{})
	}
	return policy
}

// claimOwnerMatchesSetAndPod returns false if the ownerRefs of the claim are not set consistently with the
// PVC deletion policy for the NodeSet.
func claimOwnerMatchesSetAndPod(logger klog.Logger, claim *corev1.PersistentVolumeClaim, set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) bool {
	policy := getPersistentVolumeClaimRetentionPolicy(set)
	const retain = slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType
	const delete = slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType
	switch {
	default:
		logger.Error(nil, "Unknown policy, treating as Retain",
			"policy", set.Spec.PersistentVolumeClaimRetentionPolicy)
		fallthrough
	case policy.WhenDeleted == retain:
		if !hasOwnerRef(claim, set) || hasOwnerRef(claim, pod) {
			return false
		}
	case policy.WhenDeleted == delete:
		if hasOwnerRef(claim, set) || !hasOwnerRef(claim, pod) {
			return false
		}
	}
	return true
}

// updateClaimOwnerRefForSetAndPod updates the ownerRefs for the claim according to the deletion policy of
// the NodeSet. Returns true if the claim was changed and should be updated and false otherwise.
func updateClaimOwnerRefForSetAndPod(
	logger klog.Logger,
	claim *corev1.PersistentVolumeClaim,
	set *slinkyv1alpha1.NodeSet,
	pod *corev1.Pod,
) bool {
	needsUpdate := false
	// Sometimes the version and kind are not set {pod,set}.TypeMeta. These are necessary for the ownerRef.
	// This is the case both in real clusters and the unittests.
	// TODO: there must be a better way to do this other than hardcoding the pod version?
	updateMeta := func(tm *metav1.TypeMeta, kind string) {
		if tm.APIVersion == "" {
			if kind == "NodeSet" {
				tm.APIVersion = slinkyv1alpha1.SchemeGroupVersion.String()
			} else {
				tm.APIVersion = "v1"
			}
		}
		if tm.Kind == "" {
			tm.Kind = kind
		}
	}
	podMeta := pod.TypeMeta
	updateMeta(&podMeta, "Pod")
	setMeta := set.TypeMeta
	updateMeta(&setMeta, "NodeSet")
	policy := getPersistentVolumeClaimRetentionPolicy(set)
	const retain = slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType
	const delete = slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType
	switch {
	default:
		logger.Error(nil, "Unknown policy, treating as Retain", "policy",
			set.Spec.PersistentVolumeClaimRetentionPolicy)
		fallthrough
	case policy.WhenDeleted == retain:
		needsUpdate = setOwnerRef(claim, set, &setMeta) || needsUpdate
		needsUpdate = removeOwnerRef(claim, pod) || needsUpdate
	case policy.WhenDeleted == delete:
		needsUpdate = removeOwnerRef(claim, set) || needsUpdate
		needsUpdate = setOwnerRef(claim, pod, &setMeta) || needsUpdate
	}
	return needsUpdate
}

// hasOwnerRef returns true if target has an ownerRef to owner.
func hasOwnerRef(target, owner metav1.Object) bool {
	ownerUID := owner.GetUID()
	for _, ownerRef := range target.GetOwnerReferences() {
		if ownerRef.UID == ownerUID {
			return true
		}
	}
	return false
}

// hasStaleOwnerRef returns true if target has a ref to owner that appears to be stale.
func hasStaleOwnerRef(target, owner metav1.Object) bool {
	for _, ownerRef := range target.GetOwnerReferences() {
		if ownerRef.Name == owner.GetName() && ownerRef.UID != owner.GetUID() {
			return true
		}
	}
	return false
}

// setOwnerRef adds owner to the ownerRefs of target, if necessary. Returns true if target needs to be
// updated and false otherwise.
func setOwnerRef(target, owner metav1.Object, ownerType *metav1.TypeMeta) bool {
	if hasOwnerRef(target, owner) {
		return false
	}
	ownerRefs := append(
		target.GetOwnerReferences(),
		metav1.OwnerReference{
			APIVersion: ownerType.APIVersion,
			Kind:       ownerType.Kind,
			Name:       owner.GetName(),
			UID:        owner.GetUID(),
		})
	target.SetOwnerReferences(ownerRefs)
	return true
}

// removeOwnerRef removes owner from the ownerRefs of target, if necessary. Returns true if target needs
// to be updated and false otherwise.
func removeOwnerRef(target, owner metav1.Object) bool {
	if !hasOwnerRef(target, owner) {
		return false
	}
	ownerUID := owner.GetUID()
	oldRefs := target.GetOwnerReferences()
	newRefs := make([]metav1.OwnerReference, len(oldRefs)-1)
	skip := 0
	for i := range oldRefs {
		if oldRefs[i].UID == ownerUID {
			skip = -1
		} else {
			newRefs[i+skip] = oldRefs[i]
		}
	}
	target.SetOwnerReferences(newRefs)
	return true
}

// getPersistentVolumeClaimName gets the name of PersistentVolumeClaim for a Pod with the host.
// Claim must be a PersistentVolumeClaim from set's VolumeClaimTemplates.
func getPersistentVolumeClaimName(set *slinkyv1alpha1.NodeSet, claim *corev1.PersistentVolumeClaim, host string) string {
	return fmt.Sprintf("%s-%s-%s", claim.Name, set.Name, host)
}

// getPersistentVolumeClaims gets a map of PersistentVolumeClaims to their template names, as defined in set. The
// returned PersistentVolumeClaims are each constructed with a the name specific to the Pod. This name is determined
// by getPersistentVolumeClaimName.
func getPersistentVolumeClaims(set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) map[string]corev1.PersistentVolumeClaim {
	templates := set.Spec.VolumeClaimTemplates
	claims := make(map[string]corev1.PersistentVolumeClaim, len(templates))
	for i := range templates {
		claim := templates[i]
		claim.Name = getPersistentVolumeClaimName(set, &claim, pod.Spec.Hostname)
		claim.Namespace = set.Namespace
		claim.Labels = set.Spec.Selector.MatchLabels
		claims[templates[i].Name] = claim
	}
	return claims
}

// updateStorage updates pod's Volumes to conform with the PersistentVolumeClaim of set's templates. If pod has
// conflicting local Volumes these are replaced with Volumes that conform to the set's templates.
func updateStorage(set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) {
	currentVolumes := pod.Spec.Volumes
	claims := getPersistentVolumeClaims(set, pod)
	newVolumes := make([]corev1.Volume, 0, len(claims))
	for name, claim := range claims {
		newVolumes = append(newVolumes, corev1.Volume{
			Name: name,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: claim.Name,
					// TODO: Use source definition to set this value when we have one.
					ReadOnly: false,
				},
			},
		})
	}
	for i := range currentVolumes {
		if _, ok := claims[currentVolumes[i].Name]; !ok {
			newVolumes = append(newVolumes, currentVolumes[i])
		}
	}
	pod.Spec.Volumes = newVolumes
}

// initIdentity initializes the pod's identity on the network.
func initIdentity(set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) {
	updateIdentity(set, pod)
	// Set these immutable fields only on initial Pod creation, not updates.
	pod.Spec.Hostname = pod.Name
	pod.Spec.Subdomain = set.Spec.ServiceName
}

// updateIdentity updates pod's name, hostname, and subdomain to conform to set's name
// and headless service.
func updateIdentity(set *slinkyv1alpha1.NodeSet, pod *corev1.Pod) {
	pod.Namespace = set.Namespace
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
}

// setPodRevision sets the revision of Pod to revision by adding the NodeSetRevisionLabel
func setPodRevision(pod *corev1.Pod, revision string) {
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[slinkyv1alpha1.NodeSetRevisionLabel] = revision
}

// getPodRevision gets the revision of Pod by inspecting the NodeSetRevisionLabel. If pod has no revision the empty
// string is returned.
func getPodRevision(pod *corev1.Pod) string {
	if pod.Labels == nil {
		return ""
	}
	return pod.Labels[slinkyv1alpha1.NodeSetRevisionLabel]
}

// getNodeSetRevisionLabel gets the controller revision hash for the given revision.
func getNodeSetRevisionLabel(revision *appsv1.ControllerRevision) string {
	if revision.Labels == nil {
		return ""
	}
	return revision.Labels[history.ControllerRevisionHashLabel]
}

// updateNodeSetPodAntiAffinity will add PodAffinity
func updateNodeSetPodAntiAffinity(affinity *corev1.Affinity) *corev1.Affinity {
	labelSelectorRequirement := metav1.LabelSelectorRequirement{
		Key:      "app.kubernetes.io/name",
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{"slurmd"},
	}

	podAffinityTerm := corev1.PodAffinityTerm{
		TopologyKey: corev1.LabelHostname,
		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				labelSelectorRequirement,
			},
		},
	}

	podAffinityTerms := []corev1.PodAffinityTerm{
		podAffinityTerm,
	}

	if affinity == nil {
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: podAffinityTerms,
			},
		}
	}

	if affinity.PodAntiAffinity == nil {
		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: podAffinityTerms,
		}
		return affinity
	}

	podAntiAffinity := affinity.PodAntiAffinity

	if podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = podAffinityTerms
		return affinity
	}

	podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution = append(podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, podAffinityTerms...)

	return affinity
}

// newNodeSetPod returns a new Pod conforming to the set's Spec with an identity generated from ordinal.
func newNodeSetPod(set *slinkyv1alpha1.NodeSet, nodeName, hash string) *corev1.Pod {
	pod, err := controller.GetPodFromTemplate(&set.Spec.Template, set, metav1.NewControllerRef(set, controllerKind))
	if err != nil {
		panic(err)
	}

	initIdentity(set, pod)

	// Do not set nodeName field otherwise Pod scheduler will be avoided
	// and priorityClass will not be honored.
	// newPod.Spec.NodeName = nodeName

	// Added default tolerations for NodeSet pods, pinning Pod to Node by nodeName.
	util.AddOrUpdateDaemonPodTolerations(&pod.Spec)

	// The pod's NodeAffinity will be updated to make sure the Pod is bound
	// to the target node by default scheduler. It is safe to do so because there
	// should be no conflicting node affinity with the target node.
	pod.Spec.Affinity = util.ReplaceDaemonSetPodNodeNameNodeAffinity(pod.Spec.Affinity, nodeName)

	// The pod's PodAntiAffinity will be updated to make sure the Pod is not
	// scheduled on the same Node as another NodeSet Pod.
	pod.Spec.Affinity = updateNodeSetPodAntiAffinity(pod.Spec.Affinity)

	// Set pod hostname to match the targeted node name to let user
	// better correlate the Slurm node with the Kubernetes node.
	pod.Spec.Hostname = getSafeNodeName(nodeName)

	setPodRevision(pod, hash)
	updateIdentity(set, pod)
	updateStorage(set, pod)
	return pod
}

// getPatch returns a strategic merge patch that can be applied to restore a NodeSet to a
// previous version. If the returned error is nil the patch is valid. The current state that we save is just the
// PodSpecTemplate. We can modify this later to encompass more state (or less) and remain compatible with previously
// recorded patches.
func getPatch(set *slinkyv1alpha1.NodeSet) ([]byte, error) {
	setBytes, err := runtime.Encode(patchCodec, set)
	// setBytes, err := json.Marshal(set)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	err = json.Unmarshal(setBytes, &raw)
	if err != nil {
		return nil, err
	}
	objCopy := make(map[string]any)
	specCopy := make(map[string]any)

	// Create a patch of the NodeSet that replaces spec.template
	spec := raw["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	specCopy["template"] = template
	template["$patch"] = "replace"
	objCopy["spec"] = specCopy
	patch, err := json.Marshal(objCopy)
	return patch, err
}

// Match check if the given NodeSet's template matches the template stored in the given history.
func Match(set *slinkyv1alpha1.NodeSet, history *appsv1.ControllerRevision) (bool, error) {
	patch, err := getPatch(set)
	if err != nil {
		return false, err
	}
	return bytes.Equal(patch, history.Data.Raw), nil
}

// ApplyRevision returns a new NodeSet constructed by restoring the state in revision to set. If the returned error
// is nil, the returned NodeSet is valid.
func ApplyRevision(set *slinkyv1alpha1.NodeSet, revision *appsv1.ControllerRevision) (*slinkyv1alpha1.NodeSet, error) {
	clone := set.DeepCopy()
	patched, err := strategicpatch.StrategicMergePatch([]byte(runtime.EncodeOrDie(patchCodec, clone)), revision.Data.Raw, clone)
	if err != nil {
		return nil, err
	}
	restoredSet := &slinkyv1alpha1.NodeSet{}
	err = json.Unmarshal(patched, restoredSet)
	if err != nil {
		return nil, err
	}
	return restoredSet, nil
}

// nodeInSameCondition returns true if all effective types ("Status" is true) equals;
// otherwise, returns false.
func nodeInSameCondition(old []corev1.NodeCondition, cur []corev1.NodeCondition) bool {
	if len(old) == 0 && len(cur) == 0 {
		return true
	}

	c1map := map[corev1.NodeConditionType]corev1.ConditionStatus{}
	for _, c := range old {
		if c.Status == corev1.ConditionTrue {
			c1map[c.Type] = c.Status
		}
	}

	for _, c := range cur {
		if c.Status != corev1.ConditionTrue {
			continue
		}

		if _, found := c1map[c.Type]; !found {
			return false
		}

		delete(c1map, c.Type)
	}

	return len(c1map) == 0
}

func failedPodsBackoffKey(set *slinkyv1alpha1.NodeSet, nodeName string) string {
	return fmt.Sprintf("%s/%d/%s", set.UID, set.Status.ObservedGeneration, nodeName)
}

// Predicates checks if a NodeSet's pod can run on a node.
func predicates(pod *corev1.Pod, node *corev1.Node, taints []corev1.Taint) (fitsNodeName, fitsNodeAffinity, fitsTaints bool) {
	fitsNodeName = len(pod.Spec.NodeName) == 0 || pod.Spec.NodeName == node.Name
	// Ignore parsing errors for backwards compatibility.
	fitsNodeAffinity, _ = nodeaffinity.GetRequiredNodeAffinity(pod).Match(node)
	_, hasUntoleratedTaint := v1helper.FindMatchingUntoleratedTaint(taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoExecute || t.Effect == corev1.TaintEffectNoSchedule
	})
	fitsTaints = !hasUntoleratedTaint
	return
}

// nodeShouldRunNodeSetPod checks a set of preconditions against a (node,nodeset) and returns a
// summary. Returned booleans are:
//   - shouldRun:
//     Returns true when a nodeset should run on the node if a nodeset pod is not already
//     running on that node.
//   - shouldContinueRunning:
//     Returns true when a nodeset should continue running on a node if a nodeset pod is already
//     running on that node.
func nodeShouldRunNodeSetPod(node *corev1.Node, set *slinkyv1alpha1.NodeSet) (bool, bool) {
	pod := newNodeSetPod(set, node.Name, "")

	// If the nodeset specifies a node name, check that it matches with node.Name.
	if !(set.Spec.Template.Spec.NodeName == "" || set.Spec.Template.Spec.NodeName == node.Name) {
		return false, false
	}

	taints := node.Spec.Taints
	fitsNodeName, fitsNodeAffinity, fitsTaints := predicates(pod, node, taints)
	if !fitsNodeName || !fitsNodeAffinity {
		return false, false
	}

	if !fitsTaints {
		// Scheduled nodeset pods should continue running if they tolerate NoExecute taint.
		_, hasUntoleratedTaint := v1helper.FindMatchingUntoleratedTaint(taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
			return t.Effect == corev1.TaintEffectNoExecute
		})
		return false, !hasUntoleratedTaint
	}

	return true, true
}

// isNodeSetPodAvailable returns true if a pod is ready after update progress.
func isNodeSetPodAvailable(pod *corev1.Pod, minReadySeconds int32, now metav1.Time) bool {
	return podutil.IsPodAvailable(pod, minReadySeconds, now)
}

// isNodeSetCreationProgressively returns true if and only if the progressive annotation is set to true.
func isNodeSetCreationProgressively(set *slinkyv1alpha1.NodeSet) bool {
	return set.Annotations[annotations.PodProgressiveCreate] == "true"
}

// isNodeSetPodCordon returns true if and only if the cordon annotation is set to true.
func isNodeSetPodCordon(pod metav1.Object) bool {
	return pod.GetAnnotations()[annotations.PodCordon] == "true"
}

// isNodeSetPodDelete returns true if and only if the delete annotation is set to true.
func isNodeSetPodDelete(obj metav1.Object) bool {
	return obj.GetAnnotations()[annotations.PodDelete] == "true"
}

// getNodesNeedingPods finds which nodes should run nodeset pod according to progressive flag and parititon.
func getNodesNeedingPods(
	newPodsNum, desire, partition int,
	progressive bool,
	nodesNeedingPods []*corev1.Node,
) []*corev1.Node {
	if !progressive {
		sort.Sort(utils.NodeByWeight(nodesNeedingPods))
		return nodesNeedingPods
	}

	// partition must be less than total number and greater than zero.
	partition = integer.IntMax(integer.IntMin(partition, desire), 0)

	maxCreate := integer.IntMax(desire-newPodsNum-partition, 0)
	if maxCreate > len(nodesNeedingPods) {
		maxCreate = len(nodesNeedingPods)
	}

	if maxCreate > 0 {
		sort.Sort(utils.NodeByWeight(nodesNeedingPods))
		nodesNeedingPods = nodesNeedingPods[:maxCreate]
	} else {
		nodesNeedingPods = []*corev1.Node{}
	}

	return nodesNeedingPods
}

func isNodeSetPaused(set *slinkyv1alpha1.NodeSet) bool {
	return set.Spec.UpdateStrategy.RollingUpdate != nil &&
		ptr.Deref(set.Spec.UpdateStrategy.RollingUpdate.Paused, false)
}

// unavailableCount returns 0 if unavailability is not requested, the expected
// unavailability number to allow out of numberToSchedule if requested, or an error if
// the unavailability percentage requested is invalid.
func unavailableCount(set *slinkyv1alpha1.NodeSet, numberToSchedule int) (int, error) {
	if set.Spec.UpdateStrategy.Type != slinkyv1alpha1.RollingUpdateNodeSetStrategyType {
		return 0, nil
	}
	r := set.Spec.UpdateStrategy.RollingUpdate
	if r == nil {
		return 0, nil
	}
	if r.MaxUnavailable == nil {
		return 0, nil
	}
	return intstr.GetScaledValueFromIntOrPercent(r.MaxUnavailable, numberToSchedule, true)
}

// getUnscheduledPodsWithoutNode returns list of unscheduled pods assigned to not existing nodes.
// Returned pods cannot be deleted by PodGCController so they should be deleted by NodeSetController.
func getUnscheduledPodsWithoutNode(
	runningNodesList []*corev1.Node,
	nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod,
) []*corev1.Pod {
	var results []*corev1.Pod
	isNodeRunning := make(map[*corev1.Node]bool)
	for _, node := range runningNodesList {
		isNodeRunning[node] = true
	}
	for node, pods := range nodeToNodeSetPods {
		if !isNodeRunning[node] {
			for _, pod := range pods {
				if len(pod.Spec.NodeName) == 0 {
					results = append(results, pod)
				}
			}
		}
	}
	return results
}

// findUpdatedPodsOnNode looks at non-deleted pods on a given node and returns true if there
// is at most one of each old and new pods, or false if there are multiples. We can skip
// processing the particular node in those scenarios and let the manage loop prune the
// excess pods for our next time around.
func findUpdatedPodsOnNode(
	set *slinkyv1alpha1.NodeSet,
	podsOnNode []*corev1.Pod,
	hash string,
) (newPod, oldPod *corev1.Pod, ok bool) {
	for _, pod := range podsOnNode {
		if utils.IsTerminating(pod) {
			continue
		}
		generation, err := GetTemplateGeneration(set)
		if err != nil {
			generation = nil
		}
		if util.IsPodUpdated(pod, hash, generation) {
			if newPod != nil {
				return nil, nil, false
			}
			newPod = pod
		} else {
			if oldPod != nil {
				return nil, nil, false
			}
			oldPod = pod
		}
	}
	return newPod, oldPod, true
}

// Ensure we don't use FQDN for nodeName as setting it as the hostName is not supported yet.
func getSafeNodeName(nodeName string) string {
	return strings.ReplaceAll(nodeName, ".", "-")
}
