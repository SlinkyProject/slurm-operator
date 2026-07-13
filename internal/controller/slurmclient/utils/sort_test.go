// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"sort"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/stretchr/testify/require"
)

func TestRestapisByCreationTimestamp(t *testing.T) {
	now := metav1.Now()
	then := metav1.Time{Time: now.Add(-time.Hour)}

	tests := []struct {
		name     string
		restapis []slinkyv1beta1.RestApi
		want     []string
	}{
		{
			name:     "empty",
			restapis: nil,
			want:     []string{},
		},
		{
			name: "single",
			restapis: []slinkyv1beta1.RestApi{
				newRestAPI("restapi", now),
			},
			want: []string{"restapi"},
		},
		{
			name: "sorts by creation timestamp",
			restapis: []slinkyv1beta1.RestApi{
				newRestAPI("newer", now),
				newRestAPI("older", then),
			},
			want: []string{"older", "newer"},
		},
		{
			name: "sorts equal timestamps by name",
			restapis: []slinkyv1beta1.RestApi{
				newRestAPI("bravo", now),
				newRestAPI("alpha", now),
				newRestAPI("charlie", now),
			},
			want: []string{"alpha", "bravo", "charlie"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sort.Sort(RestapisByCreationTimestamp(tt.restapis))

			got := make([]string, len(tt.restapis))
			for i := range tt.restapis {
				got[i] = tt.restapis[i].Name
			}

			require.Equal(t, tt.want, got)
		})
	}
}

func newRestAPI(name string, creationTimestamp metav1.Time) slinkyv1beta1.RestApi {
	return slinkyv1beta1.RestApi{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: creationTimestamp,
		},
	}
}
