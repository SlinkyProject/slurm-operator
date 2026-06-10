// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"sync"
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
		successes, err := SlowStartBatch(test.count, 1, test.fn)

		require.Equal(t, test.expectedSuccesses, successes, "%s: unexpected processed batch size", test.name)
		require.ErrorIs(t, err, test.expectedErr, "%s: unexpected error", test.name)
		// verify that slowStartBatch stops trying more calls after a batch fails
		require.Equal(t, test.expectedCallCnt, callCnt, "%s: slowStartBatch() still tries calls after a batch fails", test.name)
	}
}
