// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/indexes"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
)

func Test_nodeEventHandler_Create(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &nodeEventHandler{
				Reader: tt.fields.Reader,
			}
			h.Create(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("nodeEventHandler.Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeEventHandler_Delete(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &nodeEventHandler{
				Reader: tt.fields.Reader,
			}
			h.Delete(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("nodeEventHandler.Delete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeEventHandler_Generic(t *testing.T) {
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
			h := &nodeEventHandler{
				Reader: tt.fields.Reader,
			}
			h.Generic(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("nodeEventHandler.Generic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeEventHandler_Update(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	nodeset := newNodeSet("foo", "slurm", 0)
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
			name: "Node cordoned - should enqueue NodeSet",
			fields: fields{
				Reader: indexes.NewFakeClientBuilderWithIndexes(
					nodeset,
					newNodeSetPod(nodeset, 0, "test-node"),
					newNodeSetPod(nodeset, 1, "test-node2"),
				).Build(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: newNode("test-node", false),
					ObjectNew: newNode("test-node", true),
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "Node uncordoned - should enqueue NodeSet",
			fields: fields{
				Reader: indexes.NewFakeClientBuilderWithIndexes(
					nodeset,
					newNodeSetPod(nodeset, 0, "test-node"),
				).Build(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: newNode("test-node", true),
					ObjectNew: newNode("test-node", false),
				},
				q: newQueue(),
			},
			want: 1, // Should enqueue 1 NodeSet for reconciliation
		},
		{
			name: "No cordon change - should not enqueue",
			fields: fields{
				Reader: indexes.NewFakeClientBuilderWithIndexes().Build(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: newNode("test-node", false),
					ObjectNew: newNode("test-node", false),
				},
				q: newQueue(),
			},
			want: 0, // Should not enqueue anything
		},
		{
			name: "No worker pods on node - should not enqueue",
			fields: fields{
				Reader: indexes.NewFakeClientBuilderWithIndexes().Build(), // No pods
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: newNode("test-node", false),
					ObjectNew: newNode("test-node", true),
				},
				q: newQueue(),
			},
			want: 0, // Should not enqueue anything
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &nodeEventHandler{
				Reader: tt.fields.Reader,
			}
			h.Update(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("nodeEventHandler.Update() = %v, want %v", got, tt.want)
			}
		})
	}
}

func newNodeSetPod(nodeset *slinkyv1beta1.NodeSet, ordinal int, nodeName string) *corev1.Pod {
	ctld := &slinkyv1beta1.Controller{}
	pod := nodesetutils.NewNodeSetPod(nodeset, ctld, ordinal, "")
	pod.Spec.NodeName = nodeName
	return pod
}

func newNode(name string, unschedulable bool) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.NodeSpec{
			Unschedulable: unschedulable,
		},
	}
	return node
}
