// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	kubecontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/history"
	taints "k8s.io/kubernetes/pkg/util/taints"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	sinterceptor "github.com/SlinkyProject/slurm-client/pkg/client/interceptor"
	slurmobject "github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	builder "github.com/SlinkyProject/slurm-operator/internal/builder/workerbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/podcontrol"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/slurmcontrol"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/historycontrol"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podinfo"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
	slurmtaints "github.com/SlinkyProject/slurm-operator/pkg/taints"
)

func newNodeSetController(client client.Client, clientMap *clientmap.ClientMap) *NodeSetReconciler {
	eventRecorder := events.NewFakeRecorder(10)
	r := &NodeSetReconciler{
		Client:         client,
		Scheme:         client.Scheme(),
		ClientMap:      clientMap,
		builder:        builder.New(client),
		eventRecorder:  eventRecorder,
		historyControl: historycontrol.NewHistoryControl(client),
		podControl:     podcontrol.NewPodControl(client, eventRecorder),
		slurmControl:   slurmcontrol.NewSlurmControl(clientMap),
		expectations:   kubecontroller.NewUIDTrackingControllerExpectations(kubecontroller.NewControllerExpectations()),
	}
	return r
}

func newNodeSetControllerWithPropagatedNodeConditions(
	client client.Client,
	clientMap *clientmap.ClientMap,
	propagated []corev1.NodeConditionType,
) *NodeSetReconciler {
	r := newNodeSetController(client, clientMap)
	r.propagatedNodeConditions = propagated
	return r
}

func newNodeSet(name, controllerName string, replicas int32) *slinkyv1beta1.NodeSet {
	return &slinkyv1beta1.NodeSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      name,
		},
		Spec: slinkyv1beta1.NodeSetSpec{
			ControllerRef: slinkyv1beta1.ObjectReference{
				Namespace: corev1.NamespaceDefault,
				Name:      controllerName,
			},
			Replicas:    ptr.To(replicas),
			ScalingMode: slinkyv1beta1.ScalingModeStatefulset,
			Template: slinkyv1beta1.PodTemplate{
				Metadata: slinkyv1beta1.Metadata{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			Slurmd: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmd",
				},
			},
			ExtraConf: "Weight=10",
			LogFile: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "alpine",
				},
			},
		},
	}
}

func newNodeSetPodWithStatus(
	nodeset *slinkyv1beta1.NodeSet,
	controller *slinkyv1beta1.Controller,
	ordinal int,
	podPhase corev1.PodPhase,
	podConditions []corev1.PodConditionType,
) *corev1.Pod {
	pod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, ordinal, "")
	pod.Status.Phase = podPhase
	for _, condType := range podConditions {
		condition := corev1.PodCondition{
			Type:   condType,
			Status: corev1.ConditionTrue,
		}
		pod.Status.Conditions = append(pod.Status.Conditions, condition)
	}
	return pod
}

func newClientMap(controllerName string, client slurmclient.Client) *clientmap.ClientMap {
	cm := clientmap.NewClientMap()
	key := types.NamespacedName{
		Namespace: corev1.NamespaceDefault,
		Name:      controllerName,
	}
	cm.Add(key, client)
	return cm
}

func newDaemonPodForNodeSet(name, nodeName string, nodeset *slinkyv1beta1.NodeSet) *corev1.Pod {
	ns := corev1.NamespaceDefault
	if nodeset != nil {
		ns = nodeset.Namespace
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				slinkyv1beta1.LabelNodeSetPodHostname: nodeName,
				slinkyv1beta1.LabelNodeSetScalingMode: string(slinkyv1beta1.ScalingModeDaemonset),
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
	}
	if nodeset != nil {
		pod.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(nodeset, slinkyv1beta1.NodeSetGVK)}
	}
	return pod
}

func newNodeSetPodSlurmNode(pod *corev1.Pod) *slurmtypes.V0044Node {
	node := &slurmtypes.V0044Node{
		V0044Node: slurmapi.V0044Node{
			Name: ptr.To(pod.GetName()),
		},
	}
	switch {
	case podutils.IsPending(pod):
		node.State = nil
	default:
		node.State = ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE})
	}
	return node
}

func makePodCreated(pod *corev1.Pod) *corev1.Pod {
	pod.Status.Phase = corev1.PodPending
	return pod
}

func makePodHealthy(pod *corev1.Pod) *corev1.Pod {
	pod.Status.Phase = corev1.PodRunning
	podCond := corev1.PodCondition{
		Type:   corev1.PodReady,
		Status: corev1.ConditionTrue,
	}
	pod.Status.Conditions = append(pod.Status.Conditions, podCond)
	return pod
}

func TestNodeSetReconciler_adoptOrphanRevisions(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "No revisions",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: newNodeSet("foo", controller.Name, 2),
			},
			wantErr: false,
		},
		{
			name: "Adopt the revision",
			fields: fields{
				Client: fake.NewFakeClient(newNodeSet("foo", controller.Name, 2), &appsv1.ControllerRevision{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "foo-00000",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				}),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: newNodeSet("foo", controller.Name, 2),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			if err := r.adoptOrphanRevisions(tt.args.ctx, tt.args.nodeset); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.adoptOrphanRevisions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_doAdoptOrphanRevisions(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		nodeset   *slinkyv1beta1.NodeSet
		revisions []*appsv1.ControllerRevision
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "No revisions",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				nodeset:   newNodeSet("foo", controller.Name, 2),
				revisions: []*appsv1.ControllerRevision{},
			},
			wantErr: false,
		},
		{
			name: "Adopt revision",
			fields: fields{
				Client: fake.NewFakeClient(&appsv1.ControllerRevision{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "foo-00000",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				}),
			},
			args: args{
				nodeset: newNodeSet("foo", controller.Name, 2),
				revisions: []*appsv1.ControllerRevision{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: corev1.NamespaceDefault,
							Name:      "foo-00000",
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			if err := r.doAdoptOrphanRevisions(tt.args.nodeset, tt.args.revisions); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.doAdoptOrphanRevisions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_listRevisions(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		nodeset *slinkyv1beta1.NodeSet
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*appsv1.ControllerRevision
		wantErr bool
	}{
		{
			name: "Empty",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				nodeset: newNodeSet("foo", controller.Name, 2),
			},
			want:    []*appsv1.ControllerRevision{},
			wantErr: false,
		},
		{
			name: "Has revisions",
			fields: fields{
				Client: fake.NewFakeClient(&appsv1.ControllerRevision{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "foo-00000",
						Labels:    labels.NewBuilder().WithWorkerSelectorLabels(newNodeSet("foo", controller.Name, 2)).Build(),
					},
				}),
			},
			args: args{
				nodeset: newNodeSet("foo", controller.Name, 2),
			},
			want: []*appsv1.ControllerRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       corev1.NamespaceDefault,
						Name:            "foo-00000",
						Labels:          labels.NewBuilder().WithWorkerSelectorLabels(newNodeSet("foo", controller.Name, 2)).Build(),
						ResourceVersion: "999",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "No matching labels",
			fields: fields{
				Client: fake.NewFakeClient(&appsv1.ControllerRevision{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "foo-00000",
					},
				}),
			},
			args: args{
				nodeset: newNodeSet("foo", controller.Name, 2),
			},
			want:    []*appsv1.ControllerRevision{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			got, err := r.listRevisions(tt.args.nodeset)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.listRevisions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("NodeSetReconciler.listRevisions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeSetReconciler_getNodeSetPods(t *testing.T) {
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
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "selector match",
			fields: fields{
				Client: fake.NewFakeClient(
					nodeset.DeepCopy(),
					nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, ""),
					&corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name: "blank",
						},
					}),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
			},
			want:    []string{klog.KObj(nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")).String()},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			got, err := r.getNodeSetPods(tt.args.ctx, tt.args.nodeset)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.getNodeSetPods() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotPodNames := make([]string, len(got))
			for i, pod := range got {
				gotPodNames[i] = klog.KObj(pod).String()
			}
			if diff := cmp.Diff(tt.want, gotPodNames); diff != "" {
				t.Errorf("NodeSetReconciler.getNodeSetPods() (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestNodeSetReconciler_sync(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "slurm",
		},
	}
	hash := "test-hash"
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
		hash    string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Succeeds with zero replicas and empty pod list",
			fields: fields{
				Client:    fake.NewFakeClient(controller.DeepCopy()),
				ClientMap: clientmap.NewClientMap(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: newNodeSet("foo", controller.Name, 0),
				pods:    []*corev1.Pod{},
				hash:    hash,
			},
			wantErr: false,
		},
		{
			name: "Error propagated from RefreshNodeCache on Slurm list failure",
			fields: fields{
				Client: fake.NewFakeClient(controller.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					sclient := newFakeClientList(sinterceptor.Funcs{
						List: func(ctx context.Context, list slurmobject.ObjectList, opts ...slurmclient.ListOption) error {
							return errors.New("slurm connection refused")
						},
					})
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: newNodeSet("foo", controller.Name, 2),
				pods:    []*corev1.Pod{},
				hash:    hash,
			},
			wantErr: true,
		},
		{
			name: "All sync steps succeed with empty pod list",
			fields: fields{
				Client: fake.NewFakeClient(controller.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: newNodeSet("foo", controller.Name, 0),
				pods:    []*corev1.Pod{},
				hash:    hash,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.sync(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.hash); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.sync() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_syncNodeSetPods(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "slurm",
		},
	}
	hash := "test-hash"
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
		hash    string
	}
	type testCaseFields struct {
		name     string
		fields   fields
		args     args
		wantPods int
		wantErr  bool
	}
	tests := []testCaseFields{
		{
			name: "Scale up from 0 to 2 creates pods",
			fields: fields{
				Client: fake.NewFakeClient(controller.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: newNodeSet("foo", controller.Name, 2),
				pods:    []*corev1.Pod{},
				hash:    hash,
			},
			wantPods: 2,
			wantErr:  false,
		},
		func() testCaseFields {
			ns := newNodeSet("foo", controller.Name, 2)
			pod0 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), ns, controller, 0, hash)
			makePodHealthy(pod0)
			pod1 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), ns, controller, 1, hash)
			makePodHealthy(pod1)
			nodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					*newNodeSetPodSlurmNode(pod0),
					*newNodeSetPodSlurmNode(pod1),
				},
			}
			sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
			return testCaseFields{
				name: "Steady state with matching replica count processes pods",
				fields: fields{
					Client:    fake.NewFakeClient(controller.DeepCopy(), ns.DeepCopy(), pod0.DeepCopy(), pod1.DeepCopy()),
					ClientMap: newClientMap(controller.Name, sclient),
				},
				args: args{
					ctx:     context.TODO(),
					nodeset: ns.DeepCopy(),
					pods:    []*corev1.Pod{pod0.DeepCopy(), pod1.DeepCopy()},
					hash:    hash,
				},
				wantPods: 2,
				wantErr:  false,
			}
		}(),
		func() testCaseFields {
			ns := newNodeSet("foo", controller.Name, 1)
			pod0 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), ns, controller, 0, hash)
			makePodHealthy(pod0)
			pod1 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), ns, controller, 1, hash)
			makePodHealthy(pod1)
			pod2 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), ns, controller, 2, hash)
			makePodHealthy(pod2)
			nodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					*newNodeSetPodSlurmNode(pod0),
					*newNodeSetPodSlurmNode(pod1),
					*newNodeSetPodSlurmNode(pod2),
				},
			}
			sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
			return testCaseFields{
				name: "Scale down from 3 to 1 deletes excess pods",
				fields: fields{
					Client:    fake.NewFakeClient(controller.DeepCopy(), ns.DeepCopy(), pod0.DeepCopy(), pod1.DeepCopy(), pod2.DeepCopy()),
					ClientMap: newClientMap(controller.Name, sclient),
				},
				args: args{
					ctx:     context.TODO(),
					nodeset: ns.DeepCopy(),
					pods:    []*corev1.Pod{pod0.DeepCopy(), pod1.DeepCopy(), pod2.DeepCopy()},
					hash:    hash,
				},
				wantPods: 1,
				wantErr:  false,
			}
		}(),
		{
			name: "Scale up fails when Controller CR is missing",
			fields: fields{
				Client: fake.NewFakeClient(),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: newNodeSet("foo", controller.Name, 2),
				pods:    []*corev1.Pod{},
				hash:    hash,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.syncNodeSetPods(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.hash); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.syncNodeSetPods() error = %v, wantErr %v", err, tt.wantErr)
			}
			podList := &corev1.PodList{}
			optsList := &client.ListOptions{
				Namespace: tt.args.nodeset.Namespace,
			}
			err := tt.fields.Client.List(ctx, podList, optsList)
			if err != nil {
				t.Errorf("Failed to list pods for NodeSet error = %v", err)
			}

			// If we are not scaling down, podList.Items should reflect current state
			if len(tt.args.pods) <= tt.wantPods {
				if len(podList.Items) != tt.wantPods {
					t.Errorf("syncNodeSetPods() failed: expected pod count = %v, got pod count = %v", tt.wantPods, len(podList.Items))
				}
			}

			// If we are scaling down, we need to sync again
			if len(tt.args.pods) > tt.wantPods {
				if err := r.syncNodeSetPods(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.hash); (err != nil) != tt.wantErr {
					t.Errorf("NodeSetReconciler.syncNodeSetPods() error = %v, wantErr %v", err, tt.wantErr)
				}
				err := tt.fields.Client.List(ctx, podList, optsList)
				if err != nil {
					t.Errorf("Failed to list pods for NodeSet error = %v", err)
				}
			}
		})
	}
}

