// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package ratelimiter

import (
	"time"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
)

// Build creates a rate limiter for controllers.
// NOTE: we don't use workqueue.DefaultTypedControllerRateLimiter because it retries very
// aggressively, starting at 5ms.
func Build[T comparable]() workqueue.TypedRateLimiter[T] {
	// exponential backoff rate limiter
	//  - this handles per-item rate limiting for failures
	//  - it uses an exponential backoff strategy where: delay = baseDelay * 2^failures
	//  - base/cap follow Kubeflow's controller rate limiter convention (1s base, 7m cap)
	failureBaseDelay := 1 * time.Second
	failureMaxDelay := 7 * time.Minute
	failureRateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[T](failureBaseDelay, failureMaxDelay)

	// overall rate limiter
	//  - this handles overall rate limiting, ignoring individual items and only considering the
	//    overall rate
	//  - it implements a "token bucket" of size totalMaxBurst that is initially full, and which
	//    is refilled at rate totalEventsPerSecond tokens per second
	totalEventsPerSecond := 10
	totalMaxBurst := 100
	totalRateLimiter := &workqueue.TypedBucketRateLimiter[T]{
		Limiter: rate.NewLimiter(rate.Limit(totalEventsPerSecond), totalMaxBurst),
	}

	// return the worst-case (longest) of the rate limiters for a given item
	return workqueue.NewTypedMaxOfRateLimiter(failureRateLimiter, totalRateLimiter)
}
