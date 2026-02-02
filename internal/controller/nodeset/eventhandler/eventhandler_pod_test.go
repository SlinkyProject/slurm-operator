// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func Test_PodEventHandler_Create(t *testing.T) {
	type fields struct {
		Reader       client.Reader
		expectations *kubecontroller.UIDTrackingControllerExpectations
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
			name: "Empty",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{},
				q:   newQueue(),
			},
			want: 0,
		},
		{
			name: "Pod",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: corev1.NamespaceDefault,
							Name:      "foo",
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
			h := NewPodEventHandler(tt.fields.Reader, tt.fields.expectations)
			h.Create(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got > tt.want {
				t.Errorf("Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_PodEventHandler_Delete(t *testing.T) {
	type fields struct {
		Reader       client.Reader
		expectations *kubecontroller.UIDTrackingControllerExpectations
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
			name: "Empty",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{},
				q:   newQueue(),
			},
			want: 0,
		},
		{
			name: "Pod",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: corev1.NamespaceDefault,
							Name:      "foo",
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
			h := NewPodEventHandler(tt.fields.Reader, tt.fields.expectations)
			h.Delete(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got > tt.want {
				t.Errorf("Delete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_PodEventHandler_Generic(t *testing.T) {
	type fields struct {
		Reader       client.Reader
		expectations *kubecontroller.UIDTrackingControllerExpectations
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
		{
			name: "Pod",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.GenericEvent{
					Object: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: corev1.NamespaceDefault,
							Name:      "foo",
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
			h := NewPodEventHandler(tt.fields.Reader, tt.fields.expectations)
			h.Generic(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got > tt.want {
				t.Errorf("Generic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_PodEventHandler_Update(t *testing.T) {
	type fields struct {
		Reader       client.Reader
		expectations *kubecontroller.UIDTrackingControllerExpectations
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
			name: "Empty",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{},
				q:   newQueue(),
			},
			want: 0,
		},
		{
			name: "Pod",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: corev1.NamespaceDefault,
							Name:      "foo",
						},
					},
					ObjectNew: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: corev1.NamespaceDefault,
							Name:      "foo",
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
			h := NewPodEventHandler(tt.fields.Reader, tt.fields.expectations)
			h.Update(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got > tt.want {
				t.Errorf("Update() = %v, want %v", got, tt.want)
			}
		})
	}
}
