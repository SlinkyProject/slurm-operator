// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package ratelimiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestBuild_ExponentialBackoff asserts the per-item failure delay starts at 1s (not the
// aggressive 5ms default) and doubles on each repeated failure, up to the 7 minute cap.
func TestBuild_ExponentialBackoff(t *testing.T) {
	limiter := Build[string]()

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 1 * time.Second},
		{attempt: 2, want: 2 * time.Second},
		{attempt: 3, want: 4 * time.Second},
		{attempt: 4, want: 8 * time.Second},
	}
	for _, tt := range tests {
		got := limiter.When("item")
		require.Equalf(t, tt.want, got, "attempt %d", tt.attempt)
	}
}

func TestBuild_MaxDelayCap(t *testing.T) {
	limiter := Build[string]()

	var last time.Duration
	for range 15 {
		last = limiter.When("item")
	}
	require.Equal(t, 7*time.Minute, last)
}

func TestBuild_PerItemIsolation(t *testing.T) {
	limiter := Build[string]()

	require.Equal(t, 1*time.Second, limiter.When("a"))
	require.Equal(t, 2*time.Second, limiter.When("a"))
	// a different item's backoff is tracked independently of "a"
	require.Equal(t, 1*time.Second, limiter.When("b"))
}
