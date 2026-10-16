// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package batch

import (
	"context"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/SlinkyProject/slurm-operator/internal/utils/mathutils"
)

const (
	// When batching pod creates, SlowStartInitialBatchSize is the size of the
	// initial batch. The size of each successive batch is twice the size of
	// the previous batch.
	SlowStartInitialBatchSize = 1

	// When batching requests, ConcurrentLimitDefault is the maximum number of
	// concurrent requests allowed to be in-flight at any given time.
	ConcurrentLimitDefault = 32
)

func SlowStartBatchDefault(count int, fn func(i int) error) (int, error) {
	return SlowStartBatch(count, SlowStartInitialBatchSize, fn)
}

// SlowStartBatch tries to call the provided function a total of 'count' times,
// starting slow to check for errors, then speeding up if calls succeed.
//
// It groups the calls into batches, starting with a group of initialBatchSize.
// Within each batch, it may call the function multiple times concurrently with its index.
//
// If a whole batch succeeds, the next batch may get exponentially larger.
// If there are any failures in a batch, all remaining batches are skipped
// after waiting for the current batch to complete.
//
// It returns the number of successful calls to the function.
func SlowStartBatch(count, initialBatchSize int, fn func(i int) error) (int, error) {
	remaining := count
	successes := 0
	index := 0
	for batchSize := min(remaining, initialBatchSize); batchSize > 0; batchSize = min(2*batchSize, remaining) {
		errCh := make(chan error, batchSize)
		var wg sync.WaitGroup
		wg.Add(batchSize)
		for range batchSize {
			go func(idx int) {
				defer wg.Done()
				if err := fn(idx); err != nil {
					errCh <- err
				}
			}(index)
			index++
		}
		wg.Wait()
		curSuccesses := batchSize - len(errCh)
		successes += curSuccesses
		if len(errCh) > 0 {
			return successes, <-errCh
		}
		remaining -= batchSize
	}
	return successes, nil
}

func ConcurrentStartBatchDefault(ctx context.Context, count int, fn func(index int) error) (int, error) {
	limit := mathutils.Clamp(count, ConcurrentLimitDefault, count/ConcurrentLimitDefault)
	return ConcurrentStartBatch(ctx, count, limit, fn)
}

// ConcurrentStartBatch calls fn for each index in [0, count), with at most limit
// calls in flight.
//
// If there are any failures, all remaining batches are skipped after waiting
// for the in-flight batch to complete.
//
// It returns the number of successful calls to the function.
func ConcurrentStartBatch(ctx context.Context, count, limit int, fn func(index int) error) (int, error) {
	var successCount atomic.Int64

	if limit == 0 {
		limit = -1
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	for i := range count {
		if err := gCtx.Err(); err != nil {
			break
		}
		g.Go(func() error {
			if gCtx.Err() != nil {
				return nil //nolint:nilerr
			}
			if err := fn(i); err != nil {
				return err
			}
			successCount.Add(1)
			return nil
		})
	}

	err := g.Wait()
	if err == nil {
		err = ctx.Err()
	}
	return int(successCount.Load()), err
}
