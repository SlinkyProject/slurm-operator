// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	corev1helper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
	"k8s.io/klog/v2"
	k8scontroller "k8s.io/kubernetes/pkg/controller"
	daemonutils "k8s.io/kubernetes/pkg/controller/daemon/util"
	"k8s.io/kubernetes/pkg/features"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	builder "github.com/SlinkyProject/slurm-operator/internal/builder/workerbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
)

// NewNodeSetStatefulSetPod returns a new Pod conforming to the nodeset's Spec with an identity generated from ordinal.
func NewNodeSetStatefulSetPod(
	client client.Client,
	nodeset *slinkyv1beta1.NodeSet,
	controller *slinkyv1beta1.Controller,
	ordinal int,
	revisionHash string,
) *corev1.Pod {
	controllerRef := metav1.NewControllerRef(nodeset, slinkyv1beta1.NodeSetGVK)
	podTemplate := builder.New(client).BuildWorkerPodTemplate(nodeset, controller)
	pod, _ := k8scontroller.GetPodFromTemplate(&podTemplate, nodeset, controllerRef)
	pod.Name = GetOrdinalPodName(nodeset, ordinal)
	initIdentity(nodeset, pod)
	UpdateStorage(nodeset, pod)

	if revisionHash != "" {
		historycontrol.SetRevision(pod.Labels, revisionHash)
	}

	// The pod's PodAntiAffinity will be updated to make sure the Pod is not
	// scheduled on the same Node as another NodeSet pod.
	pod.Spec.Affinity = updateNodeSetPodAntiAffinity(pod.Spec.Affinity)

	// Ensure recreated pods are pinned to their node, but only if they still match their Node.
	if nodeset.Spec.PinToNode {
		pinPodToNode(client, nodeset.Status.OrdinalToNode, pod, ordinal)
	}

	// WARNING: Do not use the spec.NodeName otherwise the Pod scheduler will
	// be avoided and priorityClass will not be honored.
	pod.Spec.NodeName = ""

	return pod
}

// pinPodToNode will modify the input Pod with its Node affinity if the pin is valid
func pinPodToNode(kclient client.Client, ordinalToNode map[string]string, pod *corev1.Pod, ordinal int) {
	nodeName, ok := ordinalToNode[strconv.Itoa(ordinal)]
	if !ok {
		return
	}

	ctx := context.TODO()
	node := &corev1.Node{}
	nodeKey := types.NamespacedName{Name: nodeName}
	if err := kclient.Get(ctx, nodeKey, node); err != nil {
		return
	}
	if shouldRun, _ := PodShouldRunOnNode(ctx, pod, node); !shouldRun {
		return
	}

	pod.Spec.Affinity = daemonutils.ReplaceDaemonSetPodNodeNameNodeAffinity(pod.Spec.Affinity, nodeName)
}

func NewNodeSetDaemonSetPod(
	client client.Client,
	nodeset *slinkyv1beta1.NodeSet,
	controller *slinkyv1beta1.Controller,
	nodeName string,
	revisionHash string,
) *corev1.Pod {
	controllerRef := metav1.NewControllerRef(nodeset, slinkyv1beta1.NodeSetGVK)
	podTemplate := builder.New(client).BuildWorkerPodTemplate(nodeset, controller)
	pod, _ := k8scontroller.GetPodFromTemplate(&podTemplate, nodeset, controllerRef)

	// Ensure the hostname is RFC 1178 compliant
	safeHostname := getDaemonSetPodHostname(nodeName)
	pod.Spec.Hostname = safeHostname
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}

	initIdentity(nodeset, pod)
	UpdateStorage(nodeset, pod)

	if revisionHash != "" {
		historycontrol.SetRevision(pod.Labels, revisionHash)
	}

	// The pod's PodAntiAffinity will be updated to make sure the Pod is not
	// scheduled on the same Node as another NodeSet pod.
	pod.Spec.Affinity = updateNodeSetPodAntiAffinity(pod.Spec.Affinity)

	// WARNING: Do not use the spec.NodeName otherwise the Pod scheduler will
	// be avoided and priorityClass will not be honored.
	pod.Spec.NodeName = ""

	pod.GenerateName = nodeset.Name + "-"
	pod.Name = ""
	pod.Spec.Affinity = daemonutils.ReplaceDaemonSetPodNodeNameNodeAffinity(pod.Spec.Affinity, nodeName)

	return pod
}