func TestNodeSetReconciler_syncNodeTaint(t *testing.T) {

	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodesetNoTaint := newNodeSet("foo", controller.Name, 2)
	nodesetNoTaint.UID = "1234"
	podNoTaint := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodesetNoTaint, controller, 0, "")
	podNoTaint.Spec.NodeName = "node1"
	podNoTaint.Status.Phase = corev1.PodRunning
	if err := controllerutil.SetControllerReference(nodesetNoTaint, podNoTaint, clientgoscheme.Scheme); err != nil {
		t.Errorf("TestNodeSetReconciler_syncNodeTaint() unable to SetControllerReference to %v for %v: %v", nodesetNoTaint, podNoTaint, err)
	}

	nodesetTaint := newNodeSet("bar", controller.Name, 2)
	nodesetTaint.Spec.TaintKubeNodes = true //nolint:staticcheck // SA1019
	nodesetTaint.UID = "2345"
	podTaint := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodesetTaint, controller, 0, "")
	podTaint.Spec.NodeName = "node1"
	podTaint.Status.Phase = corev1.PodRunning
	if err := controllerutil.SetControllerReference(nodesetTaint, podTaint, clientgoscheme.Scheme); err != nil {
		t.Errorf("TestNodeSetReconciler_syncNodeTaint() unable to SetControllerReference to %v for %v: %v", nodesetTaint, podTaint, err)
	}

	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
		},
	}

	emptyNode := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node2",
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{
				slurmtaints.TaintNodeWorker,
			},
		},
	}

	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
		node    *corev1.Node
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantErr   bool
		wantTaint bool
	}{
		{
			name: "No taints applied for NodeSet with default, TaintKubeNodes: false",
			fields: fields{
				Client: fake.NewFakeClient(
					nodesetNoTaint.DeepCopy(),
					&node,
					podNoTaint,
				),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name: ptr.To(nodesetutils.GetSlurmNodeName(podNoTaint)),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodesetNoTaint,
				pod:     podNoTaint,
				node:    &node,
			},
			wantErr:   false,
			wantTaint: false,
		},
		{
			name: "Taints applied for NodeSet with TaintKubeNodes: true",
			fields: fields{
				Client: fake.NewFakeClient(
					nodesetTaint.DeepCopy(),
					&node,
					podTaint,
				),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name: ptr.To(nodesetutils.GetSlurmNodeName(podTaint)),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodesetTaint,
				pod:     podTaint,
				node:    &node,
			},
			wantErr:   false,
			wantTaint: true,
		},
		{
			name: "Taints removed from Node with no NodeSet pods",
			fields: fields{
				Client: fake.NewFakeClient(
					&emptyNode,
				),
			},
			args: args{
				ctx:  context.TODO(),
				node: &emptyNode,
			},
			wantErr:   false,
			wantTaint: false,
		},
		{
			name: "Taints not applied to Node with no NodeSet pods with TaintKubeNodes: true",
			fields: fields{
				Client: fake.NewFakeClient(
					nodesetTaint.DeepCopy(),
					&node,
				),
			},
			args: args{
				ctx:  context.TODO(),
				node: &node,
			},
			wantErr:   false,
			wantTaint: false,
		},
		{
			name: "Taints applied to Node with NodeSet pods with both TaintKubeNodes: true and TaintKubeNodes: false",
			fields: fields{
				Client: fake.NewFakeClient(
					nodesetNoTaint.DeepCopy(),
					nodesetTaint.DeepCopy(),
					&node,
					podTaint,
					podNoTaint,
				),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name: ptr.To(nodesetutils.GetSlurmNodeName(podTaint)),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:  context.TODO(),
				node: &node,
			},
			wantErr:   false,
			wantTaint: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.syncNodeTaint(ctx); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.syncNodeTaint() error = %v, wantErr %v", err, tt.wantErr)
			}
			node := &corev1.Node{}
			key := client.ObjectKeyFromObject(tt.args.node)
			if err := r.Get(tt.args.ctx, key, node); err != nil {
				t.Errorf("NodeSetReconciler.syncNodeTaint() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantTaint != taints.TaintExists(node.Spec.Taints, &slurmtaints.TaintNodeWorker) {
				t.Errorf("NodeSetReconciler.syncNodeTaint() slice.Contains(node.Spec.Taints, slurmtaints.TaintNodeWorker) = %v, wantTaintNoExecute = %v", node.Spec.Taints, tt.wantTaint)
			}
		})
	}
}

func TestNodeSetReconciler_processCondemned(t *testing.T) {
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
		ctx       context.Context
		nodeset   *slinkyv1beta1.NodeSet
		condemned []*corev1.Pod
		i         int
	}
	type testCaseFields struct {
		name           string
		fields         fields
		args           args
		wantErr        bool
		wantDrain      bool
		wantDelete     bool
		wantPodDeleted bool
	}
	tests := []testCaseFields{
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "pod-0",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			client := fake.NewFakeClient(nodeset, podList)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pods[0])),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, slurmClient)

			return testCaseFields{
				name: "drain",
				fields: fields{
					Client:    client,
					ClientMap: clientMap,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset,
					condemned: pods,
					i:         0,
				},
				wantErr:        false,
				wantDrain:      true,
				wantDelete:     false,
				wantPodDeleted: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "pod-0",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			client := fake.NewFakeClient(nodeset, podList)
			slurmClient := newFakeClientList(sinterceptor.Funcs{})
			clientMap := newClientMap(controller.Name, slurmClient)

			return testCaseFields{
				name: "delete",
				fields: fields{
					Client:    client,
					ClientMap: clientMap,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset,
					condemned: pods,
					i:         0,
				},
				wantErr:        false,
				wantDrain:      false,
				wantDelete:     true,
				wantPodDeleted: true,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "pod-0",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			client := fake.NewFakeClient(nodeset, podList)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name: ptr.To(nodesetutils.GetSlurmNodeName(pods[0])),
							State: ptr.To([]slurmapi.V0044NodeState{
								slurmapi.V0044NodeStateIDLE,
								slurmapi.V0044NodeStateDRAIN,
							}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, slurmClient)

			return testCaseFields{
				name: "delete after drain",
				fields: fields{
					Client:    client,
					ClientMap: clientMap,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset,
					condemned: pods,
					i:         0,
				},
				wantErr:        false,
				wantDrain:      true,
				wantDelete:     false,
				wantPodDeleted: true,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			now := metav1.Now()
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:         corev1.NamespaceDefault,
						Name:              "pod-0",
						DeletionTimestamp: &now,
						Finalizers:        []string{"test-finalizer"},
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			client := fake.NewFakeClient(nodeset, podList)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(nodesetutils.GetSlurmNodeName(pods[0])),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, slurmClient)

			return testCaseFields{
				name: "skip terminating pod",
				fields: fields{
					Client:    client,
					ClientMap: clientMap,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset,
					condemned: pods,
					i:         0,
				},
				wantErr:        false,
				wantDrain:      false,
				wantDelete:     false,
				wantPodDeleted: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "pod-0",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			client := fake.NewFakeClient(nodeset, podList)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(nodesetutils.GetSlurmNodeName(pods[0])),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateALLOCATED}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, slurmClient)

			return testCaseFields{
				name: "wait for drain while slurm node is busy",
				fields: fields{
					Client:    client,
					ClientMap: clientMap,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset,
					condemned: pods,
					i:         0,
				},
				wantErr:        false,
				wantDrain:      true,
				wantDelete:     false,
				wantPodDeleted: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "pod-0",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			client := fake.NewClientBuilder().
				WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
						return http.ErrHandlerTimeout
					},
					Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return http.ErrHandlerTimeout
					},
				}).
				WithRuntimeObjects(nodeset, podList).
				Build()
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pods[0])),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			clientMap := newClientMap(controller.Name, slurmClient)

			return testCaseFields{
				name: "k8s error",
				fields: fields{
					Client:    client,
					ClientMap: clientMap,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset,
					condemned: pods,
					i:         0,
				},
				wantErr:        true,
				wantDrain:      false,
				wantDelete:     false,
				wantPodDeleted: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pods := []*corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "pod-0",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			}
			podList := &corev1.PodList{
				Items: structutils.DereferenceList(pods),
			}
			client := fake.NewFakeClient(nodeset, podList)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name: ptr.To(nodesetutils.GetSlurmNodeName(pods[0])),
						},
					},
				},
			}
			slurmInterceptorFn := sinterceptor.Funcs{
				Update: func(ctx context.Context, obj slurmobject.Object, req any, opts ...slurmclient.UpdateOption) error {
					return http.ErrHandlerTimeout
				},
			}
			slurmClient := newFakeClientList(slurmInterceptorFn, slurmNodeList)
			clientMap := newClientMap(controller.Name, slurmClient)

			return testCaseFields{
				name: "slurm error",
				fields: fields{
					Client:    client,
					ClientMap: clientMap,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset,
					condemned: pods,
					i:         0,
				},
				wantErr:        true,
				wantDrain:      false,
				wantDelete:     false,
				wantPodDeleted: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.processCondemned(tt.args.ctx, tt.args.nodeset, tt.args.condemned, tt.args.i); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.processCondemned() error = %v, wantErr %v", err, tt.wantErr)
			}
			pod := tt.args.condemned[tt.args.i]
			if isDrain, err := r.slurmControl.IsNodeDrain(tt.args.ctx, tt.args.nodeset, pod); err != nil {
				t.Errorf("slurmControl.IsNodeDrain() error = %v", err)
			} else if isDrain != tt.wantDrain && !tt.wantDelete {
				t.Errorf("slurmControl.IsNodeDrain() = %v, wantDrain %v", isDrain, tt.wantDrain)
			}
			key := client.ObjectKeyFromObject(pod)
			err := r.Get(tt.args.ctx, key, pod)
			podStillExists := err == nil
			podGone := apierrors.IsNotFound(err)
			if err != nil && !podGone {
				t.Errorf("Client.Get() error = %v", err)
			}
			if tt.wantPodDeleted && !podGone {
				t.Errorf("expected pod to be deleted from the API server")
			}
			if !tt.wantPodDeleted && !podStillExists && !tt.wantErr {
				t.Errorf("expected pod to still exist")
			}
		})
	}
}

