// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package historycontrol

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_realHistory_ListControllerRevisions(t *testing.T) {
	defaultNamespace := metav1.NamespaceDefault
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "foo",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
	selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
	if err != nil {
		t.Fatal(err)
	}
	revision := &appsv1.ControllerRevision{
		TypeMeta:   rs.TypeMeta,
		ObjectMeta: rs.ObjectMeta,
		Revision:   1,
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		parent   metav1.Object
		selector labels.Selector
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*appsv1.ControllerRevision
		wantErr bool
	}{
		{
			name: "Empty list",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				parent:   rs,
				selector: selector,
			},
			want:    []*appsv1.ControllerRevision{},
			wantErr: false,
		},
		{
			name: "List revisions",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(rs, revision).Build(),
			},
			args: args{
				parent:   rs,
				selector: selector,
			},
			want:    []*appsv1.ControllerRevision{revision},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rh := &realHistory{
				Client: tt.fields.Client,
			}
			got, err := rh.ListControllerRevisions(tt.args.parent, tt.args.selector)
			if (err != nil) != tt.wantErr {
				t.Errorf("realHistory.ListControllerRevisions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("realHistory.ListControllerRevisions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Benchmark_realHistory_ListControllerRevisions(b *testing.B) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "foo",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
	selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
	if err != nil {
		b.Fatal(err)
	}
	revision := &appsv1.ControllerRevision{
		TypeMeta:   rs.TypeMeta,
		ObjectMeta: rs.ObjectMeta,
		Revision:   1,
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		parent   metav1.Object
		selector labels.Selector
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Empty list",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				parent:   rs,
				selector: selector,
			},
		},
		{
			name: "List revisions",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(rs, revision).Build(),
			},
			args: args{
				parent:   rs,
				selector: selector,
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			rh := &realHistory{
				Client: bb.fields.Client,
			}
			for b.Loop() {
				rh.ListControllerRevisions(bb.args.parent, bb.args.selector) //nolint:errcheck
			}
		})
	}
}

