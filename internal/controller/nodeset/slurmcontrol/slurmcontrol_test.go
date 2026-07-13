// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	api "github.com/SlinkyProject/slurm-client/api/v0044"
	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/client/fake"
	"github.com/SlinkyProject/slurm-client/pkg/client/interceptor"
	"github.com/SlinkyProject/slurm-client/pkg/object"
	"github.com/SlinkyProject/slurm-client/pkg/types"
	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podinfo"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
	"github.com/puttsk/hostlist"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	kubefake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func slurmUpdateFn(_ context.Context, obj object.Object, req any, opts ...client.UpdateOption) error {
	switch o := obj.(type) {
	case *types.V0044Node:
		r, ok := req.(api.V0044UpdateNodeMsg)
		if !ok {
			return errors.New("failed to cast request object")
		}
		stateSet := set.New(ptr.Deref(o.State, []api.V0044NodeState{})...)
		statesReq := ptr.Deref(r.State, []api.V0044UpdateNodeMsgState{})
		for _, stateReq := range statesReq {
			switch stateReq {
			case api.V0044UpdateNodeMsgStateUNDRAIN:
				stateSet.Delete(api.V0044NodeStateDRAIN)
			default:
				stateSet.Insert(api.V0044NodeState(stateReq))
			}
		}
		o.State = ptr.To(stateSet.UnsortedList())
		o.Comment = r.Comment
		o.Reason = r.Reason
		o.Topology = r.TopologyStr
		o.Features = r.Features
		o.ActiveFeatures = r.FeaturesAct
	case *types.V0044ReservationInfo:
		_, ok := req.(api.V0044ReservationDescMsg)
		if !ok {
			return errors.New("failed to cast request object")
		}
	default:
		return errors.New("failed to cast slurm object")
	}
	return nil
}

func newNodeSet(name, controllerName string, replicas int32) *slinkyv1beta1.NodeSet {
	return &slinkyv1beta1.NodeSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      name,
		},
		Spec: slinkyv1beta1.NodeSetSpec{
			ControllerRef: corev1.LocalObjectReference{
				Name: controllerName,
			},
			Replicas:    &replicas,
			ScalingMode: slinkyv1beta1.ScalingModeStatefulset,
		},
	}
}

func newSlurmClientMap(controllerName string, client client.Client) *clientmap.ClientMap {
	cm := clientmap.NewClientMap()
	key := k8stypes.NamespacedName{
		Namespace: corev1.NamespaceDefault,
		Name:      controllerName,
	}
	cm.Add(key, client)
	return cm
}

func Test_realSlurmControl_UpdateNodeWithPodInfo(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	nodeset.UID = k8stypes.UID("foo-uid")
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	pod.Spec.NodeName = "foo"
	type fields struct {
		node *types.V0044Node
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantPodInfo podinfo.PodInfo
		wantErr     bool
	}{
		{
			name: "smoke",
			fields: fields{
				node: &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				},
			},
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			wantPodInfo: podinfo.PodInfo{
				Namespace:   nodeset.Namespace,
				PodName:     pod.Name,
				Node:        pod.Spec.NodeName,
				NodeSetName: nodeset.Name,
				NodeSetUID:  string(nodeset.UID),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sclient := fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(tt.fields.node).Build()
			controllerName := tt.args.nodeset.Spec.ControllerRef.Name
			r := NewSlurmControl(newSlurmClientMap(controllerName, sclient))
			err := r.UpdateNodeWithPodInfo(tt.args.ctx, tt.args.nodeset, tt.args.pod)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			checkNode := &types.V0044Node{}
			if getErr := sclient.Get(ctx, tt.fields.node.GetKey(), checkNode); getErr != nil {
				if !tolerateError(getErr) {
					require.NoError(t, getErr)
				}
			}
			checkPodInfo := podinfo.PodInfo{}
			_ = podinfo.ParseIntoPodInfo(checkNode.Comment, &checkPodInfo)
			require.True(t, apiequality.Semantic.DeepEqual(checkPodInfo, tt.wantPodInfo), "UpdateNodeWithPodInfo() podInfo = %v, want %v", checkPodInfo, tt.wantPodInfo)
		})
	}
}

func Test_realSlurmControl_MakeNodeDrain(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		node *types.V0044Node
	}
	type args struct {
		ctx            context.Context
		nodeset        *slinkyv1beta1.NodeSet
		pod            *corev1.Pod
		reason         string
		overrideReason bool
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantReason string
		wantErr    bool
	}{
		{
			name: "not drained, no reason",
			fields: fields{
				node: &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				},
			},
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
				reason:  "test",
			},
			wantReason: FormatNodeReason("test"),
		},
		{
			name: "already drained, preserve reason",
			fields: fields{
				node: &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: new(nodesetutils.GetSlurmNodeName(pod)),
						State: new([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
						}),
						Reason: new("already drained"),
					},
				},
			},
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
				reason:  "do not set",
			},
			wantReason: "already drained",
		},
		{
			name: "already drained, override reason",
			fields: fields{
				node: &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: new(nodesetutils.GetSlurmNodeName(pod)),
						State: new([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
						}),
						Reason: new("already drained"),
					},
				},
			},
			args: args{
				ctx:            ctx,
				nodeset:        nodeset,
				pod:            pod,
				reason:         "override",
				overrideReason: true,
			},
			wantReason: FormatNodeReason("override"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sclient := fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(tt.fields.node).Build()
			controllerName := tt.args.nodeset.Spec.ControllerRef.Name
			r := NewSlurmControl(newSlurmClientMap(controllerName, sclient))
			err := r.MakeNodeDrain(tt.args.ctx, tt.args.nodeset, tt.args.pod, tt.args.reason, tt.args.overrideReason)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			checkNode := &types.V0044Node{}
			if getErr := sclient.Get(ctx, tt.fields.node.GetKey(), checkNode); getErr != nil {
				if !tolerateError(getErr) {
					require.NoError(t, getErr)
				}
			}
			isDrain := checkNode.GetStateAsSet().Has(api.V0044NodeStateDRAIN)
			require.True(t, isDrain, "MakeNodeDrain() failed to DRAIN the node")
			nodeReason := ptr.Deref(checkNode.Reason, "")
			require.Equal(t, tt.wantReason, nodeReason)
		})
	}
}

func Test_realSlurmControl_MakeNodeUndrain(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		node *types.V0044Node
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
		reason  string
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantUndrain bool
		wantErr     bool
	}{
		{
			name: "drain",
			fields: fields{
				node: &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDRAIN,
						}),
					},
				},
			},
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
				reason:  "test",
			},
			wantUndrain: true,
		},
		{
			name: "idle",
			fields: fields{
				node: &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				},
			},
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
				reason:  "test",
			},
			wantUndrain: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sclient := fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(tt.fields.node).Build()
			controllerName := tt.args.nodeset.Spec.ControllerRef.Name
			r := NewSlurmControl(newSlurmClientMap(controllerName, sclient))
			err := r.MakeNodeUndrain(tt.args.ctx, tt.args.nodeset, tt.args.pod, tt.args.reason)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			checkNode := &types.V0044Node{}
			if getErr := sclient.Get(ctx, tt.fields.node.GetKey(), checkNode); getErr != nil {
				if !tolerateError(getErr) {
					require.NoError(t, getErr)
				}
			}
			isUndrain := !checkNode.GetStateAsSet().Has(api.V0044NodeStateDRAIN)
			require.Equal(t, tt.wantUndrain, isUndrain)
		})
	}
}