func TestNodeSetReconciler_syncCordon(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "slurm",
			Namespace: corev1.NamespaceDefault,
		},
	}
	nodeset := newNodeSet("nodeset-a", controller.Name, 2)

	newPod := func(cordon bool) *corev1.Pod {
		p := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")
		p.Spec.NodeName = "kube-node-1"
		if cordon {
			if p.Annotations == nil {
				p.Annotations = make(map[string]string)
			}
			p.Annotations[slinkyv1beta1.AnnotationPodCordon] = "true"
		}
		return p
	}

	slurmNodeName := nodesetutils.GetSlurmNodeName(newPod(false))

	tests := []struct {
		name                     string
		kubeNode                 *corev1.Node
		pod                      *corev1.Pod
		slurmNodeList            *slurmtypes.V0044NodeList
		propagatedNodeConditions []corev1.NodeConditionType
		wantErr                  bool
		wantPodCordoned          bool
		wantSlurmDrain           bool
		wantReasonSub            string
	}{
		{
			name: "kubernetes node cordoned cordons pod and drains slurm",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: true},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			},
			wantPodCordoned: true,
			wantSlurmDrain:  true,
			wantReasonSub:   "kube-node-1",
		},
		{
			name: "kubernetes node cordon reason annotation is propagated to slurm",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-node-1",
					Annotations: map[string]string{
						slinkyv1beta1.AnnotationNodeCordonReason: "custom maintenance window",
					},
				},
				Spec: corev1.NodeSpec{Unschedulable: true},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			},
			wantPodCordoned: true,
			wantSlurmDrain:  true,
			wantReasonSub:   "custom maintenance window",
		},
		{
			name: "cordoned pod on schedulable node drains slurm",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: false},
			},
			pod: newPod(true),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			},
			wantPodCordoned: true,
			wantSlurmDrain:  true,
		},
		{
			name: "uncordoned pod undrains slurm node",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: false},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name: new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{
								slurmapi.V0044NodeStateIDLE,
								slurmapi.V0044NodeStateDRAIN,
							}),
						},
					},
				},
			},
			wantPodCordoned: false,
			wantSlurmDrain:  false,
		},
		{
			name: "external slurm reason is left unchanged",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: false},
			},
			pod: newPod(true),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:   new(slurmNodeName),
							State:  new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
							Reason: new("manual operator drain outside slurm-operator"),
						},
					},
				},
			},
			wantPodCordoned: true,
			wantSlurmDrain:  false,
		},
		{
			name: "unresponsive slurm node is left unchanged",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: false},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:   new(slurmNodeName),
							State:  new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateDOWN}),
							Reason: new("Not responding to ping"),
						},
					},
				},
			},
			wantPodCordoned: false,
			wantSlurmDrain:  false,
		},
		{
			name: "propagated node condition true becomes slurm drain reason",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:    corev1.NodeDiskPressure,
							Status:  corev1.ConditionTrue,
							Reason:  "KubeletHasDiskPressure",
							Message: "POD has insufficient ephemeral storage",
						},
					},
				},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			},
			propagatedNodeConditions: []corev1.NodeConditionType{corev1.NodeDiskPressure},
			wantPodCordoned:          true,
			wantSlurmDrain:           true,
			wantReasonSub:            "(KubeletHasDiskPressure: POD has insufficient ephemeral storage)",
		},
		{
			name: "multiple propagated node conditions join with semicolon",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:    corev1.NodeMemoryPressure,
							Status:  corev1.ConditionTrue,
							Reason:  "KubeletHasInsufficientMemory",
							Message: "Memory pressure",
						},
						{
							Type:    corev1.NodeDiskPressure,
							Status:  corev1.ConditionTrue,
							Reason:  "KubeletHasDiskPressure",
							Message: "Disk pressure",
						},
					},
				},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			},
			propagatedNodeConditions: []corev1.NodeConditionType{
				corev1.NodeMemoryPressure,
				corev1.NodeDiskPressure,
			},
			wantPodCordoned: true,
			wantSlurmDrain:  true,
			wantReasonSub:   "(KubeletHasInsufficientMemory: Memory pressure); (KubeletHasDiskPressure: Disk pressure)",
		},
		{
			name: "propagated type not true falls back to default cordon reason",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:    corev1.NodeDiskPressure,
							Status:  corev1.ConditionFalse,
							Reason:  "KubeletHasNoDiskPressure",
							Message: "ignored",
						},
					},
				},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			},
			propagatedNodeConditions: []corev1.NodeConditionType{corev1.NodeDiskPressure},
			wantPodCordoned:          true,
			wantSlurmDrain:           true,
			wantReasonSub:            "kube-node-1",
		},
		{
			name: "node condition not in propagated list uses default cordon reason",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-node-1"},
				Spec:       corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:    corev1.NodeDiskPressure,
							Status:  corev1.ConditionTrue,
							Reason:  "KubeletHasDiskPressure",
							Message: "should not propagate",
						},
					},
				},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			},
			propagatedNodeConditions: []corev1.NodeConditionType{corev1.NodeMemoryPressure},
			wantPodCordoned:          true,
			wantSlurmDrain:           true,
			wantReasonSub:            "kube-node-1",
		},
		{
			name: "node cordon reason annotation overrides propagated conditions",
			kubeNode: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kube-node-1",
					Annotations: map[string]string{
						slinkyv1beta1.AnnotationNodeCordonReason: "annotation overrides conditions",
					},
				},
				Spec: corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:    corev1.NodePIDPressure,
							Status:  corev1.ConditionTrue,
							Reason:  "KubeletHasPIDPressure",
							Message: "many processes",
						},
					},
				},
			},
			pod: newPod(false),
			slurmNodeList: &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  new(slurmNodeName),
							State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			},
			propagatedNodeConditions: []corev1.NodeConditionType{corev1.NodePIDPressure},
			wantPodCordoned:          true,
			wantSlurmDrain:           true,
			wantReasonSub:            "annotation overrides conditions",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := tt.pod.DeepCopy()
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, tt.slurmNodeList)
			clientMap := newClientMap(controller.Name, slurmClient)
			k8sClient := fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy(), tt.kubeNode.DeepCopy())
			r := newNodeSetControllerWithPropagatedNodeConditions(k8sClient, clientMap, tt.propagatedNodeConditions)

			if err := r.syncCordon(context.Background(), nodeset.DeepCopy(), []*corev1.Pod{pod}); (err != nil) != tt.wantErr {
				t.Fatalf("syncCordon() error = %v, wantErr %v", err, tt.wantErr)
			}

			gotPod := &corev1.Pod{}
			if err := r.Get(context.Background(), client.ObjectKeyFromObject(pod), gotPod); err != nil {
				t.Fatalf("Get pod: %v", err)
			}
			if got, want := podutils.IsPodCordon(gotPod), tt.wantPodCordoned; got != want {
				t.Errorf("IsPodCordon() = %v, want %v", got, want)
			}

			gotNode := &slurmtypes.V0044Node{}
			sc := r.ClientMap.Get(nodeset.Spec.ControllerRef.NamespacedName())
			if sc == nil {
				t.Fatal("ClientMap.Get() returned nil")
			}
			if err := sc.Get(context.Background(), slurmclient.ObjectKey(slurmNodeName), gotNode); err != nil {
				t.Fatalf("slurm Get node: %v", err)
			}
			if got, want := gotNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN), tt.wantSlurmDrain; got != want {
				t.Errorf("Slurm node DRAIN = %v, want %v", got, want)
			}
			if tt.wantReasonSub != "" {
				if reason := ptr.Deref(gotNode.Reason, ""); !strings.Contains(reason, tt.wantReasonSub) {
					t.Errorf("Slurm node Reason = %q, want substring %q", reason, tt.wantReasonSub)
				}
			}
		})
	}
}

