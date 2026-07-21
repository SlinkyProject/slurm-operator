// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRefreshRequeueAfter(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want time.Duration
	}{
		{
			name: "negative is floored to the positive minimum",
			d:    -1 * time.Minute,
			want: tokenRefreshRequeueFloor,
		},
		{
			name: "zero is floored to the positive minimum",
			d:    0,
			want: tokenRefreshRequeueFloor,
		},
		{
			name: "positive below the floor is raised to the floor",
			d:    500 * time.Millisecond,
			want: tokenRefreshRequeueFloor,
		},
		{
			name: "exactly the floor is kept",
			d:    tokenRefreshRequeueFloor,
			want: tokenRefreshRequeueFloor,
		},
		{
			name: "positive above the floor is kept",
			d:    12 * time.Minute,
			want: 12 * time.Minute,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := refreshRequeueAfter(tt.d)
			require.Equal(t, tt.want, got)
			// The core invariant of #240: the refresh requeue is never <= 0,
			// which controller-runtime would treat as "do not requeue".
			require.Greater(t, got, time.Duration(0))
		})
	}
}