func Test_realSlurmControl_UpdateNodeTopology(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		node *types.V0044Node
	}
	type args struct {
		ctx          context.Context
		nodeset      *slinkyv1beta1.NodeSet
		pod          *corev1.Pod
		topologySpec string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "empty",
			fields: fields{
				node: &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				},
			},
			args: args{
				ctx:          ctx,
				nodeset:      nodeset,
				pod:          pod,
				topologySpec: "",
			},
		},
		{
			name: "smoke",
			fields: fields{
				node: &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				},
			},
			args: args{
				ctx:          ctx,
				nodeset:      nodeset,
				pod:          pod,
				topologySpec: "foo:bar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sclient := fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(tt.fields.node).Build()
			controllerName := tt.args.nodeset.Spec.ControllerRef.Name
			r := NewSlurmControl(newSlurmClientMap(controllerName, sclient))
			err := r.UpdateNodeTopology(tt.args.ctx, tt.args.nodeset, tt.args.pod, tt.args.topologySpec)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			checkNode := &types.V0044Node{}
			if getErr := sclient.Get(ctx, tt.fields.node.GetKey(), checkNode); getErr != nil {
				if !tolerateError(getErr) {
					require.NoError(t, getErr)
				}
			}
			got := ptr.Deref(checkNode.Topology, "")
			require.True(t, apiequality.Semantic.DeepEqual(got, tt.args.topologySpec), "UpdateNodeTopology() topologySpec = %v", got)
		})
	}
}

func Test_realSlurmControl_UpdateNodeFeatures(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	prefix := slinkyv1beta1.NodeFeaturePrefix
	// nodeWith returns an IDLE Slurm node with the given available/active features.
	nodeWith := func(available, active []string) *types.V0044Node {
		n := &types.V0044Node{
			V0044Node: api.V0044Node{
				Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod)),
				State: ptr.To([]api.V0044NodeState{api.V0044NodeStateIDLE}),
			},
		}
		if available != nil {
			n.Features = ptr.To(api.V0044CsvString(available))
		}
		if active != nil {
			n.ActiveFeatures = ptr.To(api.V0044CsvString(active))
		}
		return n
	}
	type args struct {
		ctx      context.Context
		nodeset  *slinkyv1beta1.NodeSet
		pod      *corev1.Pod
		features []string
	}
	tests := []struct {
		name          string
		node          *types.V0044Node
		args          args
		notRegistered bool  // node absent from slurmctld: Get returns a tolerated 404
		updateErr     error // non-nil makes the Slurm Update fail with this error
		wantErr       bool
		wantSkip      bool
		wantAvailable []string
		wantActive    []string
	}{
		{
			name:          "adds prefixed features, preserves baseline",
			node:          nodeWith([]string{"foo"}, []string{"foo"}),
			args:          args{ctx: ctx, nodeset: nodeset, pod: pod, features: []string{"a100"}},
			wantAvailable: []string{"foo", prefix + "a100"},
			wantActive:    []string{"foo", prefix + "a100"},
		},
		{
			// Stale prefixed feature is replaced; non-prefixed (external) features stay.
			name:          "replaces stale prefixed, preserves external",
			node:          nodeWith([]string{"foo", prefix + "old"}, []string{"foo", prefix + "old"}),
			args:          args{ctx: ctx, nodeset: nodeset, pod: pod, features: []string{"new"}},
			wantAvailable: []string{"foo", prefix + "new"},
			wantActive:    []string{"foo", prefix + "new"},
		},
		{
			// Empty desired (annotation removed) strips the prefixed namespace only.
			name:          "empty desired strips prefixed, preserves external",
			node:          nodeWith([]string{"foo", prefix + "old"}, []string{"foo", prefix + "old"}),
			args:          args{ctx: ctx, nodeset: nodeset, pod: pod, features: nil},
			wantAvailable: []string{"foo"},
			wantActive:    []string{"foo"},
		},
		{
			name:          "already in sync skips",
			node:          nodeWith([]string{"foo", prefix + "a100"}, []string{"foo", prefix + "a100"}),
			args:          args{ctx: ctx, nodeset: nodeset, pod: pod, features: []string{"a100"}},
			wantSkip:      true,
			wantAvailable: []string{"foo", prefix + "a100"},
			wantActive:    []string{"foo", prefix + "a100"},
		},
		{
			name:          "no prefixed and empty desired skips",
			node:          nodeWith([]string{"foo"}, []string{"foo"}),
			args:          args{ctx: ctx, nodeset: nodeset, pod: pod, features: nil},
			wantSkip:      true,
			wantAvailable: []string{"foo"},
			wantActive:    []string{"foo"},
		},
		{
			// NodeFeaturesPlugins case: changeable features (mig=on/off available,
			// mig=on active) must survive; only the prefixed namespace is replaced.
			name:          "plugin-managed features survive",
			node:          nodeWith([]string{"foo", "mig=on", "mig=off", prefix + "old"}, []string{"foo", "mig=on", prefix + "old"}),
			args:          args{ctx: ctx, nodeset: nodeset, pod: pod, features: []string{"nn-a"}},
			wantAvailable: []string{"foo", "mig=off", "mig=on", prefix + "nn-a"},
			wantActive:    []string{"foo", "mig=on", prefix + "nn-a"},
		},
		{
			// Node not registered in slurmctld yet: Get returns a tolerated 404 and
			// the call is a no-op.
			name:          "node not registered is tolerated",
			node:          nodeWith([]string{"foo"}, []string{"foo"}),
			args:          args{ctx: ctx, nodeset: nodeset, pod: pod, features: []string{"a100"}},
			notRegistered: true,
		},
		{
			// A non-tolerated error from the Slurm update is surfaced to the caller.
			name:      "update error is returned",
			node:      nodeWith([]string{"foo"}, []string{"foo"}),
			args:      args{ctx: ctx, nodeset: nodeset, pod: pod, features: []string{"a100"}},
			updateErr: errors.New("boom"),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updates := 0
			updateFn := func(ctx context.Context, obj object.Object, req any, opts ...client.UpdateOption) error {
				updates++
				if tt.updateErr != nil {
					return tt.updateErr
				}
				return slurmUpdateFn(ctx, obj, req, opts...)
			}
			builder := fake.NewClientBuilder().WithUpdateFn(updateFn)
			if !tt.notRegistered {
				builder = builder.WithObjects(tt.node)
			}
			sclient := builder.Build()
			controllerName := tt.args.nodeset.Spec.ControllerRef.Name
			r := NewSlurmControl(newSlurmClientMap(controllerName, sclient))
			if err := r.UpdateNodeFeatures(tt.args.ctx, tt.args.nodeset, tt.args.pod, prefix, tt.args.features); (err != nil) != tt.wantErr {
				t.Errorf("UpdateNodeFeatures() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantSkip && updates != 0 {
				t.Errorf("UpdateNodeFeatures() issued %d update(s), want 0 (already in sync)", updates)
			}
			// The Slurm node feature set is only well-defined on the success path; for
			// the tolerated-404 and error cases there is nothing to assert.
			if tt.notRegistered || tt.wantErr {
				return
			}
			checkNode := &types.V0044Node{}
			if err := sclient.Get(ctx, tt.node.GetKey(), checkNode); err != nil {
				if !tolerateError(err) {
					t.Fatalf("client.Get() = %v", err)
				}
			}
			gotAvail := ptr.Deref(checkNode.Features, api.V0044CsvString{})
			gotActive := ptr.Deref(checkNode.ActiveFeatures, api.V0044CsvString{})
			slices.Sort(gotAvail)
			slices.Sort(gotActive)
			wantAvail := slices.Clone(tt.wantAvailable)
			wantActive := slices.Clone(tt.wantActive)
			slices.Sort(wantAvail)
			slices.Sort(wantActive)
			if !slices.Equal(gotAvail, wantAvail) {
				t.Errorf("UpdateNodeFeatures() available = %v, want %v", gotAvail, wantAvail)
			}
			if !slices.Equal(gotActive, wantActive) {
				t.Errorf("UpdateNodeFeatures() active = %v, want %v", gotActive, wantActive)
			}
		})
	}
}

