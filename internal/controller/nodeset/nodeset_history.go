// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"encoding/json"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/controller/history"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
)

// truncateHistory truncates any non-live ControllerRevisions in revisions from nodeset's history. The UpdateRevision and
// CurrentRevision in nodeset's Status are considered to be live. Any revisions associated with the Pods in pods are also
// considered to be live. Non-live revisions are deleted, starting with the revision with the lowest Revision, until
// only RevisionHistoryLimit revisions remain. If the returned error is nil the operation was successful. This method
// expects that revisions is sorted when supplied.
func (r *NodeSetReconciler) truncateHistory(
	ctx context.Context,
	nodeset *slinkyv1beta1.NodeSet,
	revisions []*appsv1.ControllerRevision,
	current, update *appsv1.ControllerRevision,
) error {
	pods, err := r.getNodeSetPods(ctx, nodeset)
	if err != nil {
		return err
	}

	history := make([]*appsv1.ControllerRevision, 0, len(revisions))
	// mark all live revisions
	live := map[string]bool{}
	if current != nil {
		live[current.Name] = true
	}
	if update != nil {
		live[update.Name] = true
	}
	for i := range pods {
		live[historycontrol.GetRevision(pods[i].GetLabels())] = true
	}
	// collect live revisions and historic revisions
	for i := range revisions {
		if !live[revisions[i].Name] {
			history = append(history, revisions[i])
		}
	}
	historyLen := len(history)
	historyLimit := int(ptr.Deref(nodeset.Spec.RevisionHistoryLimit, 0))
	if historyLen <= historyLimit {
		return nil
	}
	// delete any non-live history to maintain the revision limit.
	history = history[:(historyLen - historyLimit)]
	for i := range history {
		if err := r.historyControl.DeleteControllerRevision(history[i]); err != nil {
			return err
		}
	}
	return nil
}

// getNodeSetRevisions returns the current and update ControllerRevisions for nodeset. It also
// returns a collision count that records the number of name collisions nodeset saw when creating
// new ControllerRevisions. This count is incremented on every name collision and is used in
// building the ControllerRevision names for name collision avoidance. This method may create
// a new revision, or modify the Revision of an existing revision if an update to nodeset is detected.
// This method expects that revisions is sorted when supplied.
func (r *NodeSetReconciler) getNodeSetRevisions(
	nodeset *slinkyv1beta1.NodeSet,
	revisions []*appsv1.ControllerRevision,
) (*appsv1.ControllerRevision, *appsv1.ControllerRevision, int32, error) {
	var currentRevision, updateRevision *appsv1.ControllerRevision

	revisionCount := len(revisions)
	history.SortControllerRevisions(revisions)

	// Use a local copy of nodeset.Status.CollisionCount to avoid modifying nodeset.Status directly.
	var collisionCount int32
	if nodeset.Status.CollisionCount != nil {
		collisionCount = *nodeset.Status.CollisionCount
	}

	// create a new revision from the current nodeset
	updateRevision, err := newRevision(nodeset, nextRevision(revisions), &collisionCount)
	if err != nil {
		return nil, nil, collisionCount, err
	}

	// find any equivalent revisions
	equalRevisions := history.FindEqualRevisions(revisions, updateRevision)
	equalCount := len(equalRevisions)

	if equalCount > 0 {
		if history.EqualRevision(revisions[revisionCount-1], equalRevisions[equalCount-1]) {
			// if the equivalent revision is immediately prior the update revision has not changed
			updateRevision = revisions[revisionCount-1]
		} else {
			// if the equivalent revision is not immediately prior we will roll back by incrementing the
			// Revision of the equivalent revision
			updateRevision, err = r.historyControl.UpdateControllerRevision(
				equalRevisions[equalCount-1],
				updateRevision.Revision)
			if err != nil {
				return nil, nil, collisionCount, err
			}
		}
	} else {
		// if there is no equivalent revision we create a new one
		updateRevision, err = r.historyControl.CreateControllerRevision(nodeset, updateRevision, &collisionCount)
		if err != nil {
			return nil, nil, collisionCount, err
		}
	}

	// attempt to find the revision that corresponds to the current revision
	for i := range revisions {
		if revisions[i].Name == nodeset.Status.NodeSetHash {
			currentRevision = revisions[i]
			break
		}
	}

	// if the current revision is nil we initialize the history by setting it to the update revision
	if currentRevision == nil {
		currentRevision = updateRevision
	}

	return currentRevision, updateRevision, collisionCount, nil
}

// nextRevision finds the next valid revision number based on revisions. If the length of revisions
// is 0 this is 1. Otherwise, it is 1 greater than the largest revision's Revision. This method
// assumes that revisions has been sorted by Revision.
func nextRevision(revisions []*appsv1.ControllerRevision) int64 {
	count := len(revisions)
	if count <= 0 {
		return 1
	}
	return revisions[count-1].Revision + 1
}

// newRevision creates a new ControllerRevision containing a patch that reapplies the target state of set.
// The Revision of the returned ControllerRevision is set to revision. If the returned error is nil, the returned
// ControllerRevision is valid. NodeSet revisions are stored as patches that re-apply the current state of NodeSet
// to a new NodeSet using a strategic merge patch to replace the saved state of the new NodeSet.
func newRevision(nodeset *slinkyv1beta1.NodeSet, revision int64, collisionCount *int32) (*appsv1.ControllerRevision, error) {
	patch, err := getPatch(nodeset)
	if err != nil {
		return nil, err
	}
	cr, err := history.NewControllerRevision(
		nodeset,
		slinkyv1beta1.NodeSetGVK,
		nodeset.Spec.Template.PodMetadata.Labels,
		runtime.RawExtension{Raw: patch},
		revision,
		collisionCount)
	if err != nil {
		return nil, err
	}
	if cr.Annotations == nil {
		cr.Annotations = make(map[string]string)
	}
	maps.Copy(cr.Annotations, nodeset.Annotations)
	return cr, nil
}

// getPatch returns a strategic merge patch that can be applied to restore a NodeSet to a
// previous version. If the returned error is nil the patch is valid. The current state that we save is just the
// PodSpecTemplate. We can modify this later to encompass more state (or less) and remain compatible with previously
// recorded patches.
func getPatch(nodeset *slinkyv1beta1.NodeSet) ([]byte, error) {
	crBytes, err := json.Marshal(nodeset)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(crBytes, &raw); err != nil {
		return nil, err
	}
	objCopy := make(map[string]any)
	specCopy := make(map[string]any)

	// Create a patch of the NodeSet that replaces spec.template
	spec := raw["spec"].(map[string]any)
	template := spec["template"].(map[string]any)
	specCopy["template"] = template
	template["$patch"] = "replace"
	if slurmd, ok := spec["slurmd"].(map[string]any); ok {
		slurmd["$patch"] = "replace"
		specCopy["slurmd"] = slurmd
	}
	if extraConf, ok := spec["extraConf"].(string); ok {
		specCopy["extraConf"] = extraConf
	}
	if logfile, ok := spec["logfile"].(map[string]any); ok {
		logfile["$patch"] = "replace"
		specCopy["logfile"] = logfile
	}
	objCopy["spec"] = specCopy
	patch, err := json.Marshal(objCopy)
	return patch, err
}
