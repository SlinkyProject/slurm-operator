// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type clientIndexer struct {
	obj   client.Object
	field string
	fn    client.IndexerFunc
}

var indexers = []clientIndexer{
	{
		obj:   &corev1.Pod{},
		field: "spec.nodeName",
		fn: func(o client.Object) []string {
			obj, ok := o.(runtime.Object)
			if !ok {
				return []string{}
			}
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				return []string{}
			}
			return []string{pod.Spec.NodeName}
		},
	},
}

func addIndexers(mgr ctrl.Manager) error {
	for _, indexer := range indexers {
		err := mgr.GetFieldIndexer().IndexField(context.Background(), indexer.obj, indexer.field, indexer.fn)
		if err != nil {
			return err
		}
	}
	return nil
}

// newFakeClientBuilderWithIndexes returns a client builder with the equivalent of addIndexers applied.
func newFakeClientBuilderWithIndexes(initObjs ...runtime.Object) *fake.ClientBuilder {
	cb := fake.NewClientBuilder().WithRuntimeObjects(initObjs...)
	for _, indexer := range indexers {
		obj := indexer.obj.(runtime.Object)
		cb = cb.WithIndex(obj, indexer.field, indexer.fn)
	}
	return cb
}