func Test_realSlurmControl_IsNodeDrain(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		clientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "Not DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Is DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realSlurmControl{
				clientMap: tt.fields.clientMap,
			}
			got, err := r.IsNodeDrain(tt.args.ctx, tt.args.nodeset, tt.args.pod)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_realSlurmControl_IsNodeDrained(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		clientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "IDLE",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "MIXED",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateMIXED,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "DOWN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "IDLE+DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: true,
		},
		{
			name: "MIXED+DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateMIXED,
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "ALLOC+DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateALLOCATED,
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "DOWN+DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "IDLE+COMPLETING",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
							api.V0044NodeStateCOMPLETING,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "IDLE+DRAIN+COMPLETING",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
							api.V0044NodeStateCOMPLETING,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "IDLE+DRAIN+UNDRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
							api.V0044NodeStateUNDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realSlurmControl{
				clientMap: tt.fields.clientMap,
			}
			got, err := r.IsNodeDrained(tt.args.ctx, tt.args.nodeset, tt.args.pod)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_realSlurmControl_IsNodeDownForUnresponsive(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		clientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "IDLE",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "MIXED",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateMIXED,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "DOWN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "IDLE+DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "MIXED+DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateMIXED,
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "ALLOC+DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateALLOCATED,
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "DOWN+DRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
							api.V0044NodeStateDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "IDLE+COMPLETING",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
							api.V0044NodeStateCOMPLETING,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "IDLE+DRAIN+COMPLETING",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
							api.V0044NodeStateCOMPLETING,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "IDLE+DRAIN+UNDRAIN",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
							api.V0044NodeStateDRAIN,
							api.V0044NodeStateUNDRAIN,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "DOWN+NOT_RESPONDING",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
							api.V0044NodeStateNOTRESPONDING,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
		{
			name: "DOWN+Reason Not responding",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
						}),
						Reason: ptr.To("Not responding"),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: true,
		},
		{
			name: "DOWN+Other reason",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
						}),
						Reason: ptr.To("test reason"),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realSlurmControl{
				clientMap: tt.fields.clientMap,
			}
			got, err := r.IsNodeDownForUnresponsive(tt.args.ctx, tt.args.nodeset, tt.args.pod)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_realSlurmControl_IsNodeReasonOurs(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		clientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "no reason",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateIDLE,
						}),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: true,
		},
		{
			name: "internal reason",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
						}),
						Reason: new(FormatNodeReason("foo")),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: true,
		},
		{
			name: "external reason",
			fields: func() fields {
				node := &types.V0044Node{
					V0044Node: api.V0044Node{
						Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
						}),
						Reason: ptr.To("foo"),
					},
				}
				sclient := fake.NewClientBuilder().WithObjects(node).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realSlurmControl{
				clientMap: tt.fields.clientMap,
			}
			got, err := r.IsNodeReasonOurs(tt.args.ctx, tt.args.nodeset, tt.args.pod)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_realSlurmControl_CalculateNodeStatus(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	nodeset2 := newNodeSet("baz", controller.Name, 1)
	type fields struct {
		clientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    SlurmNodeStatus
		wantErr bool
	}{
		{
			name: "Empty",
			fields: func() fields {
				nodeList := &types.V0044NodeList{
					Items: []types.V0044Node{},
				}
				sclient := fake.NewClientBuilder().WithLists(nodeList).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pods:    []*corev1.Pod{},
			},
			want:    SlurmNodeStatus{},
			wantErr: false,
		},
		{
			name: "Different NodeSets",
			fields: func() fields {
				nodeList := &types.V0044NodeList{
					Items: []types.V0044Node{
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset2, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
								}),
							},
						},
					},
				}
				sclient := fake.NewClientBuilder().WithLists(nodeList).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pods: []*corev1.Pod{
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 1,

				Idle: 1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionIdle,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					return nodeStates
				}(),
			},
			wantErr: false,
		},
		{
			name: "Only base state",
			fields: func() fields {
				nodeList := &types.V0044NodeList{
					Items: []types.V0044Node{
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
								}),
							},
						},
					},
				}
				sclient := fake.NewClientBuilder().WithLists(nodeList).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pods: []*corev1.Pod{
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 1,

				Idle: 1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionIdle,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					return nodeStates
				}(),
			},
			wantErr: false,
		},
		{
			name: "Base and flag state",
			fields: func() fields {
				nodeList := &types.V0044NodeList{
					Items: []types.V0044Node{
						{
							V0044Node: api.V0044Node{
								Name:   ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))),
								Reason: ptr.To("Node drain"),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
									api.V0044NodeStateDRAIN,
								}),
							},
						},
					},
				}
				sclient := fake.NewClientBuilder().WithLists(nodeList).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pods: []*corev1.Pod{
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 1,

				Idle:  1,
				Drain: 1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionIdle,
							Status:  corev1.ConditionTrue,
							Message: "Node drain",
						},
						{
							Type:    slurmconditions.PodConditionDrain,
							Status:  corev1.ConditionTrue,
							Message: "Node drain",
						},
					}
					return nodeStates
				}(),
			},
			wantErr: false,
		},
		{
			name: "All base states",
			fields: func() fields {
				nodeList := &types.V0044NodeList{
					Items: []types.V0044Node{
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateALLOCATED,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateDOWN,
								}),
								Reason: ptr.To("Node is down"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateERROR,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateFUTURE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateMIXED,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateUNKNOWN,
								}),
							},
						},
					},
				}
				sclient := fake.NewClientBuilder().WithLists(nodeList).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pods: []*corev1.Pod{
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 7,

				Allocated: 1,
				Down:      1,
				Error:     1,
				Future:    1,
				Idle:      1,
				Mixed:     1,
				Unknown:   1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionAllocated,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionDown,
							Status:  corev1.ConditionTrue,
							Message: "Node is down",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionError,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionFuture,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionIdle,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionMixed,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionUnknown,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					return nodeStates
				}(),
			},
			wantErr: false,
		},
		{
			name: "All flag states",
			fields: func() fields {
				nodeList := &types.V0044NodeList{
					Items: []types.V0044Node{
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateCOMPLETING,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateDRAIN,
								}),
								Reason: ptr.To("Node set to drain"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateFAIL,
								}),
								Reason: ptr.To("Node set to fail"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateINVALID,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateINVALIDREG,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateMAINTENANCE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateNOTRESPONDING,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 7, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateUNDRAIN,
								}),
							},
						},
					},
				}
				sclient := fake.NewClientBuilder().WithLists(nodeList).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pods: []*corev1.Pod{
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 7, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 8,

				Completing:    1,
				Drain:         1,
				Fail:          1,
				Invalid:       1,
				InvalidReg:    1,
				Maintenance:   1,
				NotResponding: 1,
				Undrain:       1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionCompleting,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionDrain,
							Status:  corev1.ConditionTrue,
							Message: "Node set to drain",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionFail,
							Status:  corev1.ConditionTrue,
							Message: "Node set to fail",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionInvalid,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionInvalidReg,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionMaintenance,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionNotResponding,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 7, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionUndrain,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					return nodeStates
				}(),
			},
			wantErr: false,
		},
		{
			name: "All states",
			fields: func() fields {
				nodeList := &types.V0044NodeList{
					Items: []types.V0044Node{
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateALLOCATED,
									api.V0044NodeStateCOMPLETING,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateDOWN,
									api.V0044NodeStateDRAIN,
								}),
								Reason: ptr.To("Node set to down and drain"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateERROR,
									api.V0044NodeStateFAIL,
								}),
								Reason: ptr.To("Node set to error and fail"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateFUTURE,
									api.V0044NodeStateINVALID,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateFUTURE,
									api.V0044NodeStateINVALIDREG,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
									api.V0044NodeStateMAINTENANCE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateMIXED,
									api.V0044NodeStateNOTRESPONDING,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 7, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateUNKNOWN,
									api.V0044NodeStateUNDRAIN,
								}),
							},
						},
					},
				}
				sclient := fake.NewClientBuilder().WithLists(nodeList).Build()
				return fields{
					clientMap: newSlurmClientMap(controller.Name, sclient),
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pods: []*corev1.Pod{
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""),
					nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 7, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 8,

				Allocated: 1,
				Down:      1,
				Error:     1,
				Future:    2,
				Idle:      1,
				Mixed:     1,
				Unknown:   1,

				Completing:    1,
				Drain:         1,
				Fail:          1,
				Invalid:       1,
				InvalidReg:    1,
				Maintenance:   1,
				NotResponding: 1,
				Undrain:       1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 0, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionAllocated,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
						{
							Type:    slurmconditions.PodConditionCompleting,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 1, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionDown,
							Status:  corev1.ConditionTrue,
							Message: "Node set to down and drain",
						},
						{
							Type:    slurmconditions.PodConditionDrain,
							Status:  corev1.ConditionTrue,
							Message: "Node set to down and drain",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 2, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionError,
							Status:  corev1.ConditionTrue,
							Message: "Node set to error and fail",
						},
						{
							Type:    slurmconditions.PodConditionFail,
							Status:  corev1.ConditionTrue,
							Message: "Node set to error and fail",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 3, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionFuture,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
						{
							Type:    slurmconditions.PodConditionInvalid,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 4, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionFuture,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
						{
							Type:    slurmconditions.PodConditionInvalidReg,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 5, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionIdle,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
						{
							Type:    slurmconditions.PodConditionMaintenance,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 6, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionMixed,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
						{
							Type:    slurmconditions.PodConditionNotResponding,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetSlurmNodeName(nodesetutils.NewNodeSetStatefulSetPod(kubefake.NewFakeClient(), nodeset, controller, 7, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionUnknown,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
						{
							Type:    slurmconditions.PodConditionUndrain,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					return nodeStates
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realSlurmControl{
				clientMap: tt.fields.clientMap,
			}
			got, err := r.CalculateNodeStatus(tt.args.ctx, tt.args.nodeset, tt.args.pods)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.True(t, apiequality.Semantic.DeepEqual(got, tt.want), "realSlurmControl.CalculateNodeStatus() = %v, want %v", got, tt.want)
		})
	}
}

func Test_realSlurmControl_GetNodeDeadlines(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	kclient := kubefake.NewFakeClient()
	pod := nodesetutils.NewNodeSetStatefulSetPod(kclient, nodeset, controller, 0, "")
	pod2 := nodesetutils.NewNodeSetStatefulSetPod(kclient, nodeset, controller, 1, "")
	pods := []*corev1.Pod{pod, pod2}
	now := time.Now()
	type fields struct {
		nodeList *types.V0044NodeList
		jobList  *types.V0044JobInfoList
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "smoke",
			fields: func() fields {
				nodeList := &types.V0044NodeList{
					Items: []types.V0044Node{
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateMIXED,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(pod2)),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateMIXED,
								}),
							},
						},
					},
				}
				jobList := &types.V0044JobInfoList{
					Items: []types.V0044JobInfo{
						{
							V0044JobInfo: api.V0044JobInfo{
								JobId:     ptr.To[int32](1),
								JobState:  ptr.To([]api.V0044JobInfoJobState{api.V0044JobInfoJobStateRUNNING}),
								StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(now.Unix())}),
								TimeLimit: ptr.To(api.V0044Uint32NoValStruct{Number: ptr.To(30 * int32(time.Minute.Seconds()))}),
								Nodes: func() *string {
									hostlist, err := hostlist.Compress([]string{*nodeList.Items[0].Name})
									if err != nil {
										panic(err)
									}
									return ptr.To(hostlist)
								}(),
							},
						},
						{
							V0044JobInfo: api.V0044JobInfo{
								JobId:     ptr.To[int32](2),
								JobState:  ptr.To([]api.V0044JobInfoJobState{api.V0044JobInfoJobStateRUNNING}),
								StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(now.Unix())}),
								TimeLimit: ptr.To(api.V0044Uint32NoValStruct{Number: ptr.To(45 * int32(time.Minute.Seconds()))}),
								Nodes: func() *string {
									hostlist, err := hostlist.Compress([]string{*nodeList.Items[0].Name, *nodeList.Items[1].Name})
									if err != nil {
										panic(err)
									}
									return ptr.To(hostlist)
								}(),
							},
						},
						{
							V0044JobInfo: api.V0044JobInfo{
								JobId:     ptr.To[int32](3),
								JobState:  ptr.To([]api.V0044JobInfoJobState{api.V0044JobInfoJobStateRUNNING}),
								StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(now.Unix())}),
								TimeLimit: ptr.To(api.V0044Uint32NoValStruct{Number: ptr.To(int32(time.Hour.Seconds()))}),
								Nodes: func() *string {
									hostlist, err := hostlist.Compress([]string{*nodeList.Items[0].Name})
									if err != nil {
										panic(err)
									}
									return ptr.To(hostlist)
								}(),
							},
						},
						{
							V0044JobInfo: api.V0044JobInfo{
								JobId:    ptr.To[int32](4),
								JobState: ptr.To([]api.V0044JobInfoJobState{api.V0044JobInfoJobStateCOMPLETED}),
								Nodes: func() *string {
									hostlist, err := hostlist.Compress([]string{*nodeList.Items[0].Name, *nodeList.Items[1].Name})
									if err != nil {
										panic(err)
									}
									return ptr.To(hostlist)
								}(),
							},
						},
						{
							V0044JobInfo: api.V0044JobInfo{
								JobId:    ptr.To[int32](5),
								JobState: ptr.To([]api.V0044JobInfoJobState{api.V0044JobInfoJobStateCOMPLETED}),
								Nodes: func() *string {
									hostlist, err := hostlist.Compress([]string{*nodeList.Items[1].Name})
									if err != nil {
										panic(err)
									}
									return ptr.To(hostlist)
								}(),
							},
						},
					},
				}

				return fields{
					nodeList: nodeList,
					jobList:  jobList,
				}
			}(),
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pods:    pods,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sclient := fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithLists(tt.fields.nodeList, tt.fields.jobList).Build()
			controllerName := tt.args.nodeset.Spec.ControllerRef.Name
			r := NewSlurmControl(newSlurmClientMap(controllerName, sclient))
			got, err := r.GetNodeDeadlines(tt.args.ctx, tt.args.nodeset, tt.args.pods)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			for _, node := range tt.fields.nodeList.Items {
				ts := got.Peek(ptr.Deref(node.Name, ""))
				require.True(t, ts.After(now), "timestamp = %v, after = %v", ts, ts.After(now))
			}
		})
	}
}

