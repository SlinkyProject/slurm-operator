// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/controller/history"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	slurminterceptor "github.com/SlinkyProject/slurm-client/pkg/client/interceptor"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"
	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/slurmcontrol"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
)

func TestNodeSetReconciler_syncStatus(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	const hash = "12345"
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx             context.Context
		nodeset         *slinkyv1beta1.NodeSet
		pods            []*corev1.Pod
		currentRevision *appsv1.ControllerRevision
		updateRevision  *appsv1.ControllerRevision
		collisionCount  int32
		hash            string
		errors          []error
	}
	type testCaseFields struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}
	tests := []testCaseFields{
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := make([]*corev1.Pod, 0)
			for i := range 2 {
				pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, hash)
				pod = makePodHealthy(pod)
				pods = append(pods, pod)
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			revision := &appsv1.ControllerRevision{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						history.ControllerRevisionHashLabel: hash,
					},
				},
			}
			c := fake.NewClientBuilder().WithRuntimeObjects(nodeset, podList, revision).WithStatusSubresource(nodeset).Build()
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: func(pods []*corev1.Pod) []slurmtypes.V0044Node {
					nodeList := make([]slurmtypes.V0044Node, 0, len(pods))
					for _, pod := range pods {
						slurmNode := newNodeSetPodSlurmNode(pod)
						nodeList = append(nodeList, *slurmNode)
					}
					return nodeList
				}(pods),
			}
			sc := newFakeClientList(slurminterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, sc)

			return testCaseFields{
				name: "Healthy, up-to-date",
				fields: fields{
					Client:    c,
					ClientMap: clientMap,
				},
				args: args{
					ctx:             context.TODO(),
					nodeset:         nodeset,
					pods:            pods,
					currentRevision: revision,
					updateRevision:  revision,
					collisionCount:  0,
					hash:            hash,
				},
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := make([]*corev1.Pod, 0)
			for i := range 2 {
				pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, hash)
				pod = makePodCreated(pod)
				pods = append(pods, pod)
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			revision := &appsv1.ControllerRevision{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						history.ControllerRevisionHashLabel: hash,
					},
				},
			}
			c := fake.NewClientBuilder().WithRuntimeObjects(nodeset, podList, revision).WithStatusSubresource(nodeset).Build()
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: func(pods []*corev1.Pod) []slurmtypes.V0044Node {
					nodeList := make([]slurmtypes.V0044Node, 0, len(pods))
					for _, pod := range pods {
						slurmNode := newNodeSetPodSlurmNode(pod)
						nodeList = append(nodeList, *slurmNode)
					}
					return nodeList
				}(pods),
			}
			sc := newFakeClientList(slurminterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, sc)

			return testCaseFields{
				name: "Created, need update",
				fields: fields{
					Client:    c,
					ClientMap: clientMap,
				},
				args: args{
					ctx:             context.TODO(),
					nodeset:         nodeset,
					pods:            pods,
					currentRevision: revision,
					updateRevision:  &appsv1.ControllerRevision{},
					collisionCount:  0,
					hash:            hash,
				},
				wantErr: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.syncStatus(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.currentRevision, tt.args.updateRevision, tt.args.collisionCount, tt.args.hash, tt.args.errors...); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.syncStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_syncSlurmStatus(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
	}
	type testCaseFields struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}
	tests := []testCaseFields{
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := make([]*corev1.Pod, 0)
			for i := range 2 {
				pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, "")
				pod = makePodHealthy(pod)
				pods = append(pods, pod)
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			c := fake.NewClientBuilder().WithRuntimeObjects(nodeset, podList).WithStatusSubresource(nodeset).Build()
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: func(pods []*corev1.Pod) []slurmtypes.V0044Node {
					nodeList := make([]slurmtypes.V0044Node, 0, len(pods))
					for _, pod := range pods {
						slurmNode := newNodeSetPodSlurmNode(pod)
						nodeList = append(nodeList, *slurmNode)
					}
					return nodeList
				}(pods),
			}
			sc := newFakeClientList(slurminterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, sc)

			return testCaseFields{
				name: "Healthy pods",
				fields: fields{
					Client:    c,
					ClientMap: clientMap,
				},
				args: args{
					ctx:     context.TODO(),
					nodeset: nodeset,
					pods:    pods,
				},
				wantErr: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.syncSlurmStatus(tt.args.ctx, tt.args.nodeset, tt.args.pods); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.syncSlurmStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_syncNodeSetStatus(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	const hash = "12345"
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx             context.Context
		nodeset         *slinkyv1beta1.NodeSet
		pods            []*corev1.Pod
		currentRevision *appsv1.ControllerRevision
		updateRevision  *appsv1.ControllerRevision
		collisionCount  int32
		hash            string
	}
	type testCaseFields struct {
		name       string
		fields     fields
		args       args
		wantStatus *slinkyv1beta1.NodeSetStatus
		wantErr    bool
	}
	tests := []testCaseFields{
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := make([]*corev1.Pod, 0)
			for i := range 2 {
				pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, hash)
				pod = makePodHealthy(pod)
				pods = append(pods, pod)
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			revision := &appsv1.ControllerRevision{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						history.ControllerRevisionHashLabel: hash,
					},
				},
			}
			c := fake.NewClientBuilder().WithRuntimeObjects(nodeset, podList, revision).WithStatusSubresource(nodeset).Build()
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: func(pods []*corev1.Pod) []slurmtypes.V0044Node {
					nodeList := make([]slurmtypes.V0044Node, 0, len(pods))
					for _, pod := range pods {
						slurmNode := newNodeSetPodSlurmNode(pod)
						nodeList = append(nodeList, *slurmNode)
					}
					return nodeList
				}(pods),
			}
			sc := newFakeClientList(slurminterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, sc)

			return testCaseFields{
				name: "Healthy, up-to-date",
				fields: fields{
					Client:    c,
					ClientMap: clientMap,
				},
				args: args{
					ctx:             context.TODO(),
					nodeset:         nodeset,
					pods:            pods,
					currentRevision: revision,
					updateRevision:  revision,
					collisionCount:  0,
					hash:            hash,
				},
				wantStatus: &slinkyv1beta1.NodeSetStatus{
					Replicas:          2,
					ReadyReplicas:     2,
					AvailableReplicas: 2,
					UpdatedReplicas:   2,
					SlurmIdle:         2,
					NodeSetHash:       "12345",
					CollisionCount:    ptr.To[int32](0),
					Selector:          "app.kubernetes.io/instance=foo,app.kubernetes.io/name=slurmd",
				},
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := make([]*corev1.Pod, 0)
			for i := range 2 {
				pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, hash)
				pod = makePodCreated(pod)
				pods = append(pods, pod)
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			revision := &appsv1.ControllerRevision{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						history.ControllerRevisionHashLabel: hash,
					},
				},
			}
			c := fake.NewClientBuilder().WithRuntimeObjects(nodeset, podList, revision).WithStatusSubresource(nodeset).Build()
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: func(pods []*corev1.Pod) []slurmtypes.V0044Node {
					nodeList := make([]slurmtypes.V0044Node, 0, len(pods))
					for _, pod := range pods {
						slurmNode := newNodeSetPodSlurmNode(pod)
						nodeList = append(nodeList, *slurmNode)
					}
					return nodeList
				}(pods),
			}
			sc := newFakeClientList(slurminterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, sc)

			return testCaseFields{
				name: "Created, need update",
				fields: fields{
					Client:    c,
					ClientMap: clientMap,
				},
				args: args{
					ctx:             context.TODO(),
					nodeset:         nodeset,
					pods:            pods,
					currentRevision: revision,
					updateRevision:  &appsv1.ControllerRevision{},
					collisionCount:  0,
					hash:            hash,
				},
				wantStatus: &slinkyv1beta1.NodeSetStatus{
					Replicas:            2,
					UnavailableReplicas: 2,
					NodeSetHash:         "12345",
					CollisionCount:      ptr.To[int32](0),
					Selector:            "app.kubernetes.io/instance=foo,app.kubernetes.io/name=slurmd",
				},
				wantErr: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.syncNodeSetStatus(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.currentRevision, tt.args.updateRevision, tt.args.collisionCount, tt.args.hash); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.syncNodeSetStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := &slinkyv1beta1.NodeSet{}
			key := client.ObjectKeyFromObject(tt.args.nodeset)
			if err := r.Get(tt.args.ctx, key, got); err == nil {
				if diff := cmp.Diff(tt.wantStatus, &got.Status); diff != "" {
					t.Errorf("unexpected status (-want,+got):\n%s", diff)
				}
			}
		})
	}
}

