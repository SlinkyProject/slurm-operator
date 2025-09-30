// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/kubernetes/pkg/controller/history"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

func TestNodeSetReconciler_truncateHistory(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	const clusterName = "slurm"
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx       context.Context
		nodeset   *slinkyv1beta1.NodeSet
		revisions []*appsv1.ControllerRevision
		current   *appsv1.ControllerRevision
		update    *appsv1.ControllerRevision
	}
	type testCaseFields struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}
	tests := []testCaseFields{
		func() testCaseFields {
			nodeset := newNodeSet("foo", clusterName, 0)
			nodeset.Spec.RevisionHistoryLimit = ptr.To[int32](0)
			revisionList := &appsv1.ControllerRevisionList{
				Items: []appsv1.ControllerRevision{
					{ObjectMeta: metav1.ObjectMeta{Name: "rev-0"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "rev-1"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "rev-2"}},
				},
			}
			kclient := fake.NewFakeClient(nodeset, revisionList)

			return testCaseFields{
				name: "no pods",
				fields: fields{
					Client: kclient,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset.DeepCopy(),
					revisions: structutils.ReferenceList(revisionList.Items),
					current:   revisionList.Items[0].DeepCopy(),
					update:    revisionList.Items[1].DeepCopy(),
				},
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", clusterName, 3)
			nodeset.Spec.RevisionHistoryLimit = ptr.To[int32](2)
			podList := &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-0",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: "12345",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "pod-1",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: "98765",
							},
						},
					},
					{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}},
				},
			}
			revisionList := &appsv1.ControllerRevisionList{
				Items: []appsv1.ControllerRevision{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rev-0",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: "12345",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "rev-1",
							Labels: map[string]string{
								history.ControllerRevisionHashLabel: "98765",
							},
						},
					},
					{ObjectMeta: metav1.ObjectMeta{Name: "rev-2"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "rev-3"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "rev-4"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "rev-5"}},
				},
			}
			kclient := fake.NewFakeClient(nodeset, podList, revisionList)

			return testCaseFields{
				name: "with pods",
				fields: fields{
					Client: kclient,
				},
				args: args{
					ctx:       context.TODO(),
					nodeset:   nodeset.DeepCopy(),
					revisions: structutils.ReferenceList(revisionList.Items),
					current:   revisionList.Items[0].DeepCopy(),
					update:    revisionList.Items[1].DeepCopy(),
				},
				wantErr: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			if err := r.truncateHistory(tt.args.ctx, tt.args.nodeset, tt.args.revisions, tt.args.current, tt.args.update); (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.truncateHistory() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNodeSetReconciler_getNodeSetRevisions(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	type fields struct {
		Client client.Client
	}
	type args struct {
		nodeset   *slinkyv1beta1.NodeSet
		revisions []*appsv1.ControllerRevision
	}
	type testCaseFields struct {
		name    string
		fields  fields
		args    args
		want    *appsv1.ControllerRevision
		want1   *appsv1.ControllerRevision
		want2   int32
		wantErr bool
	}
	tests := []testCaseFields{
		func() testCaseFields {
			nodeset := newNodeSet("foo", "slurm", 2)
			revisionList := &appsv1.ControllerRevisionList{
				Items: []appsv1.ControllerRevision{
					func() appsv1.ControllerRevision {
						cr, err := newRevision(nodeset, 0, ptr.To[int32](0))
						if err != nil {
							panic(err)
						}
						return *cr
					}(),
					func() appsv1.ControllerRevision {
						cr, err := newRevision(nodeset, 1, ptr.To[int32](1))
						if err != nil {
							panic(err)
						}
						return *cr
					}(),
					func() appsv1.ControllerRevision {
						cr, err := newRevision(nodeset, 2, ptr.To[int32](2))
						if err != nil {
							panic(err)
						}
						return *cr
					}(),
				},
			}

			return testCaseFields{
				name: "nodeset hash not match",
				fields: fields{
					Client: fake.NewFakeClient(nodeset, revisionList),
				},
				args: args{
					nodeset:   nodeset.DeepCopy(),
					revisions: structutils.ReferenceList(revisionList.Items),
				},
				want:    revisionList.Items[2].DeepCopy(),
				want1:   revisionList.Items[2].DeepCopy(),
				want2:   0,
				wantErr: false,
			}
		}(),
		func() testCaseFields {
			nodeset := newNodeSet("foo", "slurm", 2)
			revisionList := &appsv1.ControllerRevisionList{
				Items: []appsv1.ControllerRevision{
					func() appsv1.ControllerRevision {
						cr, err := newRevision(nodeset, 0, ptr.To[int32](0))
						if err != nil {
							panic(err)
						}
						return *cr
					}(),
					func() appsv1.ControllerRevision {
						cr, err := newRevision(nodeset, 1, ptr.To[int32](1))
						if err != nil {
							panic(err)
						}
						return *cr
					}(),
					func() appsv1.ControllerRevision {
						cr, err := newRevision(nodeset, 2, ptr.To[int32](2))
						if err != nil {
							panic(err)
						}
						return *cr
					}(),
				},
			}
			nodeset.Status.NodeSetHash = revisionList.Items[1].Name

			return testCaseFields{
				name: "nodeset hash does match",
				fields: fields{
					Client: fake.NewFakeClient(nodeset, revisionList),
				},
				args: args{
					nodeset:   nodeset.DeepCopy(),
					revisions: structutils.ReferenceList(revisionList.Items),
				},
				want:    revisionList.Items[1].DeepCopy(),
				want1:   revisionList.Items[2].DeepCopy(),
				want2:   0,
				wantErr: false,
			}
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newNodeSetController(tt.fields.Client, nil)
			got, got1, got2, err := r.getNodeSetRevisions(tt.args.nodeset, tt.args.revisions)
			if (err != nil) != tt.wantErr {
				t.Errorf("NodeSetReconciler.getNodeSetRevisions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("NodeSetReconciler.getNodeSetRevisions() got = %v, want %v", got, tt.want)
			}
			if !apiequality.Semantic.DeepEqual(got1, tt.want1) {
				t.Errorf("NodeSetReconciler.getNodeSetRevisions() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("NodeSetReconciler.getNodeSetRevisions() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