func Test_realSlurmControl_GetNodesForPods(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	kclient := kubefake.NewFakeClient()
	type clientData struct {
		nodeList *types.V0044NodeList
	}
	type testCase struct {
		name       string
		clientData clientData
		nodeset    *slinkyv1beta1.NodeSet
		pods       []*corev1.Pod
		want       []string
		wantOk     bool
		wantErr    bool
	}
	tests := []testCase{
		func() testCase {
			nodeset := newNodeSet("foo", controller.Name, 1)
			pod := nodesetutils.NewNodeSetStatefulSetPod(kclient, nodeset, controller, 0, "")
			return testCase{
				name:    "empty",
				nodeset: nodeset,
				clientData: clientData{
					nodeList: &types.V0044NodeList{},
				},
				pods: []*corev1.Pod{
					pod,
				},
				want: []string{},
			}
		}(),
		func() testCase {
			ns0 := newNodeSet("ns0", controller.Name, 2)
			ns0pod0 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns0, controller, 0, "")
			ns0pod1 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns0, controller, 1, "")
			ns0pod0name := nodesetutils.GetSlurmNodeName(ns0pod0)
			ns0pod1name := nodesetutils.GetSlurmNodeName(ns0pod1)
			ns1 := newNodeSet("ns1", controller.Name, 2)
			ns1pod0 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns1, controller, 0, "")
			ns1pod1 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns1, controller, 1, "")
			ns1pod0name := nodesetutils.GetSlurmNodeName(ns1pod0)
			ns1pod1name := nodesetutils.GetSlurmNodeName(ns1pod1)
			return testCase{
				name:    "mixed",
				nodeset: ns0,
				clientData: clientData{
					nodeList: &types.V0044NodeList{
						Items: []types.V0044Node{
							{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
							{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
							{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
							{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
						},
					},
				},
				pods: []*corev1.Pod{
					ns0pod0,
					ns0pod1,
				},
				want: []string{
					ns0pod0name,
					ns0pod1name,
				},
				wantOk: true,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sclient := fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithLists(tt.clientData.nodeList).Build()
			controllerName := tt.nodeset.Spec.ControllerRef.Name
			r := NewSlurmControl(newSlurmClientMap(controllerName, sclient))
			got, ok, gotErr := r.GetNodesForPods(context.Background(), tt.nodeset, tt.pods)
			if gotErr != nil {
				if tt.wantErr {
					require.Error(t, gotErr)
				} else {
					require.NoError(t, gotErr)
				}
				return
			}
			if !ok {
				require.Equal(t, tt.wantOk, ok)
				return
			}
			if tt.wantErr {
				require.NoError(t, gotErr, "GetNodesForPods() succeeded unexpectedly")
			}
			slices.Sort(got)
			slices.Sort(tt.want)
			require.True(t, apiequality.Semantic.DeepEqual(got, tt.want), "GetNodesForPods() = %v, want %v", got, tt.want)
		})
	}
}

func Test_realSlurmControl_CheckReservationForNodeSet(t *testing.T) {
	// Configure times for testing
	now, err := time.Parse(time.RFC3339, "2026-03-04T00:00:00Z")
	require.NoError(t, err)
	startTime := now.Unix()
	endTime := now.Add(time.Hour).Unix()

	type testCase struct {
		name    string
		client  client.Client
		nodeset *slinkyv1beta1.NodeSet
		want    bool
		wantErr bool
	}
	tests := []testCase{
		{
			name: "invalid NodeSet (no controller ref)",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
			},
			client:  fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).Build(),
			want:    false,
			wantErr: false,
		},
		{
			name: "reservation does not exist",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
				},
			},
			client:  fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).Build(),
			wantErr: false,
			want:    false,
		},
		{
			name: "reservation exists",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.RollingUpdateNodeSetStrategyType,
					},
				},
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTime), Set: ptr.To(true)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTime), Set: ptr.To(true)}),
				},
			},
			).Build(),
			want:    true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllerName := tt.nodeset.Spec.ControllerRef.Name

			r := NewSlurmControl(newSlurmClientMap(controllerName, tt.client))

			got, gotErr := r.CheckReservationForNodeSet(context.Background(), tt.nodeset)
			if tt.wantErr {
				require.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_realSlurmControl_GetPodsUnderReservation(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	kclient := kubefake.NewFakeClient()

	// Configure times for testing
	future, err := time.Parse(time.RFC3339, "2099-03-04T00:00:00Z")
	require.NoError(t, err)
	startTimeFuture := future.Unix()
	endTimeFuture := future.Add(time.Hour).Unix()

	now := time.Now()
	startTimeNow := now.Unix()
	endTimeNow := now.Add(time.Hour).Unix()

	// Configure NodeSets and pods for testing
	ns0 := newNodeSet("ns0", controller.Name, 2)
	ns0.Spec.UpdateStrategy.Type = slinkyv1beta1.ScheduledUpdateNodeSetStrategyType
	ns0.Spec.UpdateStrategy.ScheduledUpdate.StartTime = metav1.Time{
		Time: future,
	}
	ns0reservationname := "SlurmOperatorMaint-" + ns0.Name
	ns0pod0 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns0, controller, 0, "")
	ns0pod1 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns0, controller, 1, "")
	ns0pod0name := nodesetutils.GetSlurmNodeName(ns0pod0)
	ns0pod1name := nodesetutils.GetSlurmNodeName(ns0pod1)

	ns1 := newNodeSet("ns1", controller.Name, 2)
	ns1.Spec.UpdateStrategy.Type = slinkyv1beta1.ScheduledUpdateNodeSetStrategyType
	ns1.Spec.UpdateStrategy.ScheduledUpdate.StartTime = metav1.Time{
		Time: now,
	}
	ns1reservationname := "SlurmOperatorMaint-" + ns1.Name
	ns1pod0 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns1, controller, 0, "")
	ns1pod1 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns1, controller, 1, "")
	ns1pod0name := nodesetutils.GetSlurmNodeName(ns1pod0)
	ns1pod1name := nodesetutils.GetSlurmNodeName(ns1pod1)

	nodeList := &types.V0044NodeList{
		Items: []types.V0044Node{
			{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
			{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
			{V0044Node: api.V0044Node{
				Name: ptr.To(ns1pod0name),
				State: ptr.To([]api.V0044NodeState{
					api.V0044NodeStateMAINTENANCE,
				}),
				Reservation: &ns1reservationname,
			}},
			{V0044Node: api.V0044Node{
				Name: ptr.To(ns1pod1name),
				State: ptr.To([]api.V0044NodeState{
					api.V0044NodeStateMAINTENANCE,
				}),
				Reservation: &ns1reservationname,
			}},
		},
	}

	type testCase struct {
		name    string
		client  client.Client
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
		want    []*corev1.Pod
		wantErr bool
	}
	tests := []testCase{
		{
			name:    "no pods under reservation",
			nodeset: ns0,
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To(ns0reservationname),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTimeFuture)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTimeFuture)}),
				},
			},
			).WithLists(nodeList).Build(),
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			want:    []*corev1.Pod{},
			wantErr: false,
		},
		{
			name:    "one pod under reservation",
			nodeset: ns1,
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(
				&types.V0044ReservationInfo{
					V0044ReservationInfo: api.V0044ReservationInfo{
						Name:      ptr.To(ns0reservationname),
						StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTimeFuture)}),
						EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTimeFuture)}),
					},
				},
				&types.V0044ReservationInfo{
					V0044ReservationInfo: api.V0044ReservationInfo{
						Name:      ptr.To(ns1reservationname),
						StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTimeNow)}),
						EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTimeNow)}),
					},
				},
			).WithLists(nodeList).Build(),
			pods: []*corev1.Pod{
				ns0pod0,
				ns1pod0,
			},
			want: []*corev1.Pod{
				ns1pod0,
			},
			wantErr: false,
		},
		{
			name:    "two pods under reservation",
			nodeset: ns1,
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(
				&types.V0044ReservationInfo{
					V0044ReservationInfo: api.V0044ReservationInfo{
						Name:      ptr.To(ns0reservationname),
						StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTimeFuture)}),
						EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTimeFuture)}),
					},
				},
				&types.V0044ReservationInfo{
					V0044ReservationInfo: api.V0044ReservationInfo{
						Name:      ptr.To(ns1reservationname),
						StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTimeNow)}),
						EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTimeNow)}),
					},
				},
			).WithLists(nodeList).Build(),
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
				ns1pod0,
				ns1pod1,
			},
			want: []*corev1.Pod{
				ns1pod0,
				ns1pod1,
			},
			wantErr: false,
		},
		{
			// ns0's reservation is intentionally not registered. ns1pod0's
			// node carries a non-nil Reservation pointing elsewhere; this
			// would previously panic when dereferenced against the unset
			// ns0 reservation name.
			name:    "reservation does not exist",
			nodeset: ns0,
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(
				&types.V0044ReservationInfo{
					V0044ReservationInfo: api.V0044ReservationInfo{
						Name:      ptr.To(ns1reservationname),
						StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTimeNow)}),
						EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTimeNow)}),
					},
				},
			).WithLists(nodeList).Build(),
			pods: []*corev1.Pod{
				ns1pod0,
			},
			want:    nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllerName := tt.nodeset.Spec.ControllerRef.Name

			r := NewSlurmControl(newSlurmClientMap(controllerName, tt.client))

			got, gotErr := r.GetPodsUnderReservation(context.Background(), tt.nodeset, tt.pods)
			if tt.wantErr {
				require.Error(t, gotErr)
				return
			}
			require.NoError(t, gotErr)
			require.True(t, apiequality.Semantic.DeepEqual(got, tt.want), "UpdateNodeWithPodInfo() got = %v, want %v", got, tt.want)
		})
	}
}