func TestNodeSetReconciler_calculateReplicaStatus(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	const hash = "12345"
	type args struct {
		nodeset         *slinkyv1beta1.NodeSet
		pods            []*corev1.Pod
		currentRevision *appsv1.ControllerRevision
		updateRevision  *appsv1.ControllerRevision
	}
	tests := []struct {
		name string
		args args
		want replicaStatus
	}{
		{
			name: "Empty",
			args: args{},
			want: replicaStatus{},
		},
		{
			name: "Healthy, up-to-date",
			args: func() args {
				nodeset := newNodeSet("foo", controller.Name, 2)
				pods := make([]*corev1.Pod, 0)
				for i := range 2 {
					pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, hash)
					pod = makePodHealthy(pod)
					pods = append(pods, pod)
				}
				revision := &appsv1.ControllerRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							history.ControllerRevisionHashLabel: hash,
						},
					},
				}
				return args{
					nodeset:         nodeset,
					pods:            pods,
					currentRevision: revision,
					updateRevision:  revision,
				}
			}(),
			want: replicaStatus{
				Replicas:  2,
				Available: 2,
				Ready:     2,
				Current:   2,
				Updated:   2,
			},
		},
		{
			name: "Created, need update",
			args: func() args {
				nodeset := newNodeSet("foo", controller.Name, 2)
				pods := make([]*corev1.Pod, 0)
				for i := range 2 {
					pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, hash)
					pod = makePodCreated(pod)
					pods = append(pods, pod)
				}
				revision := &appsv1.ControllerRevision{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							history.ControllerRevisionHashLabel: hash,
						},
					},
				}
				return args{
					nodeset:         nodeset,
					pods:            pods,
					currentRevision: revision,
					updateRevision:  &appsv1.ControllerRevision{},
				}
			}(),
			want: replicaStatus{
				Replicas:    2,
				Unavailable: 2,
				Current:     2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(fake.NewFakeClient(), nil)
			got := r.calculateReplicaStatus(tt.args.nodeset, tt.args.pods, tt.args.currentRevision, tt.args.updateRevision)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("unexpected status (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestNodeSetReconciler_updateNodeSetPodConditions(t *testing.T) {
	idleCondition := corev1.PodCondition{
		Type:    slurmconditions.PodConditionIdle,
		Status:  corev1.ConditionTrue,
		Message: "",
	}
	drainCondition := corev1.PodCondition{
		Type:    slurmconditions.PodConditionDrain,
		Status:  corev1.ConditionTrue,
		Message: "Node set to drain",
	}
	allocatedCondition := corev1.PodCondition{
		Type:    slurmconditions.PodConditionAllocated,
		Status:  corev1.ConditionTrue,
		Message: "",
	}

	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	const hash = "12345"
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx        context.Context
		pods       []*corev1.Pod
		nodeStatus *slurmcontrol.SlurmNodeStatus
	}
	type testCaseFields struct {
		name    string
		fields  fields
		args    args
		wantErr error
	}
	tests := []testCaseFields{
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := make([]*corev1.Pod, 0)
			for i := range 2 {
				pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, hash)
				pod = makePodHealthy(pod)
				pod.Status.Conditions = append(pod.Status.Conditions, idleCondition)
				pods = append(pods, pod)
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			c := fake.NewClientBuilder().WithRuntimeObjects(nodeset, podList).WithStatusSubresource(nodeset).Build()

			return testCaseFields{
				name: "Slurm States remains Idle",
				fields: fields{
					Client: c,
				},
				args: args{
					ctx:  context.TODO(),
					pods: pods,
					nodeStatus: &slurmcontrol.SlurmNodeStatus{
						NodeStates: func(pods []*corev1.Pod) map[string][]corev1.PodCondition {
							ns := make(map[string][]corev1.PodCondition)
							for _, pod := range pods {
								ns[pod.Name] = append(ns[pod.Name], idleCondition)
							}
							return ns
						}(pods),
					},
				},
				wantErr: nil,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := make([]*corev1.Pod, 0)
			for i := range 2 {
				pod := nodesetutils.NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, i, hash)
				pod = makePodHealthy(pod)
				pod.Status.Conditions = append(pod.Status.Conditions, allocatedCondition)
				pods = append(pods, pod)
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			c := fake.NewClientBuilder().WithRuntimeObjects(nodeset, podList).WithStatusSubresource(nodeset).Build()

			return testCaseFields{
				name: "Slurm States transition from Allocated to Idle+Drain",
				fields: fields{
					Client: c,
				},
				args: args{
					ctx:  context.TODO(),
					pods: pods,
					nodeStatus: &slurmcontrol.SlurmNodeStatus{
						NodeStates: func(pods []*corev1.Pod) map[string][]corev1.PodCondition {
							ns := make(map[string][]corev1.PodCondition)
							for _, pod := range pods {
								ns[pod.Name] = append(ns[pod.Name], idleCondition)
								ns[pod.Name] = append(ns[pod.Name], drainCondition)
							}
							return ns
						}(pods),
					},
				},
				wantErr: nil,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &NodeSetReconciler{
				Client: tt.fields.Client,
			}
			err := r.updateNodeSetPodConditions(tt.args.ctx, tt.args.pods, tt.args.nodeStatus)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NodeSetReconciler.updateNodeSetPodConditions() error = %v, wantErr %v", err, tt.wantErr)
			}
			for key, ns := range tt.args.nodeStatus.NodeStates {
				// Verify the correct conditions are present in the correct pod
				pod := &corev1.Pod{}
				err = r.Get(tt.args.ctx, client.ObjectKey{Name: key, Namespace: "default"}, pod)
				if err != nil {
					t.Errorf("NodeSetReconciler.updateNodeSetPodConditions() error = %v", err)
					return
				}
				for _, condition := range pod.Status.Conditions {
					if strings.HasPrefix(string(condition.Type), slurmconditions.StatePrefix) {
						var found bool
						for _, nodeCondition := range ns {
							if condition.Type == nodeCondition.Type &&
								condition.Message == nodeCondition.Message {
								found = true
							}
						}
						if !found {
							t.Errorf(`NodeSetReconciler.updateNodeSetPodConditions() could not find a pod (%v) condition
							as a Slurm node state (%v)`, condition, ns)
						}
					}
				}
			}
		})
	}
}