func TestNodeSetReconciler_syncSlurmNodeUndrain_skipsWhenNotDrained(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "slurm",
			Namespace: corev1.NamespaceDefault,
		},
	}
	nodeset := newNodeSet("nodeset-b", controller.Name, 2)
	pod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")
	slurmName := nodesetutils.GetSlurmNodeName(pod)
	slurmNodeList := &slurmtypes.V0044NodeList{
		Items: []slurmtypes.V0044Node{
			{
				V0044Node: slurmapi.V0044Node{
					Name:  new(slurmName),
					State: new([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
				},
			},
		},
	}
	slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
	r := newNodeSetController(fake.NewFakeClient(nodeset), newClientMap(controller.Name, slurmClient))

	if err := r.syncSlurmNodeUndrain(context.Background(), nodeset, pod, "should not be used"); err != nil {
		t.Fatalf("syncSlurmNodeUndrain() = %v", err)
	}
}

func TestNodeSetReconciler_doPodProcessing(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 2)
	hash := "test-hash"
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx          context.Context
		nodeset      *slinkyv1beta1.NodeSet
		pods         []*corev1.Pod
		podsToDelete []*corev1.Pod
		hash         string
	}
	type testCaseFields struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}
	tests := []testCaseFields{
		{
			name: "Empty pod lists",
			fields: fields{
				Client: fake.NewFakeClient(nodeset.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:          context.TODO(),
				nodeset:      nodeset.DeepCopy(),
				pods:         []*corev1.Pod{},
				podsToDelete: []*corev1.Pod{},
				hash:         hash,
			},
			wantErr: false,
		},
		func() testCaseFields {
			pod0 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, hash)
			makePodHealthy(pod0)
			nodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					*newNodeSetPodSlurmNode(pod0),
				},
			}
			sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
			return testCaseFields{
				name: "Running pods with matching hash are processed",
				fields: fields{
					Client:    fake.NewFakeClient(nodeset.DeepCopy(), pod0.DeepCopy()),
					ClientMap: newClientMap(controller.Name, sclient),
				},
				args: args{
					ctx:          context.TODO(),
					nodeset:      nodeset.DeepCopy(),
					pods:         []*corev1.Pod{pod0.DeepCopy()},
					podsToDelete: []*corev1.Pod{},
					hash:         hash,
				},
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			pod0 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, hash)
			makePodHealthy(pod0)
			nodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod0)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateDRAIN}),
						},
					},
				},
			}
			sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
			return testCaseFields{
				name: "Pods to delete are condemned",
				fields: fields{
					Client:    fake.NewFakeClient(nodeset.DeepCopy(), pod0.DeepCopy()),
					ClientMap: newClientMap(controller.Name, sclient),
				},
				args: args{
					ctx:          context.TODO(),
					nodeset:      nodeset.DeepCopy(),
					pods:         []*corev1.Pod{},
					podsToDelete: []*corev1.Pod{pod0.DeepCopy()},
					hash:         hash,
				},
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			pod0 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, hash)
			makePodHealthy(pod0)
			sclient := newFakeClientList(sinterceptor.Funcs{
				Get: func(ctx context.Context, key slurmobject.ObjectKey, obj slurmobject.Object, opts ...slurmclient.GetOption) error {
					return errors.New("slurm connection refused")
				},
			})
			return testCaseFields{
				name: "Error propagated when condemned pod processing fails",
				fields: fields{
					Client:    fake.NewFakeClient(nodeset.DeepCopy(), pod0.DeepCopy()),
					ClientMap: newClientMap(controller.Name, sclient),
				},
				args: args{
					ctx:          context.TODO(),
					nodeset:      nodeset.DeepCopy(),
					pods:         []*corev1.Pod{},
					podsToDelete: []*corev1.Pod{pod0.DeepCopy()},
					hash:         hash,
				},
				wantErr: true,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.doPodProcessing(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.podsToDelete, tt.args.hash); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.doPodProcessing() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_processNodeSetPod(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 2)
	pod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		wantErr    bool
		wantDelete bool
	}{
		{
			name: "Running pod with consistent identity is a no-op",
			fields: fields{
				Client: func() client.Client {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodRunning
					return fake.NewFakeClient(nodeset.DeepCopy(), p)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod: func() *corev1.Pod {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodRunning
					return p
				}(),
			},
			wantErr: false,
		},
		{
			name: "Failed pod triggers deletion",
			fields: fields{
				Client: func() client.Client {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodFailed
					return fake.NewFakeClient(nodeset.DeepCopy(), p)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod: func() *corev1.Pod {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodFailed
					return p
				}(),
			},
			wantErr:    false,
			wantDelete: true,
		},
		{
			name: "Succeeded pod triggers deletion",
			fields: fields{
				Client: func() client.Client {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodSucceeded
					return fake.NewFakeClient(nodeset.DeepCopy(), p)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod: func() *corev1.Pod {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodSucceeded
					return p
				}(),
			},
			wantErr:    false,
			wantDelete: true,
		},
		{
			name: "Terminating failed pod is skipped",
			fields: fields{
				Client: fake.NewFakeClient(nodeset.DeepCopy()),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod: func() *corev1.Pod {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodFailed
					now := metav1.Now()
					p.DeletionTimestamp = &now
					p.Finalizers = []string{"test-finalizer"}
					return p
				}(),
			},
			wantErr:    false,
			wantDelete: false,
		},
		{
			name: "Pending pod calls update",
			fields: fields{
				Client: func() client.Client {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodPending
					return fake.NewFakeClient(nodeset.DeepCopy(), p)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod: func() *corev1.Pod {
					p := pod.DeepCopy()
					p.Status.Phase = corev1.PodPending
					return p
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.processNodeSetPod(tt.args.ctx, tt.args.nodeset, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.processNodeSetPod() error = %v, wantErr %v", err, tt.wantErr)
			}
			gotPod := &corev1.Pod{}
			err := tt.fields.Client.Get(tt.args.ctx, client.ObjectKeyFromObject(tt.args.pod), gotPod)
			if tt.wantDelete {
				if err == nil {
					t.Errorf("expected pod to be deleted, but it still exists")
				} else if !apierrors.IsNotFound(err) {
					t.Errorf("Client.Get() unexpected error = %v", err)
				}
			} else if err != nil && !apierrors.IsNotFound(err) {
				t.Errorf("Client.Get() unexpected error = %v", err)
			}
		})
	}
}

func TestNodeSetReconciler_makePodCordonAndDrain(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 2)
	pod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
		reason  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				Client: fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "success with drain state annotations",
			fields: fields{
				Client: fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name:   ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State:  ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateDRAIN}),
									Reason: ptr.To("test reason"),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "kubernetes update failure",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithInterceptorFuncs(interceptor.Funcs{
						Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							return http.ErrAbortHandler
						},
						Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
							return http.ErrAbortHandler
						},
					}).
					WithRuntimeObjects(nodeset.DeepCopy(), pod.DeepCopy()).
					Build(),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "slurm update failure",
			fields: fields{
				Client: fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{
						Update: func(ctx context.Context, obj slurmobject.Object, req any, opts ...slurmclient.UpdateOption) error {
							return errors.New(http.StatusText(http.StatusInternalServerError))
						},
					}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.makePodCordonAndDrain(tt.args.ctx, tt.args.nodeset, tt.args.pod, tt.args.reason); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.makePodCordonAndDrain() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Check Pod Annotations
			gotPod := &corev1.Pod{}
			if err := r.Get(tt.args.ctx, client.ObjectKeyFromObject(tt.args.pod), gotPod); err != nil {
				if !apierrors.IsNotFound(err) {
					t.Errorf("client.Get() error = %v", err)
				}
			} else if !tt.wantErr {
				if ok := podutils.IsPodCordon(gotPod); !ok {
					t.Errorf("IsPodCordon() = %v", ok)
				}
			}
			// Check Slurm Node State
			gotSlurmNode := &slurmtypes.V0044Node{}
			sc := r.ClientMap.Get(tt.args.nodeset.Spec.ControllerRef.NamespacedName())
			if sc == nil {
				t.Error("ClientMap.Get() is nil")
			}
			if err := sc.Get(tt.args.ctx, slurmclient.ObjectKey(nodesetutils.GetSlurmNodeName(tt.args.pod)), gotSlurmNode); err != nil {
				if err.Error() != http.StatusText(http.StatusNotFound) {
					t.Errorf("slurmclient.Get() error = %v", err)
				}
			} else if !tt.wantErr {
				if ok := gotSlurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN); !ok {
					t.Errorf("SlurmNode Has DRAIN = %v", ok)
				}
			}
		})
	}
}

func TestNodeSetReconciler_makePodCordon(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-0",
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
			Annotations: map[string]string{
				slinkyv1beta1.AnnotationPodCordon: "true",
			},
		},
	}
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx context.Context
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{

		{
			name: "NotFound",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				pod: pod1.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "cordoned",
			fields: fields{
				Client: fake.NewFakeClient(pod2.DeepCopy()),
			},
			args: args{
				ctx: context.TODO(),
				pod: pod2.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "not cordoned",
			fields: fields{
				Client: fake.NewFakeClient(pod1.DeepCopy()),
			},
			args: args{
				ctx: context.TODO(),
				pod: pod1.DeepCopy(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.makePodCordon(tt.args.ctx, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.makePodCordon() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Check Pod Annotations
			gotPod := &corev1.Pod{}
			if err := r.Get(tt.args.ctx, client.ObjectKeyFromObject(tt.args.pod), gotPod); err != nil {
				if !apierrors.IsNotFound(err) {
					t.Errorf("client.Get() error = %v", err)
				}
			} else if !tt.wantErr {
				if ok := podutils.IsPodCordon(gotPod); !ok {
					t.Errorf("IsPodCordon() = %v", ok)
				}
			}
		})
	}
}

func TestNodeSetReconciler_makePodUncordonAndUndrain(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 2)
	pod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")
	pod.Annotations[slinkyv1beta1.AnnotationPodCordon] = "true"
	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
		reason  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				Client: fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State: ptr.To([]slurmapi.V0044NodeState{
										slurmapi.V0044NodeStateIDLE,
										slurmapi.V0044NodeStateDRAIN,
									}),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "kubernetes update failure",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithInterceptorFuncs(interceptor.Funcs{
						Update: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							return http.ErrAbortHandler
						},
						Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
							return http.ErrAbortHandler
						},
					}).
					WithRuntimeObjects(nodeset.DeepCopy(), pod.DeepCopy()).
					Build(),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State: ptr.To([]slurmapi.V0044NodeState{
										slurmapi.V0044NodeStateIDLE,
										slurmapi.V0044NodeStateDRAIN,
									}),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "slurm update failure",
			fields: fields{
				Client: fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State: ptr.To([]slurmapi.V0044NodeState{
										slurmapi.V0044NodeStateIDLE,
										slurmapi.V0044NodeStateDRAIN,
									}),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{
						Update: func(ctx context.Context, obj slurmobject.Object, req any, opts ...slurmclient.UpdateOption) error {
							return errors.New(http.StatusText(http.StatusInternalServerError))
						},
					}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.makePodUncordonAndUndrain(tt.args.ctx, tt.args.nodeset, tt.args.pod, tt.args.reason); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.makePodUncordonAndUndrain() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Check Pod Annotations
			gotPod := &corev1.Pod{}
			if err := r.Get(tt.args.ctx, client.ObjectKeyFromObject(tt.args.pod), gotPod); err != nil {
				if !apierrors.IsNotFound(err) {
					t.Errorf("client.Get() error = %v", err)
				}
			} else if !tt.wantErr {
				if ok := podutils.IsPodCordon(gotPod); ok {
					t.Errorf("IsPodCordon() = %v", ok)
				}
			}
			// Check Slurm Node State
			gotSlurmNode := &slurmtypes.V0044Node{}
			sc := r.ClientMap.Get(tt.args.nodeset.Spec.ControllerRef.NamespacedName())
			if sc == nil {
				t.Error("ClientMap.Get() is nil")
			}
			if err := sc.Get(tt.args.ctx, slurmclient.ObjectKey(nodesetutils.GetSlurmNodeName(tt.args.pod)), gotSlurmNode); err != nil {
				if err.Error() != http.StatusText(http.StatusNotFound) {
					t.Errorf("slurmclient.Get() error = %v", err)
				}
			} else if !tt.wantErr {
				if ok := gotSlurmNode.GetStateAsSet().Has(slurmapi.V0044NodeStateDRAIN); ok {
					t.Errorf("SlurmNode Has DRAIN = %v", ok)
				}
			}
		})
	}
}

