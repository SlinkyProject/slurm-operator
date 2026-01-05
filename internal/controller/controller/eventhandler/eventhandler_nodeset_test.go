// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"
	"testing"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

func Test_NodeSetEventHandler_Create(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	slurmKeyRef := testutils.NewSlurmKeyRef("foo")
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef("foo")
	controller := testutils.NewController("slurm1", slurmKeyRef, jwtHs256KeyRef, nil)
	nodeset := testutils.NewNodeset("slurmA", controller, 2)
	type fields struct {
		Reader client.Reader
	}
	type args struct {
		ctx context.Context
		evt event.CreateEvent
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "smoke",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodeset,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: nodeset,
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewNodeSetEventHandler(tt.fields.Reader)
			h.Create(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("NodeSetEventHandler.Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_NodeSetEventHandler_Delete(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	slurmKeyRef := testutils.NewSlurmKeyRef("foo")
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef("foo")
	controller := testutils.NewController("slurm1", slurmKeyRef, jwtHs256KeyRef, nil)
	nodeset := testutils.NewNodeset("slurmA", controller, 2)
	type fields struct {
		Reader client.Reader
	}
	type args struct {
		ctx context.Context
		evt event.DeleteEvent
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "smoke",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodeset,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: nodeset,
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewNodeSetEventHandler(tt.fields.Reader)
			h.Delete(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("NodeSetEventHandler.Delete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_NodeSetEventHandler_Generic(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	type fields struct {
		Reader client.Reader
	}
	type args struct {
		ctx context.Context
		evt event.GenericEvent
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "Empty",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.GenericEvent{},
				q:   newQueue(),
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewNodeSetEventHandler(tt.fields.Reader)
			h.Generic(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("NodeSetEventHandler.Generic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_NodeSetEventHandler_Update(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	slurmKeyRef := testutils.NewSlurmKeyRef("foo")
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef("foo")
	controller := testutils.NewController("slurm1", slurmKeyRef, jwtHs256KeyRef, nil)
	nodeset := testutils.NewNodeset("slurmA", controller, 2)
	type fields struct {
		Reader client.Reader
	}
	type args struct {
		ctx context.Context
		evt event.UpdateEvent
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "smoke",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodeset,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectNew: nodeset,
					ObjectOld: nodeset,
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewNodeSetEventHandler(tt.fields.Reader)
			h.Update(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("NodeSetEventHandler.Update() = %v, want %v", got, tt.want)
			}
		})
	}
}
