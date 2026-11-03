// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package syncsteps

import (
	"context"
	"fmt"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	eventutils "github.com/SlinkyProject/slurm-operator/internal/events"
)

type Step[T client.Object] struct {
	Name        string
	SyncFn      func(context.Context, T) error
	StopOnError bool
}

func Sync[T client.Object](
	ctx context.Context,
	recorder events.EventRecorder,
	obj T,
	steps []Step[T],
) error {
	mainLogger := log.FromContext(ctx)
	var errs []error
	for _, s := range steps {
		logger := mainLogger.WithValues("step", s.Name, "object", klog.KObj(obj))
		logger.V(2).Info("Starting sync step")
		if err := s.SyncFn(ctx, obj); err != nil {
			msg := fmt.Sprintf("Failed %q step: %v", s.Name, err)
			eventutils.Warning(recorder, obj, nil, eventutils.ReasonSyncFailed, eventutils.ActionSync, msg)
			errs = append(errs, fmt.Errorf("failed %q step: %w", s.Name, err))
			if s.StopOnError {
				logger.Error(err, "Failed sync step. Stopping sync for object...")
				break
			} else {
				logger.Error(err, "Failed sync step. Continuing sync for object...")
			}
		}
		logger.V(2).Info("Finished sync step")
	}
	return utilerrors.NewAggregate(errs)
}