func TestNodeSetReconciler_makePodUncordon(t *testing.T) {
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-0",
			Annotations: map[string]string{
				slinkyv1beta1.AnnotationPodCordon: "true",
			},
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod-1",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx context.Context
		pod *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "NotFound",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				pod: pod1.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "cordoned",
			fields: fields{
				Client: fake.NewFakeClient(pod1.DeepCopy()),
			},
			args: args{
				ctx: context.TODO(),
				pod: pod1.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "not cordoned",
			fields: fields{
				Client: fake.NewFakeClient(pod2.DeepCopy()),
			},
			args: args{
				ctx: context.TODO(),
				pod: pod2.DeepCopy(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			if err := r.makePodUncordon(tt.args.ctx, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.makePodUncordon() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Check Pod Annotations
			gotPod := &corev1.Pod{}
			if err := r.Get(tt.args.ctx, client.ObjectKeyFromObject(tt.args.pod), gotPod); err != nil {
				if !apierrors.IsNotFound(err) {
					t.Errorf("client.Get() error = %v", err)
				}
			} else if !tt.wantErr {
				if ok := podutils.IsPodCordon(gotPod); ok {
					t.Errorf("IsPodCordon() = %v", ok)
				}
			}
		})
	}
}

func TestNodeSetReconciler_syncUpdate(t *testing.T) {
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
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
		hash    string
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
			nodeset.Spec.UpdateStrategy.Type = slinkyv1beta1.OnDeleteNodeSetStrategyType
			pod1 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, hash)
			pod2 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 1, "")
			k8sclient := fake.NewFakeClient(nodeset, pod1, pod2)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod1)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod2)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			return testCaseFields{
				name: "OnDelete",
				fields: fields{
					Client:    k8sclient,
					ClientMap: newClientMap(controller.Name, slurmClient),
				},
				args: args{
					ctx:     context.TODO(),
					nodeset: nodeset,
					pods:    []*corev1.Pod{pod1, pod2},
					hash:    hash,
				},
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			nodeset.Spec.UpdateStrategy.Type = slinkyv1beta1.RollingUpdateNodeSetStrategyType
			nodeset.Spec.UpdateStrategy.RollingUpdate = slinkyv1beta1.RollingUpdateNodeSetStrategy{
				MaxUnavailable: ptr.To(intstr.FromString("10%")),
			}
			pod1 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, hash)
			pod2 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 1, "")
			k8sclient := fake.NewFakeClient(nodeset, pod1, pod2)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod1)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod2)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			return testCaseFields{
				name: "RollingUpdate",
				fields: fields{
					Client:    k8sclient,
					ClientMap: newClientMap(controller.Name, slurmClient),
				},
				args: args{
					ctx:     context.TODO(),
					nodeset: nodeset,
					pods:    []*corev1.Pod{pod1, pod2},
					hash:    hash,
				},
				wantErr: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.syncUpdate(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.hash); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.syncUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_syncRollingUpdate(t *testing.T) {
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
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
		hash    string
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
			nodeset.Spec.UpdateStrategy.Type = slinkyv1beta1.RollingUpdateNodeSetStrategyType
			nodeset.Spec.UpdateStrategy.RollingUpdate = slinkyv1beta1.RollingUpdateNodeSetStrategy{
				MaxUnavailable: ptr.To(intstr.FromString("10%")),
			}
			pod1 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, hash)
			makePodHealthy(pod1)
			pod2 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 1, "")
			makePodHealthy(pod2)
			k8sclient := fake.NewFakeClient(nodeset, pod1, pod2)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod1)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod2)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			return testCaseFields{
				name: "update",
				fields: fields{
					Client:    k8sclient,
					ClientMap: newClientMap(controller.Name, slurmClient),
				},
				args: args{
					ctx:     context.TODO(),
					nodeset: nodeset,
					pods:    []*corev1.Pod{pod1, pod2},
					hash:    hash,
				},
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			nodeset.Spec.UpdateStrategy.Type = slinkyv1beta1.RollingUpdateNodeSetStrategyType
			nodeset.Spec.UpdateStrategy.RollingUpdate = slinkyv1beta1.RollingUpdateNodeSetStrategy{
				MaxUnavailable: ptr.To(intstr.FromString("10%")),
			}
			pod1 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, hash)
			makePodHealthy(pod1)
			pod2 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 1, hash)
			makePodHealthy(pod2)
			k8sclient := fake.NewFakeClient(nodeset, pod1, pod2)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod1)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod2)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			return testCaseFields{
				name: "no update",
				fields: fields{
					Client:    k8sclient,
					ClientMap: newClientMap(controller.Name, slurmClient),
				},
				args: args{
					ctx:     context.TODO(),
					nodeset: nodeset,
					pods:    []*corev1.Pod{pod1, pod2},
					hash:    hash,
				},
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", controller.Name, 2)
			nodeset.Spec.UpdateStrategy.Type = slinkyv1beta1.RollingUpdateNodeSetStrategyType
			nodeset.Spec.UpdateStrategy.RollingUpdate = slinkyv1beta1.RollingUpdateNodeSetStrategy{
				MaxUnavailable: ptr.To(intstr.FromString("10%")),
			}
			pod1 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")
			makePodHealthy(pod1)
			pod2 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 1, "")
			k8sclient := fake.NewFakeClient(nodeset, pod1, pod2)
			slurmNodeList := &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{
						V0044Node: slurmapi.V0044Node{
							Name:  ptr.To(nodesetutils.GetSlurmNodeName(pod1)),
							State: ptr.To([]slurmapi.V0044NodeState{slurmapi.V0044NodeStateIDLE}),
						},
					},
				},
			}
			slurmClient := newFakeClientList(sinterceptor.Funcs{}, slurmNodeList)
			return testCaseFields{
				name: "update, with unhealthy",
				fields: fields{
					Client:    k8sclient,
					ClientMap: newClientMap(controller.Name, slurmClient),
				},
				args: args{
					ctx:     context.TODO(),
					nodeset: nodeset,
					pods:    []*corev1.Pod{pod1, pod2},
					hash:    hash,
				},
				wantErr: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.syncRollingUpdate(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.hash); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.syncRollingUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_splitUpdatePods(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	now := metav1.Now()
	const hash = "12345"
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pods    []*corev1.Pod
		hash    string
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantPodsToDelete []string
		wantPodsToKeep   []string
	}{
		{
			name: "OnDelete",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				nodeset: func() *slinkyv1beta1.NodeSet {
					nodeset := newNodeSet("foo", controller.Name, 0)
					nodeset.Spec.UpdateStrategy.Type = slinkyv1beta1.OnDeleteNodeSetStrategyType
					return nodeset
				}(),
				pods: []*corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-0",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: hash,
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodFailed,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-1",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: "",
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{
									Type:               corev1.PodReady,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: now,
								},
							},
						},
					},
				},
				hash: hash,
			},
			wantPodsToDelete: []string{},
			wantPodsToKeep:   []string{},
		},
		{
			name: "RollingUpdate",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				nodeset: func() *slinkyv1beta1.NodeSet {
					nodeset := newNodeSet("foo", controller.Name, 0)
					nodeset.Spec.UpdateStrategy.Type = slinkyv1beta1.RollingUpdateNodeSetStrategyType
					nodeset.Spec.UpdateStrategy.RollingUpdate = slinkyv1beta1.RollingUpdateNodeSetStrategy{
						MaxUnavailable: ptr.To(intstr.FromString("100%")),
					}
					return nodeset
				}(),
				pods: []*corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-0",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: hash,
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodFailed,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-1",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: "",
							},
						},
						Status: corev1.PodStatus{
							Phase: corev1.PodRunning,
							Conditions: []corev1.PodCondition{
								{
									Type:               corev1.PodReady,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: now,
								},
							},
						},
					},
				},
				hash: hash,
			},
			wantPodsToDelete: []string{},
			wantPodsToKeep:   []string{"pod-0", "pod-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			gotPodsToDelete, gotPodsToKeep := r.splitUpdatePods(tt.args.ctx, tt.args.nodeset, tt.args.pods, tt.args.hash)

			gotPodsToDeleteOrdered := make([]string, len(gotPodsToDelete))
			for i := range gotPodsToDelete {
				gotPodsToDeleteOrdered[i] = gotPodsToDelete[i].Name
			}
			gotPodsToKeepOrdered := make([]string, len(gotPodsToKeep))
			for i := range gotPodsToKeep {
				gotPodsToKeepOrdered[i] = gotPodsToKeep[i].Name
			}

			slices.Sort(gotPodsToDeleteOrdered)
			slices.Sort(gotPodsToKeepOrdered)
			if diff := cmp.Diff(tt.wantPodsToDelete, gotPodsToDeleteOrdered); diff != "" {
				t.Errorf("gotPodsToDelete (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantPodsToKeep, gotPodsToKeepOrdered); diff != "" {
				t.Errorf("gotPodsToKeep (-want,+got):\n%s", diff)
			}
		})
	}
}

func Test_findUpdatedPods(t *testing.T) {
	type args struct {
		pods []*corev1.Pod
		hash string
	}
	tests := []struct {
		name        string
		args        args
		wantNewPods []string
		wantOldPods []string
	}{
		{
			name: "1 new, 1 old",
			args: func() args {
				const hash = "12345"
				pods := []*corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-0",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: hash,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-1",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: "",
							},
						},
					},
				}
				return args{
					pods: pods,
					hash: hash,
				}
			}(),
			wantNewPods: []string{"pod-0"},
			wantOldPods: []string{"pod-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNewPods, gotOldPods := findUpdatedPods(tt.args.pods, tt.args.hash)

			gotNewPodsOrdered := make([]string, len(gotNewPods))
			for i := range tt.wantNewPods {
				gotNewPodsOrdered[i] = gotNewPods[i].Name
			}
			gotOldPodsOrdered := make([]string, len(gotOldPods))
			for i := range tt.wantNewPods {
				gotOldPodsOrdered[i] = gotOldPods[i].Name
			}

			slices.Sort(gotNewPodsOrdered)
			slices.Sort(gotOldPodsOrdered)
			if diff := cmp.Diff(tt.wantNewPods, gotNewPodsOrdered); diff != "" {
				t.Errorf("gotNewPods (-want,+got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantOldPods, gotOldPodsOrdered); diff != "" {
				t.Errorf("gotOldPods (-want,+got):\n%s", diff)
			}
		})
	}
}