func Test_realHistory_CreateControllerRevision(t *testing.T) {
	defaultNamespace := metav1.NamespaceDefault
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "foo",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
	revision := &appsv1.ControllerRevision{
		TypeMeta:   rs.TypeMeta,
		ObjectMeta: rs.ObjectMeta,
		Revision:   1,
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		parent         metav1.Object
		revision       *appsv1.ControllerRevision
		collisionCount *int32
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Create revision",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				parent:         rs,
				revision:       revision,
				collisionCount: new(int32),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rh := &realHistory{
				Client: tt.fields.Client,
			}
			_, err := rh.CreateControllerRevision(tt.args.parent, tt.args.revision, tt.args.collisionCount)
			if (err != nil) != tt.wantErr {
				t.Errorf("realHistory.CreateControllerRevision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Benchmark_realHistory_CreateControllerRevision(b *testing.B) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "foo",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
	revision := &appsv1.ControllerRevision{
		TypeMeta:   rs.TypeMeta,
		ObjectMeta: rs.ObjectMeta,
		Revision:   1,
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		parent         metav1.Object
		revision       *appsv1.ControllerRevision
		collisionCount *int32
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Create revision",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				parent:         rs,
				revision:       revision,
				collisionCount: new(int32),
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			rh := &realHistory{
				Client: bb.fields.Client,
			}
			for b.Loop() {
				rh.CreateControllerRevision(bb.args.parent, bb.args.revision, bb.args.collisionCount) //nolint:errcheck
			}
		})
	}
}

func Test_realHistory_UpdateControllerRevision(t *testing.T) {
	defaultNamespace := metav1.NamespaceDefault
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "foo",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
	revision := &appsv1.ControllerRevision{
		TypeMeta:   rs.TypeMeta,
		ObjectMeta: rs.ObjectMeta,
		Revision:   1,
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		revision    *appsv1.ControllerRevision
		newRevision int64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *appsv1.ControllerRevision
		wantErr bool
	}{
		{
			name: "Update revision",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(revision).Build(),
			},
			args: args{
				revision: revision.DeepCopy(),
				newRevision: func() int64 {
					val := revision.Revision
					val++
					return val
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rh := &realHistory{
				Client: tt.fields.Client,
			}
			_, err := rh.UpdateControllerRevision(tt.args.revision, tt.args.newRevision)
			if (err != nil) != tt.wantErr {
				t.Errorf("realHistory.UpdateControllerRevision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Benchmark_realHistory_UpdateControllerRevision(b *testing.B) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "foo",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
	revision := &appsv1.ControllerRevision{
		TypeMeta:   rs.TypeMeta,
		ObjectMeta: rs.ObjectMeta,
		Revision:   1,
	}

	type fields struct {
		Client client.Client
	}
	type args struct {
		revision    *appsv1.ControllerRevision
		newRevision int64
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Update revision",
			fields: fields{
				Client: fake.NewClientBuilder().WithObjects(revision).Build(),
			},
			args: args{
				revision:    revision.DeepCopy(),
				newRevision: revision.Revision + 1,
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			rh := &realHistory{
				Client: bb.fields.Client,
			}
			for b.Loop() {
				rh.UpdateControllerRevision(bb.args.revision, bb.args.newRevision) //nolint:errcheck
			}
		})
	}
}

func Test_realHistory_DeleteControllerRevision(t *testing.T) {
	revision := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "foo",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		revision *appsv1.ControllerRevision
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "not found",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				revision: revision.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "found",
			fields: fields{
				Client: fake.NewFakeClient(revision.DeepCopy()),
			},
			args: args{
				revision: revision.DeepCopy(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rh := &realHistory{
				Client: tt.fields.Client,
			}
			if err := rh.DeleteControllerRevision(tt.args.revision); (err != nil) != tt.wantErr {
				t.Errorf("realHistory.DeleteControllerRevision() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Benchmark_realHistory_DeleteControllerRevision(b *testing.B) {
	revision := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "foo",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		revision *appsv1.ControllerRevision
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "not found",
			fields: fields{
				Client: fake.NewFakeClient(),
			},
			args: args{
				revision: revision.DeepCopy(),
			},
		},
		{
			name: "found",
			fields: fields{
				Client: fake.NewFakeClient(revision.DeepCopy()),
			},
			args: args{
				revision: revision.DeepCopy(),
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			rh := &realHistory{
				Client: bb.fields.Client,
			}
			for b.Loop() {
				rh.DeleteControllerRevision(bb.args.revision) //nolint:errcheck
			}
		})
	}
}

func Test_realHistory_AdoptControllerRevision(t *testing.T) {
	revision := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "foo",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		parent     metav1.Object
		parentKind schema.GroupVersionKind
		revision   *appsv1.ControllerRevision
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *appsv1.ControllerRevision
		wantErr bool
	}{
		{
			name: "match",
			fields: fields{
				Client: fake.NewFakeClient(revision.DeepCopy()),
			},
			args: args{
				parent: &metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "FooResource",
					UID:       "00000",
				},
				parentKind: schema.FromAPIVersionAndKind("foo/v1", "Foo"),
				revision:   revision.DeepCopy(),
			},
			want: &appsv1.ControllerRevision{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       metav1.NamespaceDefault,
					Name:            "foo",
					ResourceVersion: "1000",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "foo/v1",
							Kind:               "Foo",
							Name:               "FooResource",
							UID:                "00000",
							Controller:         ptr.To(true),
							BlockOwnerDeletion: ptr.To(true),
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rh := &realHistory{
				Client: tt.fields.Client,
			}
			got, err := rh.AdoptControllerRevision(tt.args.parent, tt.args.parentKind, tt.args.revision)
			if (err != nil) != tt.wantErr {
				t.Errorf("realHistory.AdoptControllerRevision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("realHistory.AdoptControllerRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Benchmark_realHistory_AdoptControllerRevision(b *testing.B) {
	revision := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Name:      "foo",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		parent     metav1.Object
		parentKind schema.GroupVersionKind
		revision   *appsv1.ControllerRevision
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "match",
			fields: fields{
				Client: fake.NewFakeClient(revision.DeepCopy()),
			},
			args: args{
				parent: &metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "FooResource",
					UID:       "00000",
				},
				parentKind: schema.FromAPIVersionAndKind("foo/v1", "Foo"),
				revision:   revision.DeepCopy(),
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			rh := &realHistory{
				Client: bb.fields.Client,
			}
			for b.Loop() {
				rh.AdoptControllerRevision(bb.args.parent, bb.args.parentKind, bb.args.revision) //nolint:errcheck
			}
		})
	}
}

func Test_realHistory_ReleaseControllerRevision(t *testing.T) {
	revision := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       metav1.NamespaceDefault,
			Name:            "foo",
			ResourceVersion: "1000",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		parent   metav1.Object
		revision *appsv1.ControllerRevision
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *appsv1.ControllerRevision
		wantErr bool
	}{
		{
			name: "found",
			fields: fields{
				Client: fake.NewFakeClient(revision.DeepCopy()),
			},
			args: args{
				parent: &metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "FooResource",
					UID:       "00000",
				},
				revision: revision.DeepCopy(),
			},
			want: &appsv1.ControllerRevision{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:       metav1.NamespaceDefault,
					Name:            "foo",
					ResourceVersion: "1001",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rh := &realHistory{
				Client: tt.fields.Client,
			}
			got, err := rh.ReleaseControllerRevision(tt.args.parent, tt.args.revision)
			if (err != nil) != tt.wantErr {
				t.Errorf("realHistory.ReleaseControllerRevision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("realHistory.ReleaseControllerRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Benchmark_realHistory_ReleaseControllerRevision(b *testing.B) {
	revision := &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       metav1.NamespaceDefault,
			Name:            "foo",
			ResourceVersion: "1000",
		},
	}
	type fields struct {
		Client client.Client
	}
	type args struct {
		parent   metav1.Object
		revision *appsv1.ControllerRevision
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "found",
			fields: fields{
				Client: fake.NewFakeClient(revision.DeepCopy()),
			},
			args: args{
				parent: &metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "FooResource",
					UID:       "00000",
				},
				revision: revision.DeepCopy(),
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			rh := &realHistory{
				Client: bb.fields.Client,
			}
			for b.Loop() {
				rh.ReleaseControllerRevision(bb.args.parent, bb.args.revision) //nolint:errcheck
			}
		})
	}
}