func Test_realSlurmControl_DeleteReservationForNodeSet(t *testing.T) {
	// Configure times for testing
	now, err := time.Parse(time.RFC3339, "2026-03-04T00:00:00Z")
	require.NoError(t, err)
	startTime := now.Unix()
	endTime := now.Add(time.Hour).Unix()

	type testCase struct {
		name    string
		client  client.Client
		nodeset *slinkyv1beta1.NodeSet
		wantErr bool
	}
	tests := []testCase{
		{
			name: "invalid NodeSet (no controller ref)",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slinky",
				},
			},
			client:  fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).Build(),
			wantErr: false,
		},
		{
			name: "reservation does not exist",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slinky",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
				},
			},
			client:  fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).Build(),
			wantErr: false,
		},
		{
			name: "reservation exists with default name, status not provided (lookup by full name)",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slinky",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
				},
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTime)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTime)}),
				},
			},
			).Build(),
			wantErr: false,
		},
		{
			name: "reservation exists with custom name, status not provided (lookup by prefix)",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slinky",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
				},
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky-customName"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(startTime)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(endTime)}),
				},
			},
			).Build(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllerName := tt.nodeset.Spec.ControllerRef.Name

			r := NewSlurmControl(newSlurmClientMap(controllerName, tt.client))

			gotErr := r.DeleteReservationForNodeSet(context.Background(), tt.nodeset)
			if tt.wantErr {
				require.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func Test_realSlurmControl_SyncReservationForNodeSet(t *testing.T) {
	// Configure times for testing
	now := time.Now()

	currentStartTimeMeta := metav1.Time{Time: now}
	currentStartTime := now.Unix()
	currentEndTime := now.Add(time.Hour).Unix()

	duration := metav1.Duration{Duration: 45 * time.Minute}

	futureStartTimeMeta := metav1.Time{Time: now.Add(24 * time.Hour)}
	futureStartTime := now.Add(24 * time.Hour).Unix()
	futureEndTime := now.Add(24 * time.Hour).Unix()

	pastStartTimeMeta := metav1.Time{Time: now.Add(-24 * time.Hour)}
	pastStartTime := now.Add(-24 * time.Hour).Unix()
	pastEndTime := now.Add(-24 * time.Hour).Unix()

	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	kclient := kubefake.NewFakeClient()
	ns0 := newNodeSet("ns0", controller.Name, 2)
	ns0pod0 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns0, controller, 0, "")
	ns0pod1 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns0, controller, 1, "")
	ns0pod2 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns0, controller, 2, "")
	ns0pod0name := nodesetutils.GetSlurmNodeName(ns0pod0)
	ns0pod1name := nodesetutils.GetSlurmNodeName(ns0pod1)
	ns0pod2name := nodesetutils.GetSlurmNodeName(ns0pod2)
	ns1 := newNodeSet("ns1", controller.Name, 2)
	ns1pod0 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns1, controller, 0, "")
	ns1pod1 := nodesetutils.NewNodeSetStatefulSetPod(kclient, ns1, controller, 1, "")
	ns1pod0name := nodesetutils.GetSlurmNodeName(ns1pod0)
	ns1pod1name := nodesetutils.GetSlurmNodeName(ns1pod1)

	type testCase struct {
		name    string
		client  client.Client
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
		wantErr bool
	}
	tests := []testCase{
		{
			name: "invalid NodeSet (no controller ref)",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
			},
			client:  fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).Build(),
			wantErr: false,
		},
		{
			name: "reservation spec not provided",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
					},
				},
			},
			client:  fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).Build(),
			wantErr: false,
		},
		{
			name: "create reservation with basic spec",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: futureStartTimeMeta,
							Duration:  duration,
							Flags:     []string{"weekly", "FORCE_START"},
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).Build(),
			wantErr: false,
		},
		{
			name: "create reservation fails when Slurm Create returns error",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: futureStartTimeMeta,
							Duration:  duration,
							Flags:     []string{"weekly", "FORCE_START"},
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(context.Context, object.Object, any, ...client.CreateOption) error {
					return errors.New("Internal Server Error")
				},
			}).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).Build(),
			wantErr: true,
		},
		{
			name: "create reservation with complex spec",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: futureStartTimeMeta,
							Duration:  duration,
							Flags:     []string{"weekly", "FORCE_START", "invalid_flag", "USER_DELETE"},
						},
					},
				},
			},
			client:  fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).Build(),
			wantErr: false,
		},
		{
			name: "update reservation with simple spec",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: futureStartTimeMeta,
							Duration:  duration,
							Flags:     []string{"weekly", "FORCE_START"},
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(futureStartTime), Set: ptr.To(true)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(futureEndTime), Set: ptr.To(true)}),
				},
			},
			).Build(),
			wantErr: false,
		},
		{
			name: "update reservation fails when Slurm Update returns error",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: futureStartTimeMeta,
							Duration:  duration,
							Flags:     []string{"weekly", "FORCE_START"},
						},
					},
				},
				Status: slinkyv1beta1.NodeSetStatus{
					Conditions: []metav1.Condition{
						{
							Type:               slurmconditions.NodeSetConditionReservationCreated,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: futureStartTimeMeta,
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithInterceptorFuncs(interceptor.Funcs{
				Update: func(context.Context, object.Object, any, ...client.UpdateOption) error {
					return errors.New("Internal Server Error")
				},
			}).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(futureStartTime), Set: ptr.To(true)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(futureEndTime), Set: ptr.To(true)}),
				},
			},
			).Build(),
			wantErr: true,
		},
		{
			name: "update reservation with complex spec",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: futureStartTimeMeta,
							Duration:  duration,
							Flags: []string{
								"USER_DELETE",
								"WEEKLY",
								"FORCE_START",
							},
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(futureStartTime), Set: ptr.To(true)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(futureEndTime), Set: ptr.To(true)}),
				},
			},
			).Build(),
			wantErr: false,
		},
		{
			name: "update reservation with past start time in Slurm",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: pastStartTimeMeta,
							Duration:  duration,
							Flags: []string{
								"USER_DELETE",
								"WEEKLY",
								"FORCE_START",
							},
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(pastStartTime), Set: ptr.To(true)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(pastEndTime), Set: ptr.To(true)}),
				},
			},
			).Build(),
			wantErr: false,
		},
		{
			name: "update reservation with past start time in Slurm fails without reoccuring flag",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: pastStartTimeMeta,
							Duration:  duration,
							Flags: []string{
								"USER_DELETE",
							},
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(currentStartTime), Set: ptr.To(true)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(currentEndTime), Set: ptr.To(true)}),
				},
			},
			).Build(),
			wantErr: false,
		},
		{
			name: "update active reservation does not occur",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: currentStartTimeMeta,
							Duration:  duration,
							Flags:     []string{"weekly", "FORCE_START"},
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithInterceptorFuncs(interceptor.Funcs{
				Update: func(context.Context, object.Object, any, ...client.UpdateOption) error {
					// We can tell that Update didn't get called because an error was not returned here
					return errors.New("Internal Server Error")
				},
			}).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(currentStartTime), Set: ptr.To(true)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(currentEndTime), Set: ptr.To(true)}),
					NodeList:  ptr.To(ns0pod0name + "," + ns0pod1name),
				},
			},
			).Build(),
			wantErr: false,
		},
		{
			name: "active reservation nodes are updated",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slinky",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					ControllerRef: corev1.LocalObjectReference{
						Name: "slurm",
					},
					UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
						Type: slinkyv1beta1.ScheduledUpdateNodeSetStrategyType,
						ScheduledUpdate: slinkyv1beta1.ScheduledUpdateNodeSetStrategy{
							StartTime: currentStartTimeMeta,
							Duration:  duration,
							Flags: []string{
								"USER_DELETE",
								"WEEKLY",
								"FORCE_START",
							},
						},
					},
				},
			},
			pods: []*corev1.Pod{
				ns0pod0,
				ns0pod1,
				ns0pod2,
			},
			client: fake.NewClientBuilder().WithUpdateFn(slurmUpdateFn).WithLists(
				&types.V0044NodeList{
					Items: []types.V0044Node{
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod1name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns0pod2name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod0name)}},
						{V0044Node: api.V0044Node{Name: ptr.To(ns1pod1name)}},
					},
				},
			).WithObjects(&types.V0044ReservationInfo{
				V0044ReservationInfo: api.V0044ReservationInfo{
					Name:      ptr.To("SlurmOperatorMaint-slinky"),
					StartTime: ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(currentStartTime), Set: ptr.To(true)}),
					EndTime:   ptr.To(api.V0044Uint64NoValStruct{Number: ptr.To(currentEndTime), Set: ptr.To(true)}),
					NodeList:  ptr.To(ns0pod0name + "," + ns0pod1name),
				},
			},
			).Build(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controllerName := tt.nodeset.Spec.ControllerRef.Name

			r := NewSlurmControl(newSlurmClientMap(controllerName, tt.client))

			gotErr := r.SyncReservationForNodeSet(context.Background(), tt.nodeset, tt.pods)
			if tt.wantErr {
				require.Error(t, gotErr)
			} else {
				require.NoError(t, gotErr)
			}
		})
	}
}