func TestNodeSetReconciler_updateNodeSetStatus(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 2)
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx       context.Context
		nodeset   *slinkyv1beta1.NodeSet
		newStatus *slinkyv1beta1.NodeSetStatus
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Success",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithRuntimeObjects(nodeset).
					WithStatusSubresource(nodeset).
					Build(),
			},
			args: args{
				ctx:       context.TODO(),
				nodeset:   nodeset,
				newStatus: &slinkyv1beta1.NodeSetStatus{},
			},
			wantErr: false,
		},
		{
			name: "NotFound",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				ctx:       context.TODO(),
				nodeset:   nodeset,
				newStatus: &slinkyv1beta1.NodeSetStatus{},
			},
			wantErr: false,
		},
		{
			name: "Error",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithRuntimeObjects(nodeset).
					WithStatusSubresource(nodeset).
					WithInterceptorFuncs(interceptor.Funcs{
						SubResourceUpdate: func(ctx context.Context, client client.Client, subResourceName string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
							return errors.New("failed to update resource status")
						},
					}).
					Build(),
			},
			args: args{
				ctx:       context.TODO(),
				nodeset:   nodeset,
				newStatus: &slinkyv1beta1.NodeSetStatus{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			if err := r.updateNodeSetStatus(tt.args.ctx, tt.args.nodeset, tt.args.newStatus); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.updateNodeSetStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := &slinkyv1beta1.NodeSet{}
			key := client.ObjectKeyFromObject(tt.args.nodeset)
			if err := r.Get(tt.args.ctx, key, got); err == nil {
				if diff := cmp.Diff(tt.args.newStatus, &got.Status); diff != "" {
					t.Errorf("unexpected status (-want,+got):\n%s", diff)
				}
			}
		})
	}
}

