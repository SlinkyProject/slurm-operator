// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/indexes"
	"github.com/SlinkyProject/slurm-operator/internal/utils/confighash"
)

// applyConfigHashes, for NodeSets opted in via AnnotationReloadOnChange, writes
// a per-resource checksum annotation for every mounted Secret/ConfigMap onto the
// in-memory NodeSet pod template metadata. Because getPatch folds spec.template
// (including its metadata) into the ControllerRevision, a changed checksum
// produces a new revision and drives a rolling update. The annotations also
// propagate onto the built worker pods for observability.
//
// Change detection is based on status.ConfigHashes, which records the checksums
// observed on the previous reconcile keyed by "<kind>/<name>": when a freshly
// computed checksum differs from the recorded one, a Normal ConfigHashChanged
// event is emitted against the NodeSet. The recorded map itself is refreshed in
// syncNodeSetStatus from the checksums stamped here.
//
// It must run before getNodeSetRevisions. Transient read errors are returned so
// the request requeues without mutating annotations (avoiding spurious
// revisions); missing resources resolve to confighash.MissingSentinel.
func (r *NodeSetReconciler) applyConfigHashes(ctx context.Context, nodeset *slinkyv1beta1.NodeSet) error {
	if nodeset.Annotations[slinkyv1beta1.AnnotationReloadOnChange] != "true" {
		return nil
	}

	secrets, configMaps := confighash.References(nodeset.Spec.Template.PodSpecWrapper.PodSpec)
	if len(secrets) == 0 && len(configMaps) == 0 {
		return nil
	}

	if nodeset.Spec.Template.Metadata.Annotations == nil {
		nodeset.Spec.Template.Metadata.Annotations = map[string]string{}
	}

	// status.ConfigHashes holds the checksums observed on the previous reconcile;
	// a difference means the resource content changed since then.
	previous := nodeset.Status.ConfigHashes

	emitIfChanged := func(kind, name, statusKey, hash string) {
		old, ok := previous[statusKey]
		if !ok || old == hash {
			return
		}
		r.eventRecorder.Eventf(nodeset, nil, corev1.EventTypeNormal, ConfigHashChangedReason, "ConfigChange",
			"mounted %s %q content changed; triggering rolling update", kind, name)
	}

	for _, name := range secrets {
		hash, err := confighash.SecretHash(ctx, r.Client, nodeset.Namespace, name)
		if err != nil {
			return err
		}
		nodeset.Spec.Template.Metadata.Annotations[confighash.AnnotationKey(confighash.SecretKindPrefix, name)] = hash
		emitIfChanged("Secret", name, indexes.SecretRefPrefix+name, hash)
	}

	for _, name := range configMaps {
		hash, err := confighash.ConfigMapHash(ctx, r.Client, nodeset.Namespace, name)
		if err != nil {
			return err
		}
		nodeset.Spec.Template.Metadata.Annotations[confighash.AnnotationKey(confighash.ConfigMapKindPrefix, name)] = hash
		emitIfChanged("ConfigMap", name, indexes.ConfigMapRefPrefix+name, hash)
	}

	return nil
}

// configHashesFromTemplate returns the per-resource config-hash checksums
// currently stamped on the NodeSet pod template, keyed by "<kind>/<name>". It
// returns nil when the NodeSet is not opted in or mounts nothing trackable, so
// that status.ConfigHashes mirrors exactly the set of tracked resources. It must
// be called after applyConfigHashes has stamped the annotations.
func configHashesFromTemplate(nodeset *slinkyv1beta1.NodeSet) map[string]string {
	if nodeset.Annotations[slinkyv1beta1.AnnotationReloadOnChange] != "true" {
		return nil
	}

	secrets, configMaps := confighash.References(nodeset.Spec.Template.PodSpecWrapper.PodSpec)
	if len(secrets) == 0 && len(configMaps) == 0 {
		return nil
	}

	annotations := nodeset.Spec.Template.Metadata.Annotations
	out := make(map[string]string, len(secrets)+len(configMaps))
	for _, name := range secrets {
		out[indexes.SecretRefPrefix+name] = annotations[confighash.AnnotationKey(confighash.SecretKindPrefix, name)]
	}
	for _, name := range configMaps {
		out[indexes.ConfigMapRefPrefix+name] = annotations[confighash.AnnotationKey(confighash.ConfigMapKindPrefix, name)]
	}
	return out
}
