// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/indexes"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
)

func Test_NodeEventHandler_Create(t *testing.T) {
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
			h := NewNodeEventHandler(tt.fields.Reader, nil)
			h.Create(tt.args.ctx, tt.args.evt, tt.args.q)
			require.Equal(t, tt.want, tt.args.q.Len())
		})
	}
}

func Test_NodeEventHandler_Delete(t *testing.T) {
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
			h := NewNodeEventHandler(tt.fields.Reader, nil)
			h.Delete(tt.args.ctx, tt.args.evt, tt.args.q)
			require.Equal(t, tt.want, tt.args.q.Len())
		})
	}
}

func Test_NodeEventHandler_Generic(t *testing.T) {
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
			h := NewNodeEventHandler(tt.fields.Reader, nil)
			h.Generic(tt.args.ctx, tt.args.evt, tt.args.q)
			require.Equal(t, tt.want, tt.args.q.Len())
		})
	}
}

func Test_NodeEventHandler_Update(t *testing.T) {
	nodeset := newNodeSet("foo", "slurm", 0)
	cl := fake.NewFakeClient()
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
					newNodeSetPod(cl, nodeset, 0, "test-node"),
					newNodeSetPod(cl, nodeset, 1, "test-node2"),
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
					newNodeSetPod(cl, nodeset, 0, "test-node"),
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
			h := NewNodeEventHandler(tt.fields.Reader, nil)
			h.Update(tt.args.ctx, tt.args.evt, tt.args.q)
			require.Equal(t, tt.want, tt.args.q.Len())
		})
	}
}

func Test_NodeEventHandler_Update_PropagatedConditions(t *testing.T) {
	nodeset := newNodeSet("foo", "slurm", 0)
	cl := fake.NewFakeClient()
	propagated := []corev1.NodeConditionType{"MaintenancePlanned", "DrainInProgress"}

	withConditions := func(node *corev1.Node, conds ...corev1.NodeCondition) *corev1.Node {
		node.Status.Conditions = conds
		return node
	}
	maintenance := corev1.NodeCondition{
		Type:    "MaintenancePlanned",
		Status:  corev1.ConditionTrue,
		Reason:  "MaintenanceWindow",
		Message: "Hardware maintenance is scheduled",
	}
	draining := corev1.NodeCondition{
		Type:    "DrainInProgress",
		Status:  corev1.ConditionTrue,
		Reason:  "AdminDrain",
		Message: "Draining for firmware update",
	}
	maintenanceHeartbeat := *maintenance.DeepCopy()
	maintenanceHeartbeat.LastHeartbeatTime = metav1.Now()
	maintenanceDone := *maintenance.DeepCopy()
	maintenanceDone.Status = corev1.ConditionFalse
	diskPressure := corev1.NodeCondition{
		Type:   corev1.NodeDiskPressure,
		Status: corev1.ConditionTrue,
		Reason: "KubeletHasDiskPressure",
	}

	tests := []struct {
		name       string
		propagated []corev1.NodeConditionType
		oldNode    *corev1.Node
		newNode    *corev1.Node
		want       int
	}{
		{
			name:       "cordoned node gains propagated condition - should enqueue",
			propagated: propagated,
			oldNode:    withConditions(newNode("test-node", true), maintenance),
			newNode:    withConditions(newNode("test-node", true), maintenance, draining),
			want:       1,
		},
		{
			name:       "cordoned node propagated condition status changes - should enqueue",
			propagated: propagated,
			oldNode:    withConditions(newNode("test-node", true), maintenance),
			newNode:    withConditions(newNode("test-node", true), maintenanceDone),
			want:       1,
		},
		{
			name:       "cordoned node loses propagated condition - should enqueue",
			propagated: propagated,
			oldNode:    withConditions(newNode("test-node", true), maintenance, draining),
			newNode:    withConditions(newNode("test-node", true), maintenance),
			want:       1,
		},
		{
			name:       "cordoned node propagated condition heartbeat only - should not enqueue",
			propagated: propagated,
			oldNode:    withConditions(newNode("test-node", true), maintenance),
			newNode:    withConditions(newNode("test-node", true), maintenanceHeartbeat),
			want:       0,
		},
		{
			name:       "cordoned node non-propagated condition changes - should not enqueue",
			propagated: propagated,
			oldNode:    withConditions(newNode("test-node", true)),
			newNode:    withConditions(newNode("test-node", true), diskPressure),
			want:       0,
		},
		{
			name:       "schedulable node propagated condition changes - should not enqueue",
			propagated: propagated,
			oldNode:    withConditions(newNode("test-node", false)),
			newNode:    withConditions(newNode("test-node", false), maintenance),
			want:       0,
		},
		{
			name:       "no propagated conditions configured - should not enqueue",
			propagated: nil,
			oldNode:    withConditions(newNode("test-node", true)),
			newNode:    withConditions(newNode("test-node", true), maintenance),
			want:       0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := indexes.NewFakeClientBuilderWithIndexes(
				nodeset,
				newNodeSetPod(cl, nodeset, 0, "test-node"),
			).Build()
			q := newQueue()
			h := NewNodeEventHandler(reader, tt.propagated)
			h.Update(t.Context(), event.UpdateEvent{ObjectOld: tt.oldNode, ObjectNew: tt.newNode}, q)
			require.Equal(t, tt.want, q.Len())
		})
	}
}

func newNodeSetPod(client client.Client, nodeset *slinkyv1beta1.NodeSet, ordinal int, nodeName string) *corev1.Pod {
	ctld := &slinkyv1beta1.Controller{}
	pod := nodesetutils.NewNodeSetStatefulSetPod(client, nodeset, ctld, ordinal, "")
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
