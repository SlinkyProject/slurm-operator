// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildSecret(t *testing.T) {
	type args struct {
		opts  SecretOpts
		owner metav1.Object
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "empty",
			args: args{
				owner: &appsv1.Deployment{},
			},
		},
		{
			name:    "bad owner",
			wantErr: true,
		},
		{
			name: "with options",
			args: args{
				opts: SecretOpts{
					Key: types.NamespacedName{
						Name:      "foo",
						Namespace: "bar",
					},
					Metadata: slinkyv1beta1.Metadata{
						Annotations: map[string]string{
							"foo": "bar",
						},
						Labels: map[string]string{
							"fizz": "buzz",
						},
					},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
					StringData: map[string]string{
						"fizz": "buzz",
					},
					Immutable: true,
				},
				owner: &appsv1.Deployment{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(fake.NewFakeClient())
			got, err := b.BuildSecret(tt.args.opts, tt.args.owner)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case objectutils.KeyFunc(got) != tt.args.opts.Key.String():
				t.Errorf("NamespacedName = %v , want = %v", objectutils.KeyFunc(got), tt.args.opts.Key.String())

			case !apiequality.Semantic.DeepEqual(got.Annotations, tt.args.opts.Metadata.Annotations):
				t.Errorf("Annotations = %v , want = %v", got.Annotations, tt.args.opts.Metadata.Annotations)

			case !apiequality.Semantic.DeepEqual(got.Labels, tt.args.opts.Metadata.Labels):
				t.Errorf("Labels = %v , want = %v", got.Labels, tt.args.opts.Metadata.Labels)

			case ptr.Deref(got.Immutable, false) != tt.args.opts.Immutable:
				t.Errorf("got.Immutable = %v , want = %v", got.Immutable, tt.args.opts.Immutable)

			case !apiequality.Semantic.DeepEqual(got.Data, tt.args.opts.Data):
				t.Errorf("got.Data = %v , want = %v", got.Data, tt.args.opts.Data)

			case !apiequality.Semantic.DeepEqual(got.StringData, tt.args.opts.StringData):
				t.Errorf("got.StringData = %v , want = %v", got.StringData, tt.args.opts.StringData)
			}
		})
	}
}

func BenchmarkBuilder_BuildSecret(b *testing.B) {
	type args struct {
		opts  SecretOpts
		owner metav1.Object
	}
	benchmarks := []struct {
		name string
		args args
	}{
		{
			name: "empty",
			args: args{
				owner: &appsv1.Deployment{},
			},
		},
		{
			name: "bad owner",
		},
		{
			name: "with options",
			args: args{
				opts: SecretOpts{
					Key: types.NamespacedName{
						Name:      "foo",
						Namespace: "bar",
					},
					Metadata: slinkyv1beta1.Metadata{
						Annotations: map[string]string{
							"foo": "bar",
						},
						Labels: map[string]string{
							"fizz": "buzz",
						},
					},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
					StringData: map[string]string{
						"fizz": "buzz",
					},
					Immutable: true,
				},
				owner: &appsv1.Deployment{},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(fake.NewFakeClient())

			for b.Loop() {
				build.BuildSecret(bb.args.opts, bb.args.owner) //nolint:errcheck
			}
		})
	}
}