// NewNodeSetSimulatedPod returns a simulated Pod for predicate
// evaluation. Unlike NewNodeSetDaemonSetPod, it preserves the user's node
// affinity by setting spec.nodeName directly instead of using
// ReplaceDaemonSetPodNodeNameNodeAffinity, which overwrites the
// RequiredDuringSchedulingIgnoredDuringExecution terms.
func NewNodeSetSimulatedPod(
	client client.Client,
	nodeset *slinkyv1beta1.NodeSet,
	controller *slinkyv1beta1.Controller,
	nodeName string,
) *corev1.Pod {
	controllerRef := metav1.NewControllerRef(nodeset, slinkyv1beta1.NodeSetGVK)
	podTemplate := builder.New(client).BuildWorkerPodTemplate(nodeset, controller)
	pod, _ := k8scontroller.GetPodFromTemplate(&podTemplate, nodeset, controllerRef)
	pod.Spec.NodeName = nodeName
	return pod
}

func getDaemonSetPodHostname(nodeName string) string {
	name := nodeName
	if before, _, ok := strings.Cut(nodeName, "."); ok {
		name = before
	}
	return name
}

func initIdentity(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) {
	UpdateIdentity(nodeset, pod)
	// Set these immutable fields only on initial Pod creation, not updates.
	if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeStatefulset {
		ordinal := GetOrdinal(pod)
		paddedOrdinal := GetPaddedOrdinal(nodeset, ordinal)
		pod.Name = GetOrdinalPodName(nodeset, ordinal)
		if pod.Spec.Hostname != "" {
			pod.Spec.Hostname = fmt.Sprintf("%s%s", pod.Spec.Hostname, paddedOrdinal)
		} else {
			pod.Spec.Hostname = pod.Name
		}
	}
	pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname] = GetSlurmNodeName(pod)
	pod.Labels[slinkyv1beta1.LabelNodeSetScalingMode] = string(nodeset.Spec.ScalingMode)
}

// UpdateIdentity updates pod's labels.
func UpdateIdentity(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) {
	pod.Namespace = nodeset.Namespace
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeStatefulset {
		ordinal := GetOrdinal(pod)
		paddedOrdinal := GetPaddedOrdinal(nodeset, ordinal)
		pod.Labels[slinkyv1beta1.LabelNodeSetPodIndex] = paddedOrdinal
	}
	pod.Labels[slinkyv1beta1.LabelNodeSetPodName] = pod.Name
	pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname] = GetSlurmNodeName(pod)
}

