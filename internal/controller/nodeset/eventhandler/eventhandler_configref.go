// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/indexes"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

// enqueueNodeSetsForConfigRef enqueues every opted-in NodeSet in namespace that
// references the resource identified by refKey ("secret/<name>" or
// "configmap/<name>"), using the NodeSetConfigRefsField index.
func enqueueNodeSetsForConfigRef(
	ctx context.Context,
	reader client.Reader,
	q workqueue.TypedRateLimitingInterface[reconcile.Request],
	namespace, refKey string,
) {
	logger := log.FromContext(ctx)

	list := &slinkyv1beta1.NodeSetList{}
	if err := reader.List(ctx, list,
		client.InNamespace(namespace),
		client.MatchingFields{indexes.NodeSetConfigRefsField: refKey}); err != nil {
		logger.Error(err, "failed to list NodeSets for config ref", "refKey", refKey)
		return
	}
	for i := range list.Items {
		objectutils.EnqueueRequest(q, &list.Items[i])
	}
}
