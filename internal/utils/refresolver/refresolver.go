// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package refresolver

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/utils/objectutils"
)

type RefResolver struct {
	reader client.Reader
}

func New(reader client.Reader) *RefResolver {
	return &RefResolver{
		reader: reader,
	}
}

func (r *RefResolver) GetController(ctx context.Context, ref slinkyv1beta1.ObjectReference) (*slinkyv1beta1.Controller, error) {
	obj := &slinkyv1beta1.Controller{}
	key := ref.NamespacedName()
	if err := r.reader.Get(ctx, key, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (r *RefResolver) GetAccounting(ctx context.Context, ref slinkyv1beta1.ObjectReference) (*slinkyv1beta1.Accounting, error) {
	obj := &slinkyv1beta1.Accounting{}
	key := ref.NamespacedName()
	if err := r.reader.Get(ctx, key, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (r *RefResolver) GetNodeSetsForController(ctx context.Context, controller *slinkyv1beta1.Controller) (*slinkyv1beta1.NodeSetList, error) {
	list := &slinkyv1beta1.NodeSetList{}
	if err := r.reader.List(ctx, list); err != nil {
		return nil, err
	}

	out := &slinkyv1beta1.NodeSetList{}
	for _, item := range list.Items {
		if item.Spec.ControllerRef.IsMatch(objectutils.NamespacedName(controller)) {
			out.Items = append(out.Items, item)
		}
	}

	return out, nil
}

func (r *RefResolver) GetLoginSetsForController(ctx context.Context, controller *slinkyv1beta1.Controller) (*slinkyv1beta1.LoginSetList, error) {
	list := &slinkyv1beta1.LoginSetList{}
	if err := r.reader.List(ctx, list); err != nil {
		return nil, err
	}

	out := &slinkyv1beta1.LoginSetList{}
	for _, item := range list.Items {
		if item.Spec.ControllerRef.IsMatch(objectutils.NamespacedName(controller)) {
			out.Items = append(out.Items, item)
		}
	}

	return out, nil
}

func (r *RefResolver) GetRestapisForController(ctx context.Context, controller *slinkyv1beta1.Controller) (*slinkyv1beta1.RestApiList, error) {
	list := &slinkyv1beta1.RestApiList{}
	if err := r.reader.List(ctx, list); err != nil {
		return nil, err
	}

	out := &slinkyv1beta1.RestApiList{}
	for _, item := range list.Items {
		if item.Spec.ControllerRef.IsMatch(objectutils.NamespacedName(controller)) {
			out.Items = append(out.Items, item)
		}
	}

	return out, nil
}

func (r *RefResolver) GetControllersForAccounting(ctx context.Context, accounting *slinkyv1beta1.Accounting) (*slinkyv1beta1.ControllerList, error) {
	list := &slinkyv1beta1.ControllerList{}
	if err := r.reader.List(ctx, list); err != nil {
		return nil, err
	}

	out := &slinkyv1beta1.ControllerList{}
	for _, item := range list.Items {
		if item.Spec.AccountingRef.IsMatch(objectutils.NamespacedName(accounting)) {
			out.Items = append(out.Items, item)
		}
	}

	return out, nil
}

func (r *RefResolver) GetSecretKeyRef(ctx context.Context, selector *corev1.SecretKeySelector, namespace string) ([]byte, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Name:      selector.Name,
		Namespace: namespace,
	}
	if err := r.reader.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	data, ok := secret.Data[selector.Key]
	if !ok {
		return nil, fmt.Errorf("secret key '%s' not found", selector.Key)
	}

	return data, nil
}