func Test_tolerateError(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Nil",
			args: args{
				err: nil,
			},
			want: true,
		},
		{
			name: "Empty",
			args: args{
				err: errors.New(""),
			},
			want: false,
		},
		{
			name: "NotFound",
			args: args{
				err: errors.New(http.StatusText(http.StatusNotFound)),
			},
			want: true,
		},
		{
			name: "NoContent",
			args: args{
				err: errors.New(http.StatusText(http.StatusNoContent)),
			},
			want: true,
		},
		{
			name: "Forbidden",
			args: args{
				err: errors.New(http.StatusText(http.StatusForbidden)),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tolerateError(tt.args.err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_nodeState(t *testing.T) {
	type args struct {
		node  types.V0044Node
		state corev1.PodConditionType
	}
	tests := []struct {
		name string
		args args
		want corev1.PodCondition
	}{
		{
			name: "Idle state",
			args: args{
				node: types.V0044Node{
					V0044Node: api.V0044Node{
						Reason: ptr.To(""),
					},
				},
				state: slurmconditions.PodConditionIdle,
			},
			want: corev1.PodCondition{
				Type:    slurmconditions.PodConditionIdle,
				Status:  corev1.ConditionTrue,
				Message: "",
			},
		},
		{
			name: "Drain state",
			args: args{
				node: types.V0044Node{
					V0044Node: api.V0044Node{
						Reason: ptr.To("Drain by admin"),
					},
				},
				state: slurmconditions.PodConditionDrain,
			},
			want: corev1.PodCondition{
				Type:    slurmconditions.PodConditionDrain,
				Status:  corev1.ConditionTrue,
				Message: "Drain by admin",
			},
		},
		{
			name: "InvalidReg state",
			args: args{
				node: types.V0044Node{
					V0044Node: api.V0044Node{
						Reason: ptr.To(""),
					},
				},
				state: slurmconditions.PodConditionInvalidReg,
			},
			want: corev1.PodCondition{
				Type:    slurmconditions.PodConditionInvalidReg,
				Status:  corev1.ConditionTrue,
				Message: "",
			},
		},
		{
			name: "Maintenance state",
			args: args{
				node: types.V0044Node{
					V0044Node: api.V0044Node{
						Reason: ptr.To("Admin set to Maintenance"),
					},
				},
				state: slurmconditions.PodConditionMaintenance,
			},
			want: corev1.PodCondition{
				Type:    slurmconditions.PodConditionMaintenance,
				Status:  corev1.ConditionTrue,
				Message: "Admin set to Maintenance",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeState(tt.args.node, tt.args.state)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_realSlurmControl_GetDefunctNodesForNodeSet(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	nodeset.UID = k8stypes.UID("foo-uid")
	otherNodeSet := newNodeSet("bar", controller.Name, 1)
	otherNodeSet.UID = k8stypes.UID("bar-uid")
	defunctPodName := nodesetutils.GetOrdinalPodName(nodeset, 7)
	podInfo := func(nodeset *slinkyv1beta1.NodeSet, podName, node string) *string {
		return new((&podinfo.PodInfo{
			Namespace:   corev1.NamespaceDefault,
			PodName:     podName,
			Node:        node,
			NodeSetName: nodeset.Name,
			NodeSetUID:  string(nodeset.UID),
		}).ToString())
	}

	type fields struct {
		nodes []types.V0044Node
	}
	tests := []struct {
		name    string
		fields  fields
		want    []DefunctNode
		wantOk  bool
		wantErr bool
	}{
		{
			name: "returns only down and not responding nodes with PodInfo from this nodeset",
			fields: fields{
				nodes: []types.V0044Node{
					{
						V0044Node: api.V0044Node{
							Name: ptr.To("foo-ghost"),
							State: ptr.To([]api.V0044NodeState{
								api.V0044NodeStateDOWN,
								api.V0044NodeStateNOTRESPONDING,
							}),
							Comment: podInfo(nodeset, defunctPodName, "worker-a"),
						},
					},
					{
						V0044Node: api.V0044Node{
							Name: ptr.To("foo-missing-flag"),
							State: ptr.To([]api.V0044NodeState{
								api.V0044NodeStateDOWN,
							}),
							Comment: podInfo(nodeset, nodesetutils.GetOrdinalPodName(nodeset, 8), ""),
						},
					},
					{
						V0044Node: api.V0044Node{
							Name: ptr.To("bar-ghost"),
							State: ptr.To([]api.V0044NodeState{
								api.V0044NodeStateDOWN,
								api.V0044NodeStateNOTRESPONDING,
							}),
							Comment: podInfo(otherNodeSet, nodesetutils.GetOrdinalPodName(otherNodeSet, 0), ""),
						},
					},
					{
						V0044Node: api.V0044Node{
							Name: ptr.To("foo-no-comment"),
							State: ptr.To([]api.V0044NodeState{
								api.V0044NodeStateDOWN,
								api.V0044NodeStateNOTRESPONDING,
							}),
						},
					},
				},
			},
			want: []DefunctNode{
				{
					Name: "foo-ghost",
					PodInfo: podinfo.PodInfo{
						Namespace:   corev1.NamespaceDefault,
						PodName:     defunctPodName,
						Node:        "worker-a",
						NodeSetName: nodeset.Name,
						NodeSetUID:  string(nodeset.UID),
					},
				},
			},
			wantOk: true,
		},
		{
			name:   "no client",
			fields: fields{},
			wantOk: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var clientMap *clientmap.ClientMap
			if tt.wantOk {
				objects := make([]object.Object, 0, len(tt.fields.nodes))
				for i := range tt.fields.nodes {
					node := tt.fields.nodes[i]
					objects = append(objects, &node)
				}
				sclient := fake.NewClientBuilder().WithObjects(objects...).Build()
				clientMap = newSlurmClientMap(controller.Name, sclient)
			} else {
				clientMap = clientmap.NewClientMap()
			}

			r := &realSlurmControl{clientMap: clientMap}
			got, ok, err := r.GetDefunctNodesForNodeSet(ctx, nodeset)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantOk, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_realSlurmControl_DeleteNode(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	node := &types.V0044Node{
		V0044Node: api.V0044Node{
			Name: ptr.To("foo-0"),
			State: ptr.To([]api.V0044NodeState{
				api.V0044NodeStateDOWN,
				api.V0044NodeStateNOTRESPONDING,
			}),
		},
	}
	sclient := fake.NewClientBuilder().WithObjects(node).Build()
	r := &realSlurmControl{clientMap: newSlurmClientMap(controller.Name, sclient)}

	err := r.DeleteNode(ctx, nodeset, "foo-0")
	require.NoError(t, err)

	checkNode := &types.V0044Node{}
	getErr := sclient.Get(ctx, node.GetKey(), checkNode)
	require.True(t, tolerateError(getErr), "DeleteNode() node still exists: %v", getErr)
}
