// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"maps"
	"regexp"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8scontroller "k8s.io/kubernetes/pkg/controller"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/builder"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
)

// refResolver := refresolver.New(b.client)
// controller, err := refResolver.GetController(context.TODO(), nodeset.Spec.ControllerRef)
// if err != nil {
// 	return corev1.PodTemplateSpec{}, err
// }

// NewNodeSetPod returns a new Pod conforming to the nodeset's Spec with an identity generated from ordinal.
func NewNodeSetPod(
	nodeset *slinkyv1alpha1.NodeSet,
	controller *slinkyv1alpha1.Controller,
	ordinal int,
	revisionHash string,
) *corev1.Pod {
	controllerRef := metav1.NewControllerRef(nodeset, slinkyv1alpha1.NodeSetGVK)
	podTemplate := builder.New(nil).BuildComputePodTemplate(nodeset, controller)
	pod, _ := k8scontroller.GetPodFromTemplate(&podTemplate, nodeset, controllerRef)
	pod.Name = GetPodName(nodeset, ordinal)
	initIdentity(nodeset, pod)
	UpdateStorage(nodeset, pod)

	if revisionHash != "" {
		historycontrol.SetRevision(pod.Labels, revisionHash)
	}

	// WARNING: Do not use the spec.NodeName otherwise the Pod scheduler will
	// be avoided and priorityClass will not be honored.
	pod.Spec.NodeName = ""

	return pod
}

func initIdentity(nodeset *slinkyv1alpha1.NodeSet, pod *corev1.Pod) {
	UpdateIdentity(nodeset, pod)
	// Set these immutable fields only on initial Pod creation, not updates.
	if pod.Spec.Hostname != "" {
		pod.Spec.Hostname = fmt.Sprintf("%s%d", pod.Spec.Hostname, GetOrdinal(pod))
	} else {
		pod.Spec.Hostname = pod.Name
	}
}

// UpdateIdentity updates pod's name, hostname, and subdomain, and StatefulSetPodNameLabel to conform to nodeset's name
// and headless service.
func UpdateIdentity(nodeset *slinkyv1alpha1.NodeSet, pod *corev1.Pod) {
	ordinal := GetOrdinal(pod)
	pod.Name = GetPodName(nodeset, ordinal)
	pod.Namespace = nodeset.Namespace
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels[slinkyv1alpha1.LabelNodeSetPodName] = pod.Name
	pod.Labels[slinkyv1alpha1.LabelNodeSetPodIndex] = strconv.Itoa(ordinal)
}

// UpdateStorage updates pod's Volumes to conform with the PersistentVolumeClaim of nodeset's templates. If pod has
// conflicting local Volumes these are replaced with Volumes that conform to the nodeset's templates.
func UpdateStorage(nodeset *slinkyv1alpha1.NodeSet, pod *corev1.Pod) {
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

// IsPodFromNodeSet returns if the name schema matches
func IsPodFromNodeSet(nodeset *slinkyv1alpha1.NodeSet, pod *corev1.Pod) bool {
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

// GetPodName gets the name of nodeset's child Pod with an ordinal index of ordinal
func GetPodName(nodeset *slinkyv1alpha1.NodeSet, ordinal int) string {
	return fmt.Sprintf("%s-%d", nodeset.Name, ordinal)
}

// GetPodName gets the name of nodeset's child Pod with an ordinal index of ordinal
func GetNodeName(pod *corev1.Pod) string {
	if pod.Spec.Hostname != "" {
		return pod.Spec.Hostname
	}
	return pod.Name
}

// IsIdentityMatch returns true if pod has a valid identity and network identity for a member of nodeset.
func IsIdentityMatch(nodeset *slinkyv1alpha1.NodeSet, pod *corev1.Pod) bool {
	parent, ordinal := GetParentNameAndOrdinal(pod)
	return ordinal >= 0 &&
		nodeset.Name == parent &&
		pod.Name == GetPodName(nodeset, ordinal) &&
		pod.Namespace == nodeset.Namespace &&
		pod.Labels[slinkyv1alpha1.LabelNodeSetPodName] == pod.Name
}

// IsStorageMatch returns true if pod's Volumes cover the nodeset of PersistentVolumeClaims
func IsStorageMatch(nodeset *slinkyv1alpha1.NodeSet, pod *corev1.Pod) bool {
	ordinal := GetOrdinal(pod)
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
				GetPersistentVolumeClaimName(nodeset, &claim, ordinal) {
			return false
		}
	}
	return true
}

// GetPersistentVolumeClaims gets a map of PersistentVolumeClaims to their template names, as defined in nodeset. The
// returned PersistentVolumeClaims are each constructed with a the name specific to the Pod. This name is determined
// by GetPersistentVolumeClaimName.
func GetPersistentVolumeClaims(nodeset *slinkyv1alpha1.NodeSet, pod *corev1.Pod) map[string]corev1.PersistentVolumeClaim {
	ordinal := GetOrdinal(pod)
	templates := nodeset.Spec.VolumeClaimTemplates
	selectorLabels := labels.NewBuilder().WithComputeSelectorLabels(nodeset).Build()
	claims := make(map[string]corev1.PersistentVolumeClaim, len(templates))
	for i := range templates {
		claim := templates[i].DeepCopy()
		claim.Name = GetPersistentVolumeClaimName(nodeset, claim, ordinal)
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
func GetPersistentVolumeClaimName(nodeset *slinkyv1alpha1.NodeSet, claim *corev1.PersistentVolumeClaim, ordinal int) string {
	// NOTE: This name format is used by the heuristics for zone spreading in ChooseZoneForVolume
	return fmt.Sprintf("%s-%s-%d", claim.Name, nodeset.Name, ordinal)
}