func TestNodeSetReconciler_syncClusterWorkerService(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: newNodeSet("gpu-1", controller.Name, 2),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			if err := r.syncClusterWorkerService(tt.args.ctx, tt.args.nodeset); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.syncClusterWorkerService() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_isNodeCordoned(t *testing.T) {

	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx context.Context
		pod *corev1.Pod
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Node not cordoned, Pod not cordoned",
			fields: fields{
				Client: fake.NewFakeClient(
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
						},
						Spec: corev1.NodeSpec{
							Unschedulable: false,
						},
					},
				),
			},
			args: args{
				ctx: context.TODO(),
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-pod",
						Annotations: map[string]string{
							// No cordon annotation
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
			},
			want: false,
		},
		{
			name: "Node not cordoned, Pod cordoned",
			fields: fields{
				Client: fake.NewFakeClient(
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
						},
						Spec: corev1.NodeSpec{
							Unschedulable: false,
						},
					},
				),
			},
			args: args{
				ctx: context.TODO(),
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
						Annotations: map[string]string{
							slinkyv1beta1.AnnotationPodCordon: "true",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
			},
			want: false,
		},
		{
			name: "Node cordoned, Pod not cordoned",
			fields: fields{
				Client: fake.NewFakeClient(
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
						},
						Spec: corev1.NodeSpec{
							Unschedulable: true,
						},
					},
				),
			},
			args: args{
				ctx: context.TODO(),
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-pod",
						Annotations: map[string]string{
							// No cordon annotation
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
			},
			want: true,
		},
		{
			name: "Node cordoned, Pod cordoned",
			fields: fields{
				Client: fake.NewFakeClient(
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
						},
						Spec: corev1.NodeSpec{
							Unschedulable: true, // Node is cordoned
						},
					},
				),
			},
			args: args{
				ctx: context.TODO(),
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-pod",
						Annotations: map[string]string{
							slinkyv1beta1.AnnotationPodCordon: "true",
						},
					},
					Spec: corev1.PodSpec{
						NodeName: "test-node",
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			if got := r.isNodeCordoned(tt.args.ctx, tt.args.pod); got != tt.want {
				t.Errorf("isNodeCordoned() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_syncPodUncordon(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 2)
	pod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")
	pod.Annotations[slinkyv1beta1.AnnotationPodCordon] = "true"

	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
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
		wantErr bool
	}{
		{
			name: "success - pod uncordoned when node not cordoned",
			fields: fields{
				Client: fake.NewFakeClient(
					nodeset.DeepCopy(),
					pod.DeepCopy(),
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
						},
						Spec: corev1.NodeSpec{
							Unschedulable: false, // Node not cordoned
						},
					},
				),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State: ptr.To([]slurmapi.V0044NodeState{
										slurmapi.V0044NodeStateIDLE,
										slurmapi.V0044NodeStateDRAIN,
									}),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "skip - pod not uncordoned when node is cordoned",
			fields: fields{
				Client: fake.NewFakeClient(
					nodeset.DeepCopy(),
					pod.DeepCopy(),
					&corev1.Node{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-node",
						},
						Spec: corev1.NodeSpec{
							Unschedulable: true, // Node is cordoned
						},
					},
				),
				ClientMap: func() *clientmap.ClientMap {
					nodeList := &slurmtypes.V0044NodeList{
						Items: []slurmtypes.V0044Node{
							{
								V0044Node: slurmapi.V0044Node{
									Name: ptr.To(nodesetutils.GetSlurmNodeName(pod)),
									State: ptr.To([]slurmapi.V0044NodeState{
										slurmapi.V0044NodeStateIDLE,
										slurmapi.V0044NodeStateDRAIN,
									}),
								},
							},
						},
					}
					sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.syncPodUncordon(tt.args.ctx, tt.args.nodeset, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("syncPodUncordon() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_syncSlurmTopology(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node0",
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node1",
			Annotations: map[string]string{
				slinkyv1beta1.AnnotationNodeTopologySpec: "topo-block:b0",
			},
		},
	}
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 2)
	pod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 0, "")
	pod2 := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), nodeset, controller, 1, "")
	pod2.Spec.NodeName = node2.Name

	tests := []struct {
		name      string
		client    client.Client
		clientMap *clientmap.ClientMap
		nodeset   *slinkyv1beta1.NodeSet
		pods      []*corev1.Pod
		wantErr   bool
	}{
		{
			name:      "pending",
			client:    fake.NewFakeClient(node.DeepCopy(), node2.DeepCopy(), pod.DeepCopy()),
			clientMap: newClientMap(controller.Name, newFakeClientList(sinterceptor.Funcs{})),
			nodeset:   nodeset,
			pods:      []*corev1.Pod{pod.DeepCopy()},
		},
		{
			name:   "allocated",
			client: fake.NewFakeClient(node.DeepCopy(), node2.DeepCopy(), pod2.DeepCopy()),
			clientMap: func() *clientmap.ClientMap {
				nodeList := &slurmtypes.V0044NodeList{
					Items: []slurmtypes.V0044Node{
						{
							V0044Node: slurmapi.V0044Node{
								Name: ptr.To(nodesetutils.GetSlurmNodeName(pod2)),
								State: ptr.To([]slurmapi.V0044NodeState{
									slurmapi.V0044NodeStateIDLE,
								}),
							},
						},
					},
				}
				sclient := newFakeClientList(sinterceptor.Funcs{}, nodeList)
				return newClientMap(controller.Name, sclient)
			}(),
			nodeset: nodeset,
			pods:    []*corev1.Pod{pod2.DeepCopy()},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			r := newNodeSetController(tt.client, tt.clientMap)
			gotErr := r.syncSlurmTopology(context.Background(), tt.nodeset, tt.pods)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("syncSlurmTopology() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("syncSlurmTopology() succeeded unexpectedly")
			}
			for _, pod := range tt.pods {
				checkPod := &corev1.Pod{}
				if err := tt.client.Get(ctx, client.ObjectKeyFromObject(pod), checkPod); err != nil {
					t.Errorf("Get() failed: %v", err)
				}
				if pod.Spec.NodeName == "" {
					continue
				}
				checkNode := &corev1.Node{}
				checkNodeKey := types.NamespacedName{Name: pod.Spec.NodeName}
				if err := tt.client.Get(ctx, checkNodeKey, checkNode); err != nil {
					t.Errorf("Get() failed: %v", err)
				}
				topologySpec := checkNode.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec]
				if !apiequality.Semantic.DeepEqual(checkPod.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec], topologySpec) {
					t.Errorf("pod and node topology are incongruent: node = '%v' ; pod = '%v'", topologySpec, checkPod.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec])
				}
				sclient := tt.clientMap.Get(tt.nodeset.Spec.ControllerRef.NamespacedName())
				if sclient == nil {
					continue
				}
				slurmNode := &slurmtypes.V0044Node{}
				slurmNodeKey := slurmclient.ObjectKey(nodesetutils.GetSlurmNodeName(pod))
				if err := sclient.Get(ctx, slurmNodeKey, slurmNode); err != nil {
					t.Errorf("Get() failed: %v", err)
				}
				if !apiequality.Semantic.DeepEqual(topologySpec, ptr.Deref(slurmNode.Topology, "")) {
					t.Errorf("Kube node and Slurm node topology are incongruent: Kube node = '%v' ; slurm node = '%v'", topologySpec, ptr.Deref(slurmNode.Topology, ""))
				}
			}
		})
	}
}

func TestGetNodesToDaemonPods(t *testing.T) {
	nodeset := newNodeSet("foo", "ctrl", 1)
	nodeset.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset
	nodeset2 := newNodeSet("foo2", "ctrl", 1)
	nodeset2.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset

	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = slinkyv1beta1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	r := newNodeSetController(client, nil)

	cases := map[string]struct {
		includeDeletedTerminal bool
		pods                   []*corev1.Pod
		expectedPodNames       []string
	}{
		"exclude deleted terminal pods": {
			pods: []*corev1.Pod{
				newDaemonPodForNodeSet("matching-owned-0", "node-0", nodeset),
				newDaemonPodForNodeSet("matching-orphan-0", "node-0", nil),
				newDaemonPodForNodeSet("matching-owned-1", "node-1", nodeset),
				newDaemonPodForNodeSet("matching-orphan-1", "node-1", nil),
				func() *corev1.Pod {
					pod := newDaemonPodForNodeSet("matching-owned-succeeded-pod-0", "node-0", nodeset)
					pod.Status = corev1.PodStatus{Phase: corev1.PodSucceeded}
					return pod
				}(),
				func() *corev1.Pod {
					pod := newDaemonPodForNodeSet("matching-owned-failed-pod-1", "node-1", nodeset)
					pod.Status = corev1.PodStatus{Phase: corev1.PodFailed}
					return pod
				}(),
				func() *corev1.Pod {
					pod := newDaemonPodForNodeSet("matching-owned-succeeded-deleted-pod-0", "node-0", nodeset)
					now := metav1.Now()
					pod.DeletionTimestamp = &now
					pod.Status = corev1.PodStatus{Phase: corev1.PodSucceeded}
					return pod
				}(),
				func() *corev1.Pod {
					pod := newDaemonPodForNodeSet("matching-owned-failed-deleted-pod-1", "node-1", nodeset)
					now := metav1.Now()
					pod.DeletionTimestamp = &now
					pod.Status = corev1.PodStatus{Phase: corev1.PodFailed}
					return pod
				}(),
			},
			expectedPodNames: []string{
				"matching-owned-0", "matching-orphan-0", "matching-owned-1", "matching-orphan-1",
				"matching-owned-succeeded-pod-0", "matching-owned-failed-pod-1",
			},
		},
		"include deleted terminal pods": {
			includeDeletedTerminal: true,
			pods: []*corev1.Pod{
				newDaemonPodForNodeSet("matching-owned-0", "node-0", nodeset),
				newDaemonPodForNodeSet("matching-orphan-0", "node-0", nil),
				newDaemonPodForNodeSet("matching-owned-1", "node-1", nodeset),
				newDaemonPodForNodeSet("matching-orphan-1", "node-1", nil),
				func() *corev1.Pod {
					pod := newDaemonPodForNodeSet("matching-owned-succeeded-pod-0", "node-0", nodeset)
					pod.Status = corev1.PodStatus{Phase: corev1.PodSucceeded}
					return pod
				}(),
				func() *corev1.Pod {
					pod := newDaemonPodForNodeSet("matching-owned-failed-deleted-pod-1", "node-1", nodeset)
					now := metav1.Now()
					pod.DeletionTimestamp = &now
					pod.Status = corev1.PodStatus{Phase: corev1.PodFailed}
					return pod
				}(),
			},
			expectedPodNames: []string{
				"matching-owned-0", "matching-orphan-0", "matching-owned-1", "matching-orphan-1",
				"matching-owned-succeeded-pod-0", "matching-owned-failed-deleted-pod-1",
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			nodesToDaemonPods := r.getNodesToDaemonPods(ctx, nodeset, tc.pods, tc.includeDeletedTerminal)
			gotPods := map[string]bool{}
			for node, pods := range nodesToDaemonPods {
				for _, pod := range pods {
					if pod.Spec.NodeName != node {
						t.Errorf("pod %v grouped into %v but belongs in %v", pod.Name, node, pod.Spec.NodeName)
					}
					gotPods[pod.Name] = true
				}
			}
			for _, wantName := range tc.expectedPodNames {
				if !gotPods[wantName] {
					t.Errorf("expected pod %v but didn't get it", wantName)
				}
				delete(gotPods, wantName)
			}
			for podName := range gotPods {
				t.Errorf("unexpected pod %v was returned", podName)
			}
		})
	}
}

// newNodeForNodeSetTest returns a node for NodeShouldRunDaemonPod tests.
func newNodeForNodeSetTest(name string, labels map[string]string, unschedulable bool) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceMemory: *resource.NewQuantity(100*1024*1024, resource.BinarySI),
				corev1.ResourceCPU:    *resource.NewMilliQuantity(1000, resource.DecimalSI),
			},
		},
		Spec: corev1.NodeSpec{
			Unschedulable: unschedulable,
		},
	}
	return node
}

