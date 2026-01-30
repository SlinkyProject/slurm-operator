// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"
	"testing"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/togethercomputer/slurm-operator/internal/utils/testutils"
)

func Test_ControllerEventHandler_Create(t *testing.T) {
	slurmKeyRef := testutils.NewSlurmKeyRef("foo")
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef("foo")
	controller := testutils.NewController("slurm1", slurmKeyRef, jwtHs256KeyRef, nil)
	nodesetA := testutils.NewNodeset("slurmA", controller, 2)
	nodesetB := testutils.NewNodeset("slurmB", controller, 2)
	controller2 := testutils.NewController("slurm2", slurmKeyRef, jwtHs256KeyRef, nil)
	nodeset2A := testutils.NewNodeset("slurm2A", controller2, 2)
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
			name: "0 Refs",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: controller,
				},
				q: newQueue(),
			},
			want: 0,
		},
		{
			name: "1 Refs, controller 1",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: controller,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "1 Refs, controller 2",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: controller,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "2 Refs",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					nodesetB,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: controller,
				},
				q: newQueue(),
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewControllerEventHandler(tt.fields.Reader)
			h.Create(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("ControllerEventHandler.Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ControllerEventHandler_Delete(t *testing.T) {
	slurmKeyRef := testutils.NewSlurmKeyRef("foo")
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef("foo")
	controller := testutils.NewController("slurm1", slurmKeyRef, jwtHs256KeyRef, nil)
	nodesetA := testutils.NewNodeset("slurmA", controller, 2)
	nodesetB := testutils.NewNodeset("slurmB", controller, 2)
	controller2 := testutils.NewController("slurm2", slurmKeyRef, jwtHs256KeyRef, nil)
	nodeset2A := testutils.NewNodeset("slurm2A", controller2, 2)
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
			name: "0 Refs",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: controller,
				},
				q: newQueue(),
			},
			want: 0,
		},
		{
			name: "1 Refs, controller 1",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: controller,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "1 Refs, controller 2",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: controller,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "2 Refs",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					nodesetB,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: controller,
				},
				q: newQueue(),
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewControllerEventHandler(tt.fields.Reader)
			h.Delete(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("ControllerEventHandler.Delete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ControllerEventHandler_Generic(t *testing.T) {
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
			h := NewControllerEventHandler(tt.fields.Reader)
			h.Generic(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("ControllerEventHandler.Generic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ControllerEventHandler_Update(t *testing.T) {
	slurmKeyRef := testutils.NewSlurmKeyRef("foo")
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef("foo")
	controller := testutils.NewController("slurm1", slurmKeyRef, jwtHs256KeyRef, nil)
	nodesetA := testutils.NewNodeset("slurmA", controller, 2)
	nodesetB := testutils.NewNodeset("slurmB", controller, 2)
	controller2 := testutils.NewController("slurm2", slurmKeyRef, jwtHs256KeyRef, nil)
	nodeset2A := testutils.NewNodeset("slurm2A", controller2, 2)
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
			name: "0 Refs",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: controller,
					ObjectNew: controller,
				},
				q: newQueue(),
			},
			want: 0,
		},
		{
			name: "1 Refs, controller 1",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: controller,
					ObjectNew: controller,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "1 Refs, controller 2",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: controller2,
					ObjectNew: controller2,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "2 Refs",
			fields: fields{
				Reader: fake.NewFakeClient(
					controller,
					nodesetA,
					nodesetB,
					controller2,
					nodeset2A,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: controller,
					ObjectNew: controller,
				},
				q: newQueue(),
			},
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewControllerEventHandler(tt.fields.Reader)
			h.Update(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("ControllerEventHandler.Update() = %v, want %v", got, tt.want)
			}
		})
	}
}
