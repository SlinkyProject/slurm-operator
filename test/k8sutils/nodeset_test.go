// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package k8sutils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestMutateNodeSetRetriesConflict(t *testing.T) {
	for _, tc := range []struct {
		name             string
		replicas         int32
		concurrentUpdate bool
	}{
		{name: "scale up", replicas: 2},
		{name: "scale down", replicas: 1},
		{name: "scale up with conflict", replicas: 2, concurrentUpdate: true},
		{name: "scale down with conflict", replicas: 1, concurrentUpdate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, slinkyv1beta1.AddToScheme(scheme))
			key := crclient.ObjectKey{Namespace: "slurm-e2e", Name: "slurm-worker-slinky"}
			nodeset := &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
				Spec:       slinkyv1beta1.NodeSetSpec{Replicas: ptr.To[int32](3)},
			}

			updates := 0
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodeset).
				WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, client crclient.WithWatch, obj crclient.Object, opts ...crclient.UpdateOption) error {
						updates++
						if tc.concurrentUpdate && updates == 1 {
							latest := &slinkyv1beta1.NodeSet{}
							require.NoError(t, client.Get(ctx, key, latest))
							latest.Labels = map[string]string{"concurrent-update": "preserved"}
							require.NoError(t, client.Update(ctx, latest))
						}
						err := client.Update(ctx, obj, opts...)
						if tc.concurrentUpdate && updates == 1 {
							require.True(t, apierrors.IsConflict(err), "expected the stale update to conflict: %v", err)
						}
						return err
					},
				}).Build()

			MutateNodeSet(ctx, t, client, key, func(nodeset *slinkyv1beta1.NodeSet) {
				nodeset.Spec.Replicas = ptr.To(tc.replicas)
			})

			actual := &slinkyv1beta1.NodeSet{}
			require.NoError(t, client.Get(ctx, key, actual))
			require.NotNil(t, actual.Spec.Replicas)
			require.Equal(t, tc.replicas, *actual.Spec.Replicas)
			if tc.concurrentUpdate {
				require.Equal(t, 2, updates)
				require.Equal(t, "preserved", actual.Labels["concurrent-update"])
			} else {
				require.Equal(t, 1, updates)
			}
		})
	}
}
