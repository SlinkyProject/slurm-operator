// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

func Test_SecretEventHandler_Create(t *testing.T) {
	jwtKeyRef := testutils.NewJwtKeyRef("slurm")
	controller := testutils.NewController(
		"slurm",
		testutils.NewSlurmKeyRef("slurm"),
		jwtKeyRef,
		nil,
	)
	otherController := testutils.NewController(
		"other",
		testutils.NewSlurmKeyRef("other"),
		testutils.NewJwtKeyRef("other"),
		nil,
	)

	tests := []struct {
		name   string
		object client.Object
		want   []reconcile.Request
	}{
		{
			name:   "referenced JWT key",
			object: testutils.NewJwtKeySecret(jwtKeyRef),
			want: []reconcile.Request{{
				NamespacedName: client.ObjectKeyFromObject(controller),
			}},
		},
		{
			name:   "unreferenced JWT key",
			object: testutils.NewJwtKeySecret(testutils.NewJwtKeyRef("unreferenced")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSecretEventHandler(fake.NewFakeClient(controller, otherController))
			q := newQueue()
			defer q.ShutDown()

			h.Create(context.Background(), event.CreateEvent{Object: tt.object}, q)

			requireQueuedRequests(t, q, tt.want)
		})
	}
}

func Test_SecretEventHandler_Update(t *testing.T) {
	jwtKeyRef := testutils.NewJwtKeyRef("slurm")
	controller := testutils.NewController(
		"slurm",
		testutils.NewSlurmKeyRef("slurm"),
		jwtKeyRef,
		nil,
	)
	secret := testutils.NewJwtKeySecret(jwtKeyRef)
	h := NewSecretEventHandler(fake.NewFakeClient(controller))
	q := newQueue()
	defer q.ShutDown()

	h.Update(context.Background(), event.UpdateEvent{
		ObjectOld: secret.DeepCopy(),
		ObjectNew: secret,
	}, q)

	requireQueuedRequests(t, q, []reconcile.Request{{
		NamespacedName: client.ObjectKeyFromObject(controller),
	}})
}

func Test_SecretEventHandler_Delete(t *testing.T) {
	jwtKeyRef := testutils.NewJwtKeyRef("slurm")
	controller := testutils.NewController(
		"slurm",
		testutils.NewSlurmKeyRef("slurm"),
		jwtKeyRef,
		nil,
	)
	h := NewSecretEventHandler(fake.NewFakeClient(controller))
	q := newQueue()
	defer q.ShutDown()

	h.Delete(context.Background(), event.DeleteEvent{
		Object: testutils.NewJwtKeySecret(jwtKeyRef),
	}, q)

	requireQueuedRequests(t, q, []reconcile.Request{{
		NamespacedName: client.ObjectKeyFromObject(controller),
	}})
}

func Test_SecretEventHandler_Generic(t *testing.T) {
	jwtKeyRef := testutils.NewJwtKeyRef("slurm")
	controller := testutils.NewController(
		"slurm",
		testutils.NewSlurmKeyRef("slurm"),
		jwtKeyRef,
		nil,
	)
	h := NewSecretEventHandler(fake.NewFakeClient(controller))
	q := newQueue()
	defer q.ShutDown()

	h.Generic(context.Background(), event.GenericEvent{
		Object: testutils.NewJwtKeySecret(jwtKeyRef),
	}, q)

	requireQueuedRequests(t, q, nil)
}

func requireQueuedRequests(
	t *testing.T,
	q interface {
		Len() int
		Get() (reconcile.Request, bool)
		Done(reconcile.Request)
	},
	want []reconcile.Request,
) {
	t.Helper()
	require.Equal(t, len(want), q.Len())
	for _, wantRequest := range want {
		got, shutdown := q.Get()
		require.False(t, shutdown)
		require.Equal(t, wantRequest, got)
		q.Done(got)
	}
}