func Test_calculateNodeAssignments(t *testing.T) {
	now := metav1.Now()
	recentTime := metav1.NewTime(now.Add(-30 * time.Second))
	expiredTime := metav1.NewTime(now.Add(-600 * time.Second))

	makeNodeSet := func(name string, replicas int32, lockNodes bool, lifetime int32, existing map[string]slinkyv1beta1.NodeAssignment) *slinkyv1beta1.NodeSet {
		ns := newNodeSet(name, "slurm", replicas)
		ns.Spec.LockNodes = lockNodes
		ns.Spec.LockNodeLifetime = lifetime
		ns.Status.NodeAssignments = existing
		return ns
	}

	// Scheduled pod without a phase -- counts for new assignment recording but
	// does NOT trigger a timestamp refresh (pod is not Running yet).
	makePod := func(name, nodeName string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       corev1.PodSpec{NodeName: nodeName},
		}
	}

	// Running pod -- triggers timestamp refresh on existing assignments.
	makeRunningPod := func(name, nodeName string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       corev1.PodSpec{NodeName: nodeName},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		}
	}

	type args struct {
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
		now     metav1.Time
	}
	tests := []struct {
		name string
		args args
		want map[string]slinkyv1beta1.NodeAssignment
	}{
		{
			name: "lockNodes disabled returns nil",
			args: args{
				nodeset: makeNodeSet("foo", 2, false, 0, nil),
				pods:    []*corev1.Pod{makePod("foo-0", "node-1")},
				now:     now,
			},
			want: nil,
		},
		{
			name: "lockNodes enabled, new pods get recorded",
			args: args{
				nodeset: makeNodeSet("foo", 2, true, 0, nil),
				pods: []*corev1.Pod{
					makePod("foo-0", "node-1"),
					makePod("foo-1", "node-2"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: now},
				"foo-1": {NodeName: "node-2", AssignedAt: now},
			},
		},
		{
			name: "lockNodes enabled, preserves existing assignment when pod not running",
			args: args{
				nodeset: makeNodeSet("foo", 2, true, 0, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
				}),
				pods: []*corev1.Pod{
					makePod("foo-1", "node-2"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
				"foo-1": {NodeName: "node-2", AssignedAt: now},
			},
		},
		{
			name: "lockNodes enabled, existing assignment not overwritten by pod on different node",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 0, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
				}),
				pods: []*corev1.Pod{
					makeRunningPod("foo-0", "node-999"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
			},
		},
		{
			name: "lockNodes enabled, scale-down prunes stale assignments",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 0, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
					"foo-1": {NodeName: "node-2", AssignedAt: recentTime},
				}),
				pods: []*corev1.Pod{
					makePod("foo-0", "node-1"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
			},
		},
		{
			name: "lockNodes enabled, pod without NodeName skipped",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 0, nil),
				pods: []*corev1.Pod{
					makePod("foo-0", ""),
				},
				now: now,
			},
			want: nil,
		},
		{
			name: "lifetime=0 means permanent, old assignment preserved",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 0, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: expiredTime},
				}),
				pods: []*corev1.Pod{},
				now:  now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: expiredTime},
			},
		},
		{
			name: "lifetime>0, recent assignment preserved",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 300, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
				}),
				pods: []*corev1.Pod{},
				now:  now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
			},
		},
		{
			name: "lifetime>0, expired assignment pruned",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 300, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: expiredTime},
				}),
				pods: []*corev1.Pod{},
				now:  now,
			},
			want: nil,
		},
		{
			name: "lifetime>0, expired assignment pruned then pod re-recorded",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 300, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: expiredTime},
				}),
				pods: []*corev1.Pod{
					makePod("foo-0", "node-2"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-2", AssignedAt: now},
			},
		},
		{
			name: "running pod on locked node refreshes timestamp",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 300, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
				}),
				pods: []*corev1.Pod{
					makeRunningPod("foo-0", "node-1"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: now},
			},
		},
		{
			name: "non-running pod on locked node does NOT refresh timestamp",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 300, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
				}),
				pods: []*corev1.Pod{
					makePod("foo-0", "node-1"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
			},
		},
		{
			name: "running pod on different node does NOT refresh timestamp",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 300, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
				}),
				pods: []*corev1.Pod{
					makeRunningPod("foo-0", "node-999"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: recentTime},
			},
		},
		{
			name: "running pod prevents near-expiry assignment from being pruned",
			args: args{
				nodeset: makeNodeSet("foo", 1, true, 300, map[string]slinkyv1beta1.NodeAssignment{
					"foo-0": {NodeName: "node-1", AssignedAt: metav1.NewTime(now.Add(-299 * time.Second))},
				}),
				pods: []*corev1.Pod{
					makeRunningPod("foo-0", "node-1"),
				},
				now: now,
			},
			want: map[string]slinkyv1beta1.NodeAssignment{
				"foo-0": {NodeName: "node-1", AssignedAt: now},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateNodeAssignments(tt.args.nodeset, tt.args.pods, tt.args.now)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("calculateNodeAssignments() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
