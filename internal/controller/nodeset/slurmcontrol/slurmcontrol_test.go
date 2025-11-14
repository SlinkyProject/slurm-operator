// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/puttsk/hostlist"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"k8s.io/utils/set"

	api "github.com/SlinkyProject/slurm-client/api/v0044"
	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/client/fake"
	"github.com/SlinkyProject/slurm-client/pkg/object"
	"github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podinfo"
	slurmconditions "github.com/SlinkyProject/slurm-operator/pkg/conditions"
)

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
			Replicas: &replicas,
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

var _ = Describe("SlurmControlInterface", func() {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	var slurmcontrol SlurmControlInterface
	var nodeset *slinkyv1beta1.NodeSet
	var pod *corev1.Pod
	var sclient client.Client

	updateFn := func(_ context.Context, obj object.Object, req any, opts ...client.UpdateOption) error {
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
		default:
			return errors.New("failed to cast slurm object")
		}
		return nil
	}

	Context("UpdateNodeWithPodInfo()", func() {
		It("Should update node comment with podInfo", func() {
			By("Setup initial system state")
			nodeset = newNodeSet("foo", controller.Name, 1)
			pod = nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
			slurmNodename := nodesetutils.GetNodeName(pod)
			node := &types.V0044Node{
				V0044Node: api.V0044Node{
					Name: ptr.To(slurmNodename),
					State: ptr.To([]api.V0044NodeState{
						api.V0044NodeStateIDLE,
					}),
				},
			}
			sclient = fake.NewClientBuilder().WithUpdateFn(updateFn).WithObjects(node).Build()
			controllers := newSlurmClientMap(controller.Name, sclient)
			slurmcontrol = NewSlurmControl(controllers)

			By("Update Slurm pod info")
			err := slurmcontrol.UpdateNodeWithPodInfo(ctx, nodeset, pod)
			Expect(err).ToNot(HaveOccurred())

			By("Check Slurm Node podInfo")
			wantPodInfo := podinfo.PodInfo{
				Namespace: pod.GetNamespace(),
				PodName:   pod.GetName(),
				Node:      pod.Spec.NodeName,
			}
			checkNode := &types.V0044Node{}
			key := object.ObjectKey(slurmNodename)
			err = sclient.Get(ctx, key, checkNode)
			Expect(err).ToNot(HaveOccurred())
			checkPodInfo := podinfo.PodInfo{}
			err = podinfo.ParseIntoPodInfo(checkNode.Comment, &checkPodInfo)
			Expect(err).ToNot(HaveOccurred())
			Expect(checkPodInfo.Equal(wantPodInfo)).To(BeTrue())
		})
	})

	Context("MakeNodeDrain()", func() {
		It("Should DRAIN the IDLE Slurm node", func() {
			By("Setup initial system state")
			nodeset = newNodeSet("foo", controller.Name, 1)
			pod = nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
			slurmNodename := nodesetutils.GetNodeName(pod)
			node := &types.V0044Node{
				V0044Node: api.V0044Node{
					Name: ptr.To(slurmNodename),
					State: ptr.To([]api.V0044NodeState{
						api.V0044NodeStateIDLE,
					}),
				},
			}
			sclient = fake.NewClientBuilder().WithUpdateFn(updateFn).WithObjects(node).Build()
			controllers := newSlurmClientMap(controller.Name, sclient)
			slurmcontrol = NewSlurmControl(controllers)

			By("Draining matching Slurm node")
			err := slurmcontrol.MakeNodeDrain(ctx, nodeset, pod, "drain")
			Expect(err).ToNot(HaveOccurred())

			By("Check Slurm Node state")
			checkNode := &types.V0044Node{}
			key := object.ObjectKey(slurmNodename)
			err = sclient.Get(ctx, key, checkNode)
			Expect(err).ToNot(HaveOccurred())
			isDrain := checkNode.GetStateAsSet().Has(api.V0044NodeStateDRAIN)
			Expect(isDrain).To(BeTrue())
		})
	})

	Context("UpdateNodeWithPodInfo()", func() {
		It("Should reset Slurm node state when podInfo indicates migration to a new Kube node", func() {
			By("Setup initial system state")
			nodeset = newNodeSet("foo", controller.Name, 1)
			pod = nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
			slurmNodename := nodesetutils.GetNodeName(pod)
			node := &types.V0044Node{
				V0044Node: api.V0044Node{
					Name: ptr.To(slurmNodename),
					State: ptr.To([]api.V0044NodeState{
						api.V0044NodeStateIDLE,
					}),
				},
			}
			sclient = fake.NewClientBuilder().WithUpdateFn(updateFn).WithObjects(node).Build()
			controllers := newSlurmClientMap(controller.Name, sclient)
			slurmcontrol = NewSlurmControl(controllers)

			By("Update Slurm pod info")
			err := slurmcontrol.UpdateNodeWithPodInfo(ctx, nodeset, pod)
			Expect(err).ToNot(HaveOccurred())

			By("Check Slurm Node podInfo")
			wantPodInfo := podinfo.PodInfo{
				Namespace: pod.GetNamespace(),
				PodName:   pod.GetName(),
				Node:      pod.Spec.NodeName,
			}
			checkNode := &types.V0044Node{}
			key := object.ObjectKey(slurmNodename)
			err = sclient.Get(ctx, key, checkNode)
			Expect(err).ToNot(HaveOccurred())
			checkPodInfo := podinfo.PodInfo{}
			err = podinfo.ParseIntoPodInfo(checkNode.Comment, &checkPodInfo)
			Expect(err).ToNot(HaveOccurred())
			Expect(checkPodInfo.Equal(wantPodInfo)).To(BeTrue())

			By("Make node drain")
			err = slurmcontrol.MakeNodeDrain(ctx, nodeset, pod, "drain")
			Expect(err).ToNot(HaveOccurred())

			By("Migrate pod to a new Kube node")
			pod.Spec.NodeName = "bar"

			By("Update Slurm pod info")
			err = slurmcontrol.UpdateNodeWithPodInfo(ctx, nodeset, pod)
			Expect(err).ToNot(HaveOccurred())

			By("Check Slurm Node podInfo")
			wantPodInfo = podinfo.PodInfo{
				Namespace: pod.GetNamespace(),
				PodName:   pod.GetName(),
				Node:      pod.Spec.NodeName,
			}
			checkNode = &types.V0044Node{}
			key = object.ObjectKey(slurmNodename)
			err = sclient.Get(ctx, key, checkNode)
			Expect(err).ToNot(HaveOccurred())
			checkPodInfo = podinfo.PodInfo{}
			err = podinfo.ParseIntoPodInfo(checkNode.Comment, &checkPodInfo)
			Expect(err).ToNot(HaveOccurred())
			Expect(checkPodInfo.Equal(wantPodInfo)).To(BeTrue())

			By("Check Slurm Node state")
			checkNode = &types.V0044Node{}
			key = object.ObjectKey(slurmNodename)
			err = sclient.Get(ctx, key, checkNode)
			Expect(err).ToNot(HaveOccurred())
			isIdle := checkNode.GetStateAsSet().Has(api.V0044NodeStateIDLE)
			Expect(isIdle).To(BeTrue())
		})
	})

	Context("MakeNodeUndrain()", func() {
		It("Should UNDRAIN the IDLE Slurm node", func() {
			By("Setup initial system state")
			nodeset = newNodeSet("foo", controller.Name, 1)
			pod = nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
			node := &types.V0044Node{
				V0044Node: api.V0044Node{
					Name: ptr.To(nodesetutils.GetNodeName(pod)),
					State: ptr.To([]api.V0044NodeState{
						api.V0044NodeStateIDLE,
						api.V0044NodeStateDRAIN,
					}),
				},
			}
			sclient = fake.NewClientBuilder().WithUpdateFn(updateFn).WithObjects(node).Build()
			controllers := newSlurmClientMap(controller.Name, sclient)
			slurmcontrol = NewSlurmControl(controllers)

			By("Draining matching Slurm node")
			err := slurmcontrol.MakeNodeUndrain(ctx, nodeset, pod, "undrain")
			Expect(err).ToNot(HaveOccurred())

			By("Check Slurm Node state")
			checkNode := &types.V0044Node{}
			key := object.ObjectKey(nodesetutils.GetNodeName(pod))
			err = sclient.Get(ctx, key, checkNode)
			Expect(err).ToNot(HaveOccurred())
			isundrain := !checkNode.GetStateAsSet().Has(api.V0044NodeStateDRAIN)
			Expect(isundrain).To(BeTrue())
		})
	})

	Context("GetNodeDeadlines()", func() {
		now := time.Now()

		It("Should get completion time for jobs", func() {
			By("Setup initial system state")
			nodeset = newNodeSet("bar", controller.Name, 1)
			pod = nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
			pod2 := nodesetutils.NewNodeSetPod(nodeset, controller, 1, "")
			pods := []*corev1.Pod{pod, pod2}
			nodeList := &types.V0044NodeList{
				Items: []types.V0044Node{
					{
						V0044Node: api.V0044Node{
							Name: ptr.To(nodesetutils.GetNodeName(pod)),
							State: ptr.To([]api.V0044NodeState{
								api.V0044NodeStateMIXED,
							}),
						},
					},
					{
						V0044Node: api.V0044Node{
							Name: ptr.To(nodesetutils.GetNodeName(pod2)),
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
			sclient = fake.NewClientBuilder().WithLists(nodeList, jobList).Build()
			controllers := newSlurmClientMap(controller.Name, sclient)
			slurmcontrol = NewSlurmControl(controllers)

			By("Getting TimeStore")
			ts, err := slurmcontrol.GetNodeDeadlines(ctx, nodeset, pods)
			Expect(err).ToNot(HaveOccurred())

			By("Check TimeStore for Slurm Nodes")
			for _, node := range nodeList.Items {
				Expect(ts.Peek(*node.Name).After(now)).To(BeTrue())
			}
		})
	})
})

func Test_realSlurmControl_IsNodeDrain(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}
	nodeset := newNodeSet("foo", controller.Name, 1)
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
		{
			name: "No Slurm client - fail closed",
			fields: fields{
				clientMap: clientmap.NewClientMap(),
			},
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Node not found - fail closed",
			fields: func() fields {
				sclient := fake.NewClientBuilder().Build()
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realSlurmControl{
				clientMap: tt.fields.clientMap,
			}
			got, err := r.IsNodeDrain(tt.args.ctx, tt.args.nodeset, tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("realSlurmControl.IsNodeDrain() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("realSlurmControl.IsNodeDrain() = %v, want %v", got, tt.want)
			}
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
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
			name: "No Slurm client - fail closed",
			fields: fields{
				clientMap: clientmap.NewClientMap(),
			},
			args: args{
				ctx:     ctx,
				nodeset: nodeset,
				pod:     pod,
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "Node not found - fail closed",
			fields: func() fields {
				sclient := fake.NewClientBuilder().Build()
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realSlurmControl{
				clientMap: tt.fields.clientMap,
			}
			got, err := r.IsNodeDrained(tt.args.ctx, tt.args.nodeset, tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("realSlurmControl.IsNodeDrained() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("realSlurmControl.IsNodeDrained() = %v, want %v", got, tt.want)
			}
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
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
			if (err != nil) != tt.wantErr {
				t.Errorf("realSlurmControl.IsNodeDownForUnresponsive() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("realSlurmControl.IsNodeDownForUnresponsive() = %v, want %v", got, tt.want)
			}
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
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
						State: ptr.To([]api.V0044NodeState{
							api.V0044NodeStateDOWN,
						}),
						Reason: ptr.To(nodeReasonPrefix + " " + "foo"),
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
						Name: ptr.To(nodesetutils.GetNodeName(pod)),
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
			if (err != nil) != tt.wantErr {
				t.Errorf("realSlurmControl.IsNodeReasonOurs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("realSlurmControl.IsNodeReasonOurs() = %v, want %v", got, tt.want)
			}
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
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset2, controller, 0, ""))),
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
					nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 1,

				Idle: 1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))] = []corev1.PodCondition{
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
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))),
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
					nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 1,

				Idle: 1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))] = []corev1.PodCondition{
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
								Name:   ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))),
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
					nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""),
				},
			},
			want: SlurmNodeStatus{
				Total: 1,

				Idle:  1,
				Drain: 1,

				NodeStates: func() map[string][]corev1.PodCondition {
					nodeStates := make(map[string][]corev1.PodCondition)
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))] = []corev1.PodCondition{
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
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateALLOCATED,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateDOWN,
								}),
								Reason: ptr.To("Node is down"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateERROR,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateFUTURE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateMIXED,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""))),
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
					nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""),
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionAllocated,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionDown,
							Status:  corev1.ConditionTrue,
							Message: "Node is down",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionError,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionFuture,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionIdle,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionMixed,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""))] = []corev1.PodCondition{
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
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateCOMPLETING,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateDRAIN,
								}),
								Reason: ptr.To("Node set to drain"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateFAIL,
								}),
								Reason: ptr.To("Node set to fail"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateINVALID,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateINVALIDREG,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateMAINTENANCE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateNOTRESPONDING,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 7, ""))),
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
					nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 7, ""),
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionCompleting,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionDrain,
							Status:  corev1.ConditionTrue,
							Message: "Node set to drain",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionFail,
							Status:  corev1.ConditionTrue,
							Message: "Node set to fail",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionInvalid,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionInvalidReg,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionMaintenance,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""))] = []corev1.PodCondition{
						{
							Type:    slurmconditions.PodConditionNotResponding,
							Status:  corev1.ConditionTrue,
							Message: "",
						},
					}
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 7, ""))] = []corev1.PodCondition{
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
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateALLOCATED,
									api.V0044NodeStateCOMPLETING,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateDOWN,
									api.V0044NodeStateDRAIN,
								}),
								Reason: ptr.To("Node set to down and drain"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateERROR,
									api.V0044NodeStateFAIL,
								}),
								Reason: ptr.To("Node set to error and fail"),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateFUTURE,
									api.V0044NodeStateINVALID,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateFUTURE,
									api.V0044NodeStateINVALIDREG,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateIDLE,
									api.V0044NodeStateMAINTENANCE,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""))),
								State: ptr.To([]api.V0044NodeState{
									api.V0044NodeStateMIXED,
									api.V0044NodeStateNOTRESPONDING,
								}),
							},
						},
						{
							V0044Node: api.V0044Node{
								Name: ptr.To(nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 7, ""))),
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
					nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""),
					nodesetutils.NewNodeSetPod(nodeset, controller, 7, ""),
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 0, ""))] = []corev1.PodCondition{
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 1, ""))] = []corev1.PodCondition{
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 2, ""))] = []corev1.PodCondition{
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 3, ""))] = []corev1.PodCondition{
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 4, ""))] = []corev1.PodCondition{
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 5, ""))] = []corev1.PodCondition{
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 6, ""))] = []corev1.PodCondition{
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
					nodeStates[nodesetutils.GetNodeName(nodesetutils.NewNodeSetPod(nodeset, controller, 7, ""))] = []corev1.PodCondition{
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
			if (err != nil) != tt.wantErr {
				t.Errorf("realSlurmControl.CalculateNodeStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("realSlurmControl.CalculateNodeStatus() = %v, want %v", got, tt.want)
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
			if got := tolerateError(tt.args.err); got != tt.want {
				t.Errorf("tolerateError() = %v, want %v", got, tt.want)
			}
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
			if got := nodeState(tt.args.node, tt.args.state); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nodeState() = %v, want %v", got, tt.want)
			}
		})
	}
}
