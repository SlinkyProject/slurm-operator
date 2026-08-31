// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateObjectIfNotExists creates the object identified by key when it is
// absent, using out as the read target for the existence check.
//
// Unlike [SyncObject], build only runs on the create path. This suits objects
// that must not be regenerated once they exist, such as immutable Secrets
// holding generated keys, where building on every reconcile is wasted work.
func CreateObjectIfNotExists[T client.Object](
	c client.Client,
	ctx context.Context,
	eventRecorder events.EventRecorder,
	eventObj client.Object,
	key client.ObjectKey,
	out T,
	build func() (T, error),
) error {
	logger := log.FromContext(ctx)

	if err := c.Get(ctx, key, out); err == nil {
		logger.V(2).Info(fmt.Sprintf("%s already exists. Skipping create...", key))
		return nil
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("error getting %s: %w", key, err)
	}

	newObj, err := build()
	if err != nil {
		return err
	}

	if err := c.Create(ctx, newObj); err != nil {
		if apierrors.IsAlreadyExists(err) {
			logger.V(2).Info(fmt.Sprintf("%s already exists. Skipping create...", key))
			return nil
		}
		if eventRecorder != nil {
			eventRecorder.Eventf(eventObj, out, corev1.EventTypeWarning, ReasonCreateFailed, "Create", "Error creating %T: %s: %v", newObj, key, err)
		}
		return fmt.Errorf("error creating %s: %w", key, err)
	}

	if eventRecorder != nil {
		eventRecorder.Eventf(eventObj, out, corev1.EventTypeNormal, ReasonCreateSucceeded, "Create", "Created %T: %s", newObj, key)
	}

	return nil
}
