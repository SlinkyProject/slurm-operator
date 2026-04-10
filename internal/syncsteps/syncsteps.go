// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package syncsteps

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const syncAction = "Sync"
const failedReason = "SyncFailed"

type Step[T client.Object] struct {
	Name   string
	SyncFn func(context.Context, T) error
}

func Sync[T client.Object](
	ctx context.Context,
	recorder events.EventRecorder,
	obj T,
	steps []Step[T],
) error {
	for _, s := range steps {
		if err := s.SyncFn(ctx, obj); err != nil {
			msg := fmt.Sprintf("Failed %q step: %v", s.Name, err)
			if recorder != nil {
				recorder.Eventf(obj, nil, corev1.EventTypeWarning, failedReason, syncAction, msg)
			}
			return fmt.Errorf("failed %q step: %w", s.Name, err)
		}
	}
	return nil
}
