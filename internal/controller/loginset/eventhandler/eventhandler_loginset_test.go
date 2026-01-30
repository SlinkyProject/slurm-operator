// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"
	"testing"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Test_ControllerEventHandler_Create(t *testing.T) {
	type fields struct {
		client client.Client
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
			name: "empty",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{},
				q:   newQueue(),
			},
			want: 0,
		},
		{
			name: "non-empty",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.RestApi{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
						Spec: slinkyv1beta1.RestApiSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name: "slurm",
							},
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: &slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					},
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewControllerEventHandler(tt.fields.client)
			e.Create(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got > tt.want {
				t.Errorf("Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ControllerEventHandler_Update(t *testing.T) {
	type fields struct {
		client client.Client
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
			name: "empty",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{},
				q:   newQueue(),
			},
			want: 0,
		},
		{
			name: "non-empty",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.RestApi{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
						Spec: slinkyv1beta1.RestApiSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name: "slurm",
							},
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectNew: &slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					},
					ObjectOld: &slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					},
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewControllerEventHandler(tt.fields.client)
			e.Update(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got > tt.want {
				t.Errorf("Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ControllerEventHandler_Delete(t *testing.T) {
	type fields struct {
		client client.Client
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
			name: "empty",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{},
				q:   newQueue(),
			},
			want: 0,
		},
		{
			name: "non-empty",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.RestApi{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
						Spec: slinkyv1beta1.RestApiSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name: "slurm",
							},
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: &slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					},
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewControllerEventHandler(tt.fields.client)
			e.Delete(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got > tt.want {
				t.Errorf("Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ControllerEventHandler_Generic(t *testing.T) {
	type fields struct {
		client client.Client
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
			name: "empty",
			fields: fields{
				client: fake.NewFakeClient(),
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
			e := NewControllerEventHandler(tt.fields.client)
			e.Generic(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got > tt.want {
				t.Errorf("Create() = %v, want %v", got, tt.want)
			}
		})
	}
}
