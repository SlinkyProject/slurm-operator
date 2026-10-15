// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package refresolver

import (
	"context"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

const (
	ControllerRefField = "spec.controllerRef.name"
	AccountingRefField = "spec.accountingRef.name"
)

// indexed tracks the caches that already have these indexes.
var indexed sync.Map

// SetupIndexes registers reference lookups on the manager cache.
// Safe to call from every controller; a repeat on the same cache is a no-op.
func SetupIndexes(ctx context.Context, indexer client.FieldIndexer) error {
	if _, ok := indexed.Load(indexer); ok {
		return nil
	}
	specs := []struct {
		obj   client.Object
		field string
	}{
		{&slinkyv1beta1.NodeSet{}, ControllerRefField},
		{&slinkyv1beta1.LoginSet{}, ControllerRefField},
		{&slinkyv1beta1.RestApi{}, ControllerRefField},
		{&slinkyv1beta1.Controller{}, AccountingRefField},
	}
	for _, spec := range specs {
		fn := indexControllerRef
		if spec.field == AccountingRefField {
			fn = indexAccountingRef
		}
		if err := indexer.IndexField(ctx, spec.obj, spec.field, fn); err != nil {
			return err
		}
	}
	indexed.Store(indexer, struct{}{})
	return nil
}

// WithIndexes adds the same fields to a fake client. Call WithScheme first when
// the builder scheme is not the global one.
func WithIndexes(b *fake.ClientBuilder) *fake.ClientBuilder {
	return b.
		WithIndex(&slinkyv1beta1.NodeSet{}, ControllerRefField, indexControllerRef).
		WithIndex(&slinkyv1beta1.LoginSet{}, ControllerRefField, indexControllerRef).
		WithIndex(&slinkyv1beta1.RestApi{}, ControllerRefField, indexControllerRef).
		WithIndex(&slinkyv1beta1.Controller{}, AccountingRefField, indexAccountingRef)
}

func indexControllerRef(obj client.Object) []string {
	var name string
	switch o := obj.(type) {
	case *slinkyv1beta1.NodeSet:
		name = o.Spec.ControllerRef.Name
	case *slinkyv1beta1.LoginSet:
		name = o.Spec.ControllerRef.Name
	case *slinkyv1beta1.RestApi:
		name = o.Spec.ControllerRef.Name
	default:
		return nil
	}
	if name == "" {
		return nil
	}
	return []string{name}
}

func indexAccountingRef(obj client.Object) []string {
	controller, ok := obj.(*slinkyv1beta1.Controller)
	if !ok || controller.Spec.AccountingRef == nil || controller.Spec.AccountingRef.Name == "" {
		return nil
	}
	return []string{controller.Spec.AccountingRef.Name}
}
