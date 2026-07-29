// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package historycontrol

import (
	"bytes"
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/controller/history"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HistoryControlInterface interface {
	history.Interface
}

type realHistory struct {
	client.Client
}

var _ HistoryControlInterface = &realHistory{}

// NewHistoryControl returns an instance of Interface that uses client to communicate with the API Server and lister to list
// ControllerRevisions. This method should be used to create an Interface for all scenarios other than testing.
func NewHistoryControl(client client.Client) HistoryControlInterface {
	return &realHistory{
		Client: client,
	}
}

func (rh *realHistory) ListControllerRevisions(
	parent metav1.Object,
	_ schema.GroupVersionKind,
	selector labels.Selector,
) ([]*appsv1.ControllerRevision, error) {
	// List all revisions in the namespace that match the selector
	opts := &client.ListOptions{
		Namespace:     parent.GetNamespace(),
		LabelSelector: selector,
	}
	revisionList := &appsv1.ControllerRevisionList{}
	err := rh.List(context.TODO(), revisionList, opts)
	if err != nil {
		return nil, err
	}
	var owned []*appsv1.ControllerRevision
	for i := range revisionList.Items {
		ref := metav1.GetControllerOf(&revisionList.Items[i])
		if ref == nil || ref.UID == parent.GetUID() {
			owned = append(owned, &revisionList.Items[i])
		}
	}
	return owned, err
}

func (rh *realHistory) CreateControllerRevision(
	parent metav1.Object,
	revision *appsv1.ControllerRevision,
	collisionCount *int32,
) (*appsv1.ControllerRevision, error) {
	if collisionCount == nil {
		return nil, fmt.Errorf("collisionCount should not be nil")
	}
	namespace := parent.GetNamespace()

	// Clone the input
	clone := revision.DeepCopy()

	// Continue to attempt to create the revision updating the name with a new hash on each iteration
	for {
		hash := history.HashControllerRevision(revision, collisionCount)
		// Update the revisions name
		clone.Name = history.ControllerRevisionName(parent.GetName(), hash)

		created := clone.DeepCopy()
		created.Namespace = namespace
		err := rh.Create(context.TODO(), created)
		if apierrors.IsAlreadyExists(err) {
			namespacedName := types.NamespacedName{
				Namespace: namespace,
				Name:      clone.GetName(),
			}
			exists := &appsv1.ControllerRevision{}
			err := rh.Get(context.TODO(), namespacedName, exists)
			if err != nil {
				return nil, err
			}
			if bytes.Equal(exists.Data.Raw, clone.Data.Raw) {
				return exists, nil
			}
			*collisionCount++
			continue
		}
		return created, err
	}
}

func (rh *realHistory) UpdateControllerRevision(
	revision *appsv1.ControllerRevision,
	newRevision int64,
) (*appsv1.ControllerRevision, error) {
	clone := revision.DeepCopy()
	namespacedName := types.NamespacedName{
		Namespace: clone.GetNamespace(),
		Name:      clone.GetName(),
	}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if clone.Revision == newRevision {
			return nil
		}
		clone.Revision = newRevision
		updateErr := rh.Update(context.TODO(), clone)
		if updateErr == nil {
			return nil
		}
		got := &appsv1.ControllerRevision{}
		if err := rh.Get(context.TODO(), namespacedName, got); err == nil {
			clone = got
		}
		return updateErr
	})
	return clone, err
}

func (rh *realHistory) DeleteControllerRevision(revision *appsv1.ControllerRevision) error {
	return rh.Delete(context.TODO(), revision)
}

func (rh *realHistory) AdoptControllerRevision(
	parent metav1.Object,
	parentKind schema.GroupVersionKind,
	revision *appsv1.ControllerRevision,
) (*appsv1.ControllerRevision, error) {
	clone := revision.DeepCopy()
	namespacedName := types.NamespacedName{
		Namespace: clone.Namespace,
		Name:      clone.Name,
	}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Return an error if the parent does not own the revision
		if owner := metav1.GetControllerOf(clone); owner != nil {
			return fmt.Errorf("attempt to adopt revision owned by %v", owner)
		}

		clone.OwnerReferences = append(clone.OwnerReferences, metav1.OwnerReference{
			APIVersion:         parentKind.GroupVersion().String(),
			Kind:               parentKind.Kind,
			Name:               parent.GetName(),
			UID:                parent.GetUID(),
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		})

		updateErr := rh.Update(context.TODO(), clone)
		if updateErr == nil {
			return nil
		}

		got := &appsv1.ControllerRevision{}
		if err := rh.Get(context.TODO(), namespacedName, got); err == nil {
			clone = got
		}
		return updateErr
	})

	if err != nil {
		return nil, err
	}
	return clone, nil
}

func (rh *realHistory) ReleaseControllerRevision(
	parent metav1.Object,
	revision *appsv1.ControllerRevision,
) (*appsv1.ControllerRevision, error) {
	clone := revision.DeepCopy()
	namespacedName := types.NamespacedName{
		Namespace: clone.Namespace,
		Name:      clone.Name,
	}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// Return an error if the parent does not own the revision
		if owner := metav1.GetControllerOf(clone); owner != nil {
			return fmt.Errorf("attempt to adopt revision owned by %v", owner)
		}

		var newOwners []metav1.OwnerReference
		for _, o := range clone.OwnerReferences {
			if o.UID == parent.GetUID() {
				continue
			}
			newOwners = append(newOwners, o)
		}
		clone.OwnerReferences = newOwners
		updateErr := rh.Update(context.TODO(), clone)
		if updateErr == nil {
			return nil
		}

		got := &appsv1.ControllerRevision{}
		if err := rh.Get(context.TODO(), namespacedName, got); err == nil {
			clone = got
		}
		return updateErr
	})

	if err != nil {
		if apierrors.IsNotFound(err) {
			// We ignore deleted revisions.
			return nil, nil //nolint:nilnil
		}
		if apierrors.IsInvalid(err) {
			// We ignore cases where the parent no longer owns the revision or
			// where the revision has no owner.
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	return clone, nil
}