// UpdateStorage updates pod's Volumes to conform with the PersistentVolumeClaim of nodeset's templates. If pod has
// conflicting local Volumes these are replaced with Volumes that conform to the nodeset's templates.
func UpdateStorage(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) {
	currentVolumes := pod.Spec.Volumes
	claims := GetPersistentVolumeClaims(nodeset, pod)
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

// updateNodeSetPodAntiAffinity will add PodAntiAffinity such that a Kube node can only have one NodeSet pod.
func updateNodeSetPodAntiAffinity(affinity *corev1.Affinity) *corev1.Affinity {
	labelSelectorRequirement := metav1.LabelSelectorRequirement{
		Key:      labels.AppLabel,
		Operator: metav1.LabelSelectorOpIn,
		Values:   []string{labels.WorkerApp},
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

// IsPodFromNodeSet returns if the name schema matches
func IsPodFromNodeSet(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) bool {
	found, err := regexp.MatchString(fmt.Sprintf("^%s-", nodeset.Name), pod.Name)
	if err != nil {
		return false
	}
	return found
}

// GetOrdinal gets pod's ordinal. If pod has no ordinal, -1 is returned.
func GetOrdinal(pod *corev1.Pod) int {
	_, ordinal := GetParentNameAndOrdinal(pod)
	return ordinal
}

// nodesetPodRegex is a regular expression that extracts the parent NodeSet and ordinal from the Name of a Pod
var nodesetPodRegex = regexp.MustCompile("(.*)-([0-9]+)$")

// GetParentNameAndOrdinal gets the name of pod's parent NodeSet and pod's ordinal as extracted from its Name. If
// the Pod was not created by a NodeSet, its parent is considered to be empty string, and its ordinal is considered
// to be -1.
func GetParentNameAndOrdinal(pod *corev1.Pod) (string, int) {
	parent := ""
	ordinal := -1
	subMatches := nodesetPodRegex.FindStringSubmatch(pod.Name)
	if len(subMatches) < 3 {
		return parent, ordinal
	}
	parent = subMatches[1]
	if i, err := strconv.ParseInt(subMatches[2], 10, 32); err == nil {
		ordinal = int(i)
	}
	return parent, ordinal
}

// GetPaddedOrdinal gets the name of nodeset's child Pod with an ordinal index of ordinal
func GetPaddedOrdinal(nodeset *slinkyv1beta1.NodeSet, ordinal int) string {
	format := fmt.Sprintf("%%0%vd", nodeset.Spec.OrdinalPadding)
	return fmt.Sprintf(format, ordinal)
}

// GetOrdinalPodName gets the name of nodeset's child Pod with an ordinal index of ordinal
func GetOrdinalPodName(nodeset *slinkyv1beta1.NodeSet, ordinal int) string {
	paddedOrdinal := GetPaddedOrdinal(nodeset, ordinal)
	return fmt.Sprintf("%s-%s", nodeset.Name, paddedOrdinal)
}

// GetSlurmNodeName returns the Slurm node name.
func GetSlurmNodeName(pod *corev1.Pod) string {
	if pod.Labels[slinkyv1beta1.LabelNodeSetScalingMode] == string(slinkyv1beta1.ScalingModeStatefulset) {
		if pod.Spec.HostNetwork {
			return pod.Spec.NodeName
		}
		if pod.Spec.Hostname != "" {
			return pod.Spec.Hostname
		}
		return pod.Name
	} else {
		return pod.Spec.Hostname
	}
}

// IsIdentityMatch returns true if pod has a valid identity and network identity for a member of nodeset.
func IsIdentityMatch(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) bool {
	if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeStatefulset {
		parent, ordinal := GetParentNameAndOrdinal(pod)
		return ordinal >= 0 &&
			nodeset.Name == parent &&
			pod.Namespace == nodeset.Namespace &&
			pod.Labels[slinkyv1beta1.LabelNodeSetPodName] == pod.Name
	}
	return nodeset.Name == strings.TrimSuffix(pod.GenerateName, "-") &&
		pod.Namespace == nodeset.Namespace &&
		pod.Labels[slinkyv1beta1.LabelNodeSetPodName] == pod.Name
}

// IsStorageMatch returns true if pod's Volumes cover the nodeset of PersistentVolumeClaims
func IsStorageMatch(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) bool {
	var paddedOrdinal string
	if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeStatefulset {
		ordinal := GetOrdinal(pod)
		paddedOrdinal = GetPaddedOrdinal(nodeset, ordinal)
		if ordinal < 0 {
			return false
		}
	}
	volumes := make(map[string]corev1.Volume, len(pod.Spec.Volumes))
	for _, volume := range pod.Spec.Volumes {
		volumes[volume.Name] = volume
	}
	for _, claim := range nodeset.Spec.VolumeClaimTemplates {
		volume, found := volumes[claim.Name]
		if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeStatefulset {
			if !found ||
				volume.PersistentVolumeClaim == nil ||
				volume.PersistentVolumeClaim.ClaimName !=
					GetPersistentVolumeClaimNameOrdinal(nodeset, &claim, paddedOrdinal) {
				return false
			}
		} else {
			if !found ||
				volume.PersistentVolumeClaim == nil ||
				volume.PersistentVolumeClaim.ClaimName !=
					GetPersistentVolumeClaimNameNodeName(nodeset, &claim, pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname]) {
				return false
			}
		}
	}
	return true
}

// GetPersistentVolumeClaims gets a map of PersistentVolumeClaims to their template names, as defined in nodeset. The
// returned PersistentVolumeClaims are each constructed with a the name specific to the Pod. This name is determined
// by GetPersistentVolumeClaimName.
func GetPersistentVolumeClaims(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) map[string]corev1.PersistentVolumeClaim {
	var paddedOrdinal string
	if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeStatefulset {
		ordinal := GetOrdinal(pod)
		paddedOrdinal = GetPaddedOrdinal(nodeset, ordinal)
	}
	templates := nodeset.Spec.VolumeClaimTemplates
	selectorLabels := labels.NewBuilder().WithWorkerSelectorLabels(nodeset).Build()
	claims := make(map[string]corev1.PersistentVolumeClaim, len(templates))
	for i := range templates {
		claim := templates[i].DeepCopy()
		if nodeset.Spec.ScalingMode == slinkyv1beta1.ScalingModeStatefulset {
			claim.Name = GetPersistentVolumeClaimNameOrdinal(nodeset, claim, paddedOrdinal)
		} else {
			claim.Name = GetPersistentVolumeClaimNameNodeName(nodeset, claim, pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname])
		}
		claim.Namespace = nodeset.Namespace
		if claim.Labels != nil {
			maps.Copy(claim.Labels, selectorLabels)
		} else {
			claim.Labels = selectorLabels
		}
		claims[templates[i].Name] = *claim
	}
	return claims
}

// GetPersistentVolumeClaimNameOrdinal gets the name of PersistentVolumeClaim for a Pod with an ordinal index of ordinal. claim
// must be a PersistentVolumeClaim from nodeset's VolumeClaims template.
func GetPersistentVolumeClaimNameOrdinal(nodeset *slinkyv1beta1.NodeSet, claim *corev1.PersistentVolumeClaim, paddedOrdinal string) string {
	// NOTE: This name format is used by the heuristics for zone spreading in ChooseZoneForVolume
	return fmt.Sprintf("%s-%s-%s", claim.Name, nodeset.Name, paddedOrdinal)
}

// GetPersistentVolumeClaimNameNodeName gets the name of PersistentVolumeClaim for a Pod with a node name. claim
// must be a PersistentVolumeClaim from nodeset's VolumeClaims template.
func GetPersistentVolumeClaimNameNodeName(nodeset *slinkyv1beta1.NodeSet, claim *corev1.PersistentVolumeClaim, nodeName string) string {
	// NOTE: This name format is used by the heuristics for zone spreading in ChooseZoneForVolume
	return fmt.Sprintf("%s-%s-%s", claim.Name, nodeset.Name, nodeName)
}

// SetOwnerReferences modifies the object with all NodeSets as non-controller owners.
func SetOwnerReferences(r client.Client, ctx context.Context, object metav1.Object, clusterName string) error {
	nodesetList := &slinkyv1beta1.NodeSetList{}
	if err := r.List(ctx, nodesetList); err != nil {
		return err
	}
	sort.Slice(nodesetList.Items, func(i, j int) bool {
		return nodesetList.Items[i].Name < nodesetList.Items[j].Name
	})

	opts := []controllerutil.OwnerReferenceOption{
		controllerutil.WithBlockOwnerDeletion(true),
	}
	for _, nodeset := range nodesetList.Items {
		if nodeset.Spec.ControllerRef.Name != clusterName {
			continue
		}
		if err := controllerutil.SetOwnerReference(&nodeset, object, r.Scheme(), opts...); err != nil {
			return fmt.Errorf("failed to set owner: %w", err)
		}
	}

	return nil
}

// PodShouldRunOnNode checks pod preconditions against a node and returns a summary.
// Returned booleans are:
//   - shouldRun:
//     Returns true when a pod should run on the node if a pod is not already
//     running on that node.
//   - shouldContinueRunning:
//     Returns true when a should continue running on a node if a pod is already
//     running on that node.
func PodShouldRunOnNode(ctx context.Context, pod *corev1.Pod, node *corev1.Node) (shouldRun bool, shouldContinueRunning bool) {
	logger := log.FromContext(ctx)

	taints := node.Spec.Taints
	fitsNodeName, fitsNodeAffinity, fitsTaints := predicates(logger, pod, node, taints)
	if !fitsNodeName || !fitsNodeAffinity {
		return false, false
	}

	if !fitsTaints {
		// Scheduled pods should continue running if they tolerate NoExecute taint.
		_, hasUntoleratedTaint := corev1helper.FindMatchingUntoleratedTaint(logger, taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
			return t.Effect == corev1.TaintEffectNoExecute
		}, utilfeature.DefaultFeatureGate.Enabled(features.TaintTolerationComparisonOperators))
		return false, !hasUntoleratedTaint
	}

	return true, true
}

func predicates(logger klog.Logger, pod *corev1.Pod, node *corev1.Node, taints []corev1.Taint) (fitsNodeName, fitsNodeAffinity, fitsTaints bool) {
	fitsNodeName = len(pod.Spec.NodeName) == 0 || pod.Spec.NodeName == node.Name
	// Ignore parsing errors for backwards compatibility.
	fitsNodeAffinity, _ = nodeaffinity.GetRequiredNodeAffinity(pod).Match(node)
	_, hasUntoleratedTaint := corev1helper.FindMatchingUntoleratedTaint(logger, taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoExecute || t.Effect == corev1.TaintEffectNoSchedule
	}, utilfeature.DefaultFeatureGate.Enabled(features.TaintTolerationComparisonOperators))
	fitsTaints = !hasUntoleratedTaint
	return fitsNodeName, fitsNodeAffinity, fitsTaints
}