func TestNodeShouldRunDaemonPod(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "slurm",
			Namespace: corev1.NamespaceDefault,
		},
	}
	sch := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(sch)
	_ = slinkyv1beta1.AddToScheme(sch)

	// NodeSet with no extra constraints (template has default pod spec from newNodeSet).
	nodeSetBasic := newNodeSet("foo", controller.Name, 1)
	nodeSetBasic.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset

	// NodeSet with NodeSelector that does not match node (node has type=production, selector wants type=test).
	nodeSetNodeSelectorMismatch := newNodeSet("foo", controller.Name, 1)
	nodeSetNodeSelectorMismatch.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset
	nodeSetNodeSelectorMismatch.Spec.Template.PodSpecWrapper.NodeSelector = map[string]string{"type": "test"}

	// NodeSet with NodeSelector that matches node (type=production).
	nodeSetNodeSelectorMatch := newNodeSet("foo", controller.Name, 1)
	nodeSetNodeSelectorMatch.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset
	nodeSetNodeSelectorMatch.Spec.Template.PodSpecWrapper.NodeSelector = map[string]string{"type": "production"}

	// NodeSet with NodeAffinity required type=production -> matches node with type=production.
	nodeSetAffinityMatch := newNodeSet("foo", controller.Name, 1)
	nodeSetAffinityMatch.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset
	nodeSetAffinityMatch.Spec.Template.PodSpecWrapper.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      "type",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"production"},
					}},
				}},
			},
		},
	}

	// NodeSet with NodeAffinity required type=production -> does NOT match node with type=staging.
	nodeSetAffinityMismatch := newNodeSet("foo", controller.Name, 1)
	nodeSetAffinityMismatch.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset
	nodeSetAffinityMismatch.Spec.Template.PodSpecWrapper.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      "type",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"production"},
					}},
				}},
			},
		},
	}

	cases := []struct {
		predicateName                    string
		node                             *corev1.Node
		nodeset                          *slinkyv1beta1.NodeSet
		shouldRun, shouldContinueRunning bool
	}{
		{
			predicateName:         "ShouldRunDaemonPod",
			node:                  newNodeForNodeSetTest("test-node", map[string]string{"type": "production"}, false),
			nodeset:               nodeSetBasic,
			shouldRun:             true,
			shouldContinueRunning: true,
		},
		{
			predicateName:         "ErrNodeSelectorNotMatch",
			node:                  newNodeForNodeSetTest("test-node", map[string]string{"type": "production"}, false),
			nodeset:               nodeSetNodeSelectorMismatch,
			shouldRun:             false,
			shouldContinueRunning: false,
		},
		{
			predicateName:         "ShouldRunDaemonPod_NodeSelectorMatch",
			node:                  newNodeForNodeSetTest("test-node", map[string]string{"type": "production"}, false),
			nodeset:               nodeSetNodeSelectorMatch,
			shouldRun:             true,
			shouldContinueRunning: true,
		},
		{
			predicateName:         "ShouldRunDaemonPod_NodeAffinityMatch",
			node:                  newNodeForNodeSetTest("test-node", map[string]string{"type": "production"}, false),
			nodeset:               nodeSetAffinityMatch,
			shouldRun:             true,
			shouldContinueRunning: true,
		},
		{
			predicateName:         "ShouldRunDaemonPodOnUnschedulableNode",
			node:                  newNodeForNodeSetTest("test-node", map[string]string{"type": "production"}, true),
			nodeset:               nodeSetBasic,
			shouldRun:             true,
			shouldContinueRunning: true,
		},
		{
			predicateName:         "ErrNodeAffinityNotMatch",
			node:                  newNodeForNodeSetTest("test-node", map[string]string{"type": "staging"}, false),
			nodeset:               nodeSetAffinityMismatch,
			shouldRun:             false,
			shouldContinueRunning: false,
		},
	}

	for _, c := range cases {
		t.Run(c.predicateName, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewClientBuilder().
				WithScheme(sch).
				WithObjects(controller).
				Build()
			r := newNodeSetController(client, nil)

			shouldRun, shouldContinueRunning := r.NodeShouldRunDaemonPod(ctx, c.node, c.nodeset)
			if shouldRun != c.shouldRun {
				t.Errorf("NodeShouldRunDaemonPod(): predicateName: %v expected shouldRun: %v, got: %v", c.predicateName, c.shouldRun, shouldRun)
			}
			if shouldContinueRunning != c.shouldContinueRunning {
				t.Errorf("NodeShouldRunDaemonPod(): predicateName: %v expected shouldContinueRunning: %v, got: %v", c.predicateName, c.shouldContinueRunning, shouldContinueRunning)
			}
		})
	}
}

func TestNodeSetReconciler_podsShouldBeOnNode(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "slurm",
			Namespace: corev1.NamespaceDefault,
		},
	}
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = slinkyv1beta1.AddToScheme(scheme)

	nodesetBasic := newNodeSet("foo", controller.Name, 1)
	nodesetBasic.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset

	nodesetNodeSelectorMismatch := newNodeSet("bar", controller.Name, 1)
	nodesetNodeSelectorMismatch.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset
	nodesetNodeSelectorMismatch.Spec.Template.PodSpecWrapper.NodeSelector = map[string]string{"type": "test"}

	nodeReady := newNodeForNodeSetTest("node-ready", map[string]string{"type": "production"}, false)
	nodeMismatch := newNodeForNodeSetTest("node-mismatch", map[string]string{"type": "production"}, false)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(controller).
		Build()
	r := newNodeSetController(client, nil)

	ctx := context.Background()

	tests := []struct {
		name                      string
		node                      *corev1.Node
		nodeset                   *slinkyv1beta1.NodeSet
		nodeToDaemonPods          map[string][]*corev1.Pod
		expectedNeedingDaemonPods []string
		expectedPodNamesToDelete  []string
	}{
		{
			name:                      "Node should run daemon pod but no pod exists",
			node:                      nodeReady,
			nodeset:                   nodesetBasic,
			nodeToDaemonPods:          map[string][]*corev1.Pod{},
			expectedNeedingDaemonPods: []string{"node-ready"},
			expectedPodNamesToDelete:  nil,
		},
		{
			name:    "Node should run and has one running pod",
			node:    nodeReady,
			nodeset: nodesetBasic,
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"node-ready": {newDaemonPodForNodeSet("pod-1", "node-ready", nodesetBasic)},
			},
			expectedNeedingDaemonPods: nil,
			expectedPodNamesToDelete:  nil,
		},
		{
			name:    "Node should run and has failed pod",
			node:    nodeReady,
			nodeset: nodesetBasic,
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"node-ready": func() []*corev1.Pod {
					pod := newDaemonPodForNodeSet("failed-pod", "node-ready", nodesetBasic)
					pod.Status.Phase = corev1.PodFailed
					return []*corev1.Pod{pod}
				}(),
			},
			expectedNeedingDaemonPods: nil,
			expectedPodNamesToDelete:  []string{"failed-pod"},
		},
		{
			name:    "Node should run and has succeeded pod",
			node:    nodeReady,
			nodeset: nodesetBasic,
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"node-ready": func() []*corev1.Pod {
					pod := newDaemonPodForNodeSet("succeeded-pod", "node-ready", nodesetBasic)
					pod.Status.Phase = corev1.PodSucceeded
					return []*corev1.Pod{pod}
				}(),
			},
			expectedNeedingDaemonPods: nil,
			expectedPodNamesToDelete:  []string{"succeeded-pod"},
		},
		{
			name:    "Node should run and has multiple running pods, prunes to oldest",
			node:    nodeReady,
			nodeset: nodesetBasic,
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"node-ready": func() []*corev1.Pod {
					p1 := newDaemonPodForNodeSet("old-pod", "node-ready", nodesetBasic)
					p1.Status = corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Unix(1, 0))},
						},
					}
					p1.CreationTimestamp = metav1.NewTime(time.Unix(1, 0))
					p2 := newDaemonPodForNodeSet("new-pod", "node-ready", nodesetBasic)
					p2.Status = corev1.PodStatus{
						Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{
							{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(time.Unix(2, 0))},
						},
					}
					p2.CreationTimestamp = metav1.NewTime(time.Unix(2, 0))
					return []*corev1.Pod{p1, p2}
				}(),
			},
			expectedNeedingDaemonPods: nil,
			expectedPodNamesToDelete:  []string{"old-pod"},
		},
		{
			name:    "Node should not run (selector mismatch) but has pods, delete all",
			node:    nodeMismatch,
			nodeset: nodesetNodeSelectorMismatch,
			nodeToDaemonPods: map[string][]*corev1.Pod{
				"node-mismatch": {newDaemonPodForNodeSet("pod-tainted", "node-mismatch", nodesetNodeSelectorMismatch)},
			},
			expectedNeedingDaemonPods: nil,
			expectedPodNamesToDelete:  []string{"pod-tainted"},
		},
		{
			name:                      "Node should not run and no pods",
			node:                      nodeMismatch,
			nodeset:                   nodesetNodeSelectorMismatch,
			nodeToDaemonPods:          map[string][]*corev1.Pod{},
			expectedNeedingDaemonPods: nil,
			expectedPodNamesToDelete:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needing, toDelete := r.podsShouldBeOnNode(ctx, tt.node, tt.nodeToDaemonPods, tt.nodeset)
			if !slices.Equal(needing, tt.expectedNeedingDaemonPods) {
				t.Errorf("nodesNeedingDaemonPods = %v, want %v", needing, tt.expectedNeedingDaemonPods)
			}
			gotNames := make([]string, 0, len(toDelete))
			for _, p := range toDelete {
				gotNames = append(gotNames, p.Name)
			}
			if !slices.Equal(gotNames, tt.expectedPodNamesToDelete) {
				t.Errorf("podsToDelete names = %v, want %v", gotNames, tt.expectedPodNamesToDelete)
			}
		})
	}
}

func TestNodeSetReconciler_syncSlurmNodes(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	type testCase struct {
		name      string
		kclient   client.Client
		clientMap *clientmap.ClientMap
		nodeset   *slinkyv1beta1.NodeSet
		pods      []*corev1.Pod
		wantOk    bool
		wantErr   bool
	}
	tests := []testCase{
		{
			name:      "empty",
			kclient:   fake.NewFakeClient(),
			clientMap: newClientMap(controller.Name, newFakeClientList(sinterceptor.Funcs{})),
			nodeset:   newNodeSet("foo", controller.Name, 2),
		},
		func() testCase {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pod0 := newNodeSetPodWithStatus(nodeset, controller, 0, corev1.PodRunning, []corev1.PodConditionType{corev1.PodReady})
			pod1 := newNodeSetPodWithStatus(nodeset, controller, 1, corev1.PodRunning, []corev1.PodConditionType{corev1.PodReady})
			kclient := fake.NewFakeClient(nodeset, pod0, pod1)
			sclient := newFakeClientList(sinterceptor.Funcs{}, &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{Name: ptr.To(nodesetutils.GetSlurmNodeName(pod0))}},
					{V0044Node: slurmapi.V0044Node{Name: ptr.To(nodesetutils.GetSlurmNodeName(pod1))}},
				},
			})
			clientMap := newClientMap(controller.Name, sclient)
			return testCase{
				name:      "all are registered",
				kclient:   kclient,
				clientMap: clientMap,
				nodeset:   nodeset,
				pods: []*corev1.Pod{
					pod0,
					pod1,
				},
				wantOk: true,
			}
		}(),
		func() testCase {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pod0 := newNodeSetPodWithStatus(nodeset, controller, 0, corev1.PodRunning, []corev1.PodConditionType{corev1.PodReady})
			pod1 := newNodeSetPodWithStatus(nodeset, controller, 1, corev1.PodRunning, []corev1.PodConditionType{corev1.PodReady})
			kclient := fake.NewFakeClient(nodeset, pod0, pod1)
			sclient := newFakeClientList(sinterceptor.Funcs{}, &slurmtypes.V0044NodeList{
				Items: []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{Name: ptr.To(nodesetutils.GetSlurmNodeName(pod0))}},
					// {V0044Node: slurmapi.V0044Node{Name: ptr.To(nodesetutils.GetSlurmNodeName(pod1))}}, // unregistered
				},
			})
			clientMap := newClientMap(controller.Name, sclient)
			return testCase{
				name:      "one unregistered pod",
				kclient:   kclient,
				clientMap: clientMap,
				nodeset:   nodeset,
				pods: []*corev1.Pod{
					pod0,
					pod1,
				},
				wantOk: true,
			}
		}(),
		func() testCase {
			nodeset := newNodeSet("foo", controller.Name, 2)
			pod0 := newNodeSetPodWithStatus(nodeset, controller, 0, corev1.PodRunning, []corev1.PodConditionType{corev1.PodReady})
			pod1 := newNodeSetPodWithStatus(nodeset, controller, 1, corev1.PodRunning, []corev1.PodConditionType{corev1.PodReady})
			return testCase{
				name:      "no client",
				kclient:   fake.NewFakeClient(nodeset, pod0, pod1),
				clientMap: clientmap.NewClientMap(),
				nodeset:   nodeset,
				pods: []*corev1.Pod{
					pod0,
					pod1,
				},
				wantOk: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			r := NewReconciler(tt.kclient, tt.clientMap, nil)
			gotErr := r.syncSlurmNodes(context.Background(), tt.nodeset, tt.pods)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("syncSlurmNodes() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("syncSlurmNodes() succeeded unexpectedly")
			}
			scontrol := slurmcontrol.NewSlurmControl(tt.clientMap)
			slurmNodeNames, ok, err := scontrol.GetNodesForPods(ctx, tt.nodeset, tt.pods)
			if err != nil {
				t.Fatalf("slurmControl failed to get Slurm node names: %v", err)
			}
			if !ok {
				if ok != tt.wantOk {
					t.Fatal("slurmControl used a client unexpectedly")
				}
				return
			}
			podList := &corev1.PodList{}
			if err := tt.kclient.List(ctx, podList); err != nil {
				t.Fatalf("kclient failed to list pods: %v", err)
			}
			if len(podList.Items) != len(slurmNodeNames) {
				t.Errorf("syncSlurmNodes() unregistered Slurm node but healthy pod was not deleted")
			}
			slurmNodeNameSet := set.New(slurmNodeNames...)
			for _, pod := range podList.Items {
				slurmNodeName := nodesetutils.GetSlurmNodeName(&pod)
				if !slurmNodeNameSet.Has(slurmNodeName) {
					t.Errorf("syncSlurmNodes() unexpected pod exists: %v", slurmNodeName)
				}
			}
		})
	}
}

