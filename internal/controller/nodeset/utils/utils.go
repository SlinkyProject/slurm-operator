// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8scontroller "k8s.io/kubernetes/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	builder "github.com/SlinkyProject/slurm-operator/internal/builder/workerbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
)

// NewNodeSetPod returns a new Pod conforming to the nodeset's Spec with an identity generated from ordinal.
func NewNodeSetPod(
	client client.Client,
	nodeset *slinkyv1beta1.NodeSet,
	controller *slinkyv1beta1.Controller,
	ordinal int,
	revisionHash string,
) *corev1.Pod {
	controllerRef := metav1.NewControllerRef(nodeset, slinkyv1beta1.NodeSetGVK)
	podTemplate := builder.New(client).BuildWorkerPodTemplate(nodeset, controller)
	pod, _ := k8scontroller.GetPodFromTemplate(&podTemplate, nodeset, controllerRef)
	pod.Name = GetPodName(nodeset, ordinal)
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

	return pod
}

func initIdentity(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) {
	UpdateIdentity(nodeset, pod)
	// Set these immutable fields only on initial Pod creation, not updates.
	if pod.Spec.Hostname != "" {
		ordinal := GetOrdinal(pod)
		paddedOrdinal := GetPaddedOrdinal(nodeset, ordinal)
		pod.Spec.Hostname = fmt.Sprintf("%s%s", pod.Spec.Hostname, paddedOrdinal)
	} else {
		pod.Spec.Hostname = pod.Name
	}
	pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname] = GetNodeName(pod)
}

// UpdateIdentity updates pod's name, hostname, and subdomain, and StatefulSetPodNameLabel to conform to nodeset's name
// and headless service.
func UpdateIdentity(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) {
	ordinal := GetOrdinal(pod)
	paddedOrdinal := GetPaddedOrdinal(nodeset, ordinal)
	pod.Name = GetPodName(nodeset, ordinal)
	pod.Namespace = nodeset.Namespace
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[slinkyv1beta1.LabelNodeSetPodName] = pod.Name
	pod.Labels[slinkyv1beta1.LabelNodeSetPodIndex] = paddedOrdinal
	pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname] = GetNodeName(pod)
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

// GetParentName gets the name of pod's parent NodeSet. If pod has not parent, the empty string is returned.
func GetParentName(pod *corev1.Pod) string {
	parent, _ := GetParentNameAndOrdinal(pod)
	return parent
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

// GetPodName gets the name of nodeset's child Pod with an ordinal index of ordinal
func GetPodName(nodeset *slinkyv1beta1.NodeSet, ordinal int) string {
	paddedOrdinal := GetPaddedOrdinal(nodeset, ordinal)
	return fmt.Sprintf("%s-%s", nodeset.Name, paddedOrdinal)
}

// GetNodeName returns the Slurm node name
func GetNodeName(pod *corev1.Pod) string {
	if pod.Spec.HostNetwork {
		return pod.Spec.NodeName
	}
	if pod.Spec.Hostname != "" {
		return pod.Spec.Hostname
	}
	return pod.Name
}

// IsIdentityMatch returns true if pod has a valid identity and network identity for a member of nodeset.
func IsIdentityMatch(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) bool {
	parent, ordinal := GetParentNameAndOrdinal(pod)
	return ordinal >= 0 &&
		nodeset.Name == parent &&
		pod.Name == GetPodName(nodeset, ordinal) &&
		pod.Namespace == nodeset.Namespace &&
		pod.Labels[slinkyv1beta1.LabelNodeSetPodName] == pod.Name
}

// IsStorageMatch returns true if pod's Volumes cover the nodeset of PersistentVolumeClaims
func IsStorageMatch(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) bool {
	ordinal := GetOrdinal(pod)
	paddedOrdinal := GetPaddedOrdinal(nodeset, ordinal)
	if ordinal < 0 {
		return false
	}
	volumes := make(map[string]corev1.Volume, len(pod.Spec.Volumes))
	for _, volume := range pod.Spec.Volumes {
		volumes[volume.Name] = volume
	}
	for _, claim := range nodeset.Spec.VolumeClaimTemplates {
		volume, found := volumes[claim.Name]
		if !found ||
			volume.PersistentVolumeClaim == nil ||
			volume.PersistentVolumeClaim.ClaimName !=
				GetPersistentVolumeClaimName(nodeset, &claim, paddedOrdinal) {
			return false
		}
	}
	return true
}

// GetPersistentVolumeClaims gets a map of PersistentVolumeClaims to their template names, as defined in nodeset. The
// returned PersistentVolumeClaims are each constructed with a the name specific to the Pod. This name is determined
// by GetPersistentVolumeClaimName.
func GetPersistentVolumeClaims(nodeset *slinkyv1beta1.NodeSet, pod *corev1.Pod) map[string]corev1.PersistentVolumeClaim {
	ordinal := GetOrdinal(pod)
	paddedOrdinal := GetPaddedOrdinal(nodeset, ordinal)
	templates := nodeset.Spec.VolumeClaimTemplates
	selectorLabels := labels.NewBuilder().WithWorkerSelectorLabels(nodeset).Build()
	claims := make(map[string]corev1.PersistentVolumeClaim, len(templates))
	for i := range templates {
		claim := templates[i].DeepCopy()
		claim.Name = GetPersistentVolumeClaimName(nodeset, claim, paddedOrdinal)
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

// GetPersistentVolumeClaimName gets the name of PersistentVolumeClaim for a Pod with an ordinal index of ordinal. claim
// must be a PersistentVolumeClaim from nodeset's VolumeClaims template.
func GetPersistentVolumeClaimName(nodeset *slinkyv1beta1.NodeSet, claim *corev1.PersistentVolumeClaim, paddedOrdinal string) string {
	// NOTE: This name format is used by the heuristics for zone spreading in ChooseZoneForVolume
	return fmt.Sprintf("%s-%s-%s", claim.Name, nodeset.Name, paddedOrdinal)
}

// SetOwnerReferences modifies the object with all NodeSets as non-controller owners.
func SetOwnerReferences(r client.Client, ctx context.Context, object metav1.Object, clusterName string) error {
	nodesetList := &slinkyv1beta1.NodeSetList{}
	if err := r.List(ctx, nodesetList); err != nil {
		return err
	}

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
