// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package batch

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlowStartBatch(t *testing.T) {
	fakeErr := fmt.Errorf("fake error")
	callCnt := 0
	callLimit := 0
	var lock sync.Mutex
	fn := func(idx int) error {
		lock.Lock()
		defer lock.Unlock()
		callCnt++
		if callCnt > callLimit {
			return fakeErr
		}
		return nil
	}

	tests := []struct {
		name              string
		count             int
		callLimit         int
		fn                func(int) error
		expectedSuccesses int
		expectedErr       error
		expectedCallCnt   int
	}{
		{
			name:              "callLimit = 0 (all fail)",
			count:             10,
			callLimit:         0,
			fn:                fn,
			expectedSuccesses: 0,
			expectedErr:       fakeErr,
			expectedCallCnt:   1, // 1(first batch): function will be called at least once
		},
		{
			name:              "callLimit = count (all succeed)",
			count:             10,
			callLimit:         10,
			fn:                fn,
			expectedSuccesses: 10,
			expectedErr:       nil,
			expectedCallCnt:   10, // 1(first batch) + 2(2nd batch) + 4(3rd batch) + 3(4th batch) = 10
		},
		{
			name:              "callLimit < count (some succeed)",
			count:             10,
			callLimit:         5,
			fn:                fn,
			expectedSuccesses: 5,
			expectedErr:       fakeErr,
			expectedCallCnt:   7, // 1(first batch) + 2(2nd batch) + 4(3rd batch) = 7
		},
	}

	for _, test := range tests {
		callCnt = 0
		callLimit = test.callLimit
		successes, err := SlowStartBatch(test.count, SlowStartInitialBatchSize, test.fn)

		require.Equal(t, test.expectedSuccesses, successes, "%s: unexpected processed batch size", test.name)
		require.ErrorIs(t, err, test.expectedErr, "%s: unexpected error", test.name)
		// verify that slowStartBatch stops trying more calls after a batch fails
		if callCnt != test.expectedCallCnt {
			t.Errorf("%s: slowStartBatch() still tries calls after a batch fails, expected %d calls, got %d", test.name, test.expectedCallCnt, callCnt)
		}
	}
}

func TestConcurrentStartBatch(t *testing.T) {
	errA := fmt.Errorf("error a")

	tests := []struct {
		name              string
		count             int
		limit             int
		fn                func(int) error
		expectedSuccesses int
		expectedErr       error
		expectedCallCnt   int
	}{
		{
			name:              "count = 0",
			count:             0,
			limit:             2,
			fn:                func(int) error { return nil },
			expectedSuccesses: 0,
			expectedCallCnt:   0,
		},
		{
			name:              "all succeed",
			count:             10,
			limit:             2,
			fn:                func(int) error { return nil },
			expectedSuccesses: 10,
			expectedCallCnt:   10,
		},
		{
			name:              "immediate error",
			count:             5,
			limit:             1,
			fn:                func(int) error { return errA },
			expectedSuccesses: 0,
			expectedCallCnt:   1,
			expectedErr:       errA,
		},
		{
			name:  "error after some succeed",
			count: 5,
			limit: 1,
			fn: func(idx int) error {
				if idx >= 2 {
					return errA
				}
				return nil
			},
			expectedSuccesses: 2,
			expectedErr:       errA,
			expectedCallCnt:   3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.TODO()
			var seen sync.Map
			fn := func(idx int) error {
				n, _ := seen.LoadOrStore(idx, new(atomic.Int64))
				n.(*atomic.Int64).Add(1)
				return test.fn(idx)
			}

			successes, err := ConcurrentStartBatch(ctx, test.count, test.limit, fn)

			require.Equal(t, test.expectedSuccesses, successes, "%s: unexpected processed batch size", test.name)
			require.ErrorIs(t, err, test.expectedErr, "%s: unexpected error", test.name)

			calls := 0
			seen.Range(func(_, value any) bool {
				calls++
				require.Equal(t, int64(1), value.(*atomic.Int64).Load())
				return true
			})
			require.Equal(t, test.expectedCallCnt, calls)
			for i := range test.expectedCallCnt {
				_, ok := seen.Load(i)
				require.True(t, ok, "index %d", i)
			}
		})
	}

	t.Run("in-flight calls finish, later calls are not started", func(t *testing.T) {
		const count = 20
		const limit = 4
		var calls atomic.Int32
		successes, err := ConcurrentStartBatch(t.Context(), count, limit, func(int) error {
			calls.Add(1)
			return errA
		})
		require.Equal(t, 0, successes)
		require.ErrorIs(t, err, errA)
		require.Positive(t, calls.Load())
		require.LessOrEqual(t, calls.Load(), int32(limit))
	})
}