func TestNodeSetReconciler_syncDefunctNodes(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}

	defunctNodeState := ptr.To([]slurmapi.V0044NodeState{
		slurmapi.V0044NodeStateDOWN,
		slurmapi.V0044NodeStateNOTRESPONDING,
	})

	tests := []struct {
		name         string
		scalingMode  slinkyv1beta1.ScalingModeType
		pruneDefunct bool
		// setup returns extra kubernetes objects to seed (pods, nodes) and the
		// Slurm nodes to seed on the fake Slurm client, plus the expected state
		// of Slurm node names after syncDefunctNodes runs.
		setup func(ns *slinkyv1beta1.NodeSet) (kubeObjs []runtime.Object, slurmNodes []slurmtypes.V0044Node, stillExist, pruned []string)
		// interceptor lets a test case inject Slurm client behavior — e.g. make
		// Delete fail to simulate a reservation conflict.
		interceptor sinterceptor.Funcs
	}{
		{
			name:         "prunes defunct ghost node but keeps node backed by running pod",
			scalingMode:  slinkyv1beta1.ScalingModeDaemonset,
			pruneDefunct: true,
			setup: func(ns *slinkyv1beta1.NodeSet) ([]runtime.Object, []slurmtypes.V0044Node, []string, []string) {
				pod := newNodeSetPodWithStatus(ns, controller, 0, corev1.PodRunning, []corev1.PodConditionType{corev1.PodReady})
				pod.Spec.Hostname = "worker-b"
				if pod.Labels == nil {
					pod.Labels = make(map[string]string)
				}
				pod.Labels[slinkyv1beta1.LabelNodeSetScalingMode] = string(slinkyv1beta1.ScalingModeDaemonset)
				kubeNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-b"}}
				existingSlurmName := nodesetutils.GetSlurmNodeName(pod)
				defunctPodName := nodesetutils.GetOrdinalPodName(ns, 1)
				nodes := []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{
						Name:  ptr.To("foo-ghost"),
						State: defunctNodeState,
						Comment: ptr.To((&podinfo.PodInfo{
							Namespace: corev1.NamespaceDefault,
							PodName:   defunctPodName,
							Node:      "worker-a",
						}).ToString()),
					}},
					{V0044Node: slurmapi.V0044Node{
						Name:  ptr.To(existingSlurmName),
						State: defunctNodeState,
						Comment: ptr.To((&podinfo.PodInfo{
							Namespace: corev1.NamespaceDefault,
							PodName:   pod.Name,
							Node:      "worker-b",
						}).ToString()),
					}},
				}
				return []runtime.Object{pod, kubeNode}, nodes, []string{existingSlurmName}, []string{"foo-ghost"}
			},
		},
		{
			name:         "skips when kube node still maps to slurm node by default",
			scalingMode:  slinkyv1beta1.ScalingModeDaemonset,
			pruneDefunct: true,
			setup: func(ns *slinkyv1beta1.NodeSet) ([]runtime.Object, []slurmtypes.V0044Node, []string, []string) {
				ghostPodName := nodesetutils.GetOrdinalPodName(ns, 1)
				kubeNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-a"}}
				nodes := []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{
						Name:  ptr.To("worker-a"),
						State: defunctNodeState,
						Comment: ptr.To((&podinfo.PodInfo{
							Namespace: corev1.NamespaceDefault,
							PodName:   ghostPodName,
							Node:      "worker-a",
						}).ToString()),
					}},
				}
				return []runtime.Object{kubeNode}, nodes, []string{"worker-a"}, nil
			},
		},
		{
			name:         "skips when kube node override still maps to slurm node",
			scalingMode:  slinkyv1beta1.ScalingModeDaemonset,
			pruneDefunct: true,
			setup: func(ns *slinkyv1beta1.NodeSet) ([]runtime.Object, []slurmtypes.V0044Node, []string, []string) {
				ghostPodName := nodesetutils.GetOrdinalPodName(ns, 1)
				kubeNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-a",
						Annotations: map[string]string{
							slinkyv1beta1.AnnotationNodeHostnameOverride: "slurm-node-x",
						},
					},
				}
				nodes := []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{
						Name:  ptr.To("slurm-node-x"),
						State: defunctNodeState,
						Comment: ptr.To((&podinfo.PodInfo{
							Namespace: corev1.NamespaceDefault,
							PodName:   ghostPodName,
							Node:      "worker-a",
						}).ToString()),
					}},
				}
				return []runtime.Object{kubeNode}, nodes, []string{"slurm-node-x"}, nil
			},
		},
		{
			name:         "prunes when kube node override no longer maps to slurm node",
			scalingMode:  slinkyv1beta1.ScalingModeDaemonset,
			pruneDefunct: true,
			setup: func(ns *slinkyv1beta1.NodeSet) ([]runtime.Object, []slurmtypes.V0044Node, []string, []string) {
				ghostPodName := nodesetutils.GetOrdinalPodName(ns, 1)
				kubeNode := &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "worker-a",
						Annotations: map[string]string{
							slinkyv1beta1.AnnotationNodeHostnameOverride: "new-name",
						},
					},
				}
				nodes := []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{
						Name:  ptr.To("stale-name"),
						State: defunctNodeState,
						Comment: ptr.To((&podinfo.PodInfo{
							Namespace: corev1.NamespaceDefault,
							PodName:   ghostPodName,
							Node:      "worker-a",
						}).ToString()),
					}},
				}
				return []runtime.Object{kubeNode}, nodes, nil, []string{"stale-name"}
			},
		},
		{
			name:         "skips when prune gate disabled",
			scalingMode:  slinkyv1beta1.ScalingModeDaemonset,
			pruneDefunct: false,
			setup: func(ns *slinkyv1beta1.NodeSet) ([]runtime.Object, []slurmtypes.V0044Node, []string, []string) {
				defunctPodName := nodesetutils.GetOrdinalPodName(ns, 1)
				nodes := []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{
						Name:  ptr.To("foo-ghost"),
						State: defunctNodeState,
						Comment: ptr.To((&podinfo.PodInfo{
							Namespace: corev1.NamespaceDefault,
							PodName:   defunctPodName,
							Node:      "worker-a",
						}).ToString()),
					}},
				}
				return nil, nodes, []string{"foo-ghost"}, nil
			},
		},
		{
			name:         "skips for statefulset scaling mode",
			scalingMode:  "",
			pruneDefunct: true,
			setup: func(ns *slinkyv1beta1.NodeSet) ([]runtime.Object, []slurmtypes.V0044Node, []string, []string) {
				defunctPodName := nodesetutils.GetOrdinalPodName(ns, 1)
				nodes := []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{
						Name:  ptr.To("foo-ghost"),
						State: defunctNodeState,
						Comment: ptr.To((&podinfo.PodInfo{
							Namespace: corev1.NamespaceDefault,
							PodName:   defunctPodName,
							Node:      "worker-a",
						}).ToString()),
					}},
				}
				return nil, nodes, []string{"foo-ghost"}, nil
			},
		},
		{
			name:         "logs but does not error when slurm delete fails (e.g. reservation conflict)",
			scalingMode:  slinkyv1beta1.ScalingModeDaemonset,
			pruneDefunct: true,
			setup: func(ns *slinkyv1beta1.NodeSet) ([]runtime.Object, []slurmtypes.V0044Node, []string, []string) {
				defunctPodName := nodesetutils.GetOrdinalPodName(ns, 1)
				// Pod and kube node are gone — prune would normally proceed, but
				// the injected interceptor makes Slurm reject the delete (mimicking
				// a reservation conflict). The controller must log and continue,
				// not return an error and hot-loop reconciles.
				nodes := []slurmtypes.V0044Node{
					{V0044Node: slurmapi.V0044Node{
						Name:  ptr.To("reserved-node"),
						State: defunctNodeState,
						Comment: ptr.To((&podinfo.PodInfo{
							Namespace: corev1.NamespaceDefault,
							PodName:   defunctPodName,
							Node:      "worker-a",
						}).ToString()),
					}},
				}
				return nil, nodes, []string{"reserved-node"}, nil
			},
			interceptor: sinterceptor.Funcs{
				Delete: func(_ context.Context, _ slurmobject.Object, _ ...slurmclient.DeleteOption) error {
					return errors.New("Node used by active reservation")
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeset := newNodeSet("foo", controller.Name, 1)
			if tt.scalingMode != "" {
				nodeset.Spec.ScalingMode = tt.scalingMode
			}
			nodeset.Spec.PruneDefunctSlurmNodes = tt.pruneDefunct

			kubeObjs, slurmNodes, stillExist, pruned := tt.setup(nodeset)
			initObjs := append([]runtime.Object{nodeset}, kubeObjs...)

			kclient := fake.NewFakeClient(initObjs...)
			sclient := newFakeClientList(tt.interceptor, &slurmtypes.V0044NodeList{Items: slurmNodes})
			clientMap := newClientMap(controller.Name, sclient)
			r := NewReconciler(kclient, clientMap, nil)

			if err := r.syncDefunctNodes(context.Background(), nodeset); err != nil {
				t.Fatalf("syncDefunctNodes() error = %v", err)
			}

			for _, name := range stillExist {
				if err := sclient.Get(context.Background(), slurmclient.ObjectKey(name), &slurmtypes.V0044Node{}); err != nil {
					t.Fatalf("expected Slurm node %q to remain, got: %v", name, err)
				}
			}
			for _, name := range pruned {
				if err := sclient.Get(context.Background(), slurmclient.ObjectKey(name), &slurmtypes.V0044Node{}); err == nil {
					t.Fatalf("expected Slurm node %q to be pruned, it still exists", name)
				}
			}
		})
	}
}
