// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNewBuilder(t *testing.T) {
	type args struct {
		key types.NamespacedName
	}
	tests := []struct {
		name string
		args args
		want metav1.ObjectMeta
	}{
		{
			name: "empty",
			args: args{
				key: types.NamespacedName{},
			},
			want: metav1.ObjectMeta{},
		},
		{
			name: "non-empty",
			args: args{
				key: types.NamespacedName{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			want: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBuilder(tt.args.key).Build(); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("NewBuilder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadataBuilder_WithMetadata(t *testing.T) {
	type fields struct {
		key types.NamespacedName
	}
	type args struct {
		meta slinkyv1beta1.Metadata
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   metav1.ObjectMeta
	}{
		{
			name: "empty",
			fields: fields{
				key: types.NamespacedName{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			args: args{
				meta: slinkyv1beta1.Metadata{},
			},
			want: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		},
		{
			name: "non-empty",
			fields: fields{
				key: types.NamespacedName{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			args: args{
				meta: slinkyv1beta1.Metadata{
					Annotations: map[string]string{
						"foo": "bar",
					},
					Labels: map[string]string{
						"fizz": "buzz",
					},
				},
			},
			want: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				Annotations: map[string]string{
					"foo": "bar",
				},
				Labels: map[string]string{
					"fizz": "buzz",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder(tt.fields.key)
			if got := b.WithMetadata(tt.args.meta).Build(); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("MetadataBuilder.WithMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadataBuilder_WithAnnotations(t *testing.T) {
	type fields struct {
		key types.NamespacedName
	}
	type args struct {
		annotations map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   metav1.ObjectMeta
	}{
		{
			name: "empty",
			fields: fields{
				key: types.NamespacedName{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			args: args{
				annotations: map[string]string{},
			},
			want: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		},
		{
			name: "non-empty",
			fields: fields{
				key: types.NamespacedName{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			args: args{
				annotations: map[string]string{
					"foo": "bar",
				},
			},
			want: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				Annotations: map[string]string{
					"foo": "bar",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder(tt.fields.key)
			if got := b.WithAnnotations(tt.args.annotations).Build(); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("MetadataBuilder.WithAnnotations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadataBuilder_WithLabels(t *testing.T) {
	type fields struct {
		key types.NamespacedName
	}
	type args struct {
		labels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   metav1.ObjectMeta
	}{
		{
			name: "empty",
			fields: fields{
				key: types.NamespacedName{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			args: args{
				labels: map[string]string{},
			},
			want: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
			},
		},
		{
			name: "non-empty",
			fields: fields{
				key: types.NamespacedName{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			args: args{
				labels: map[string]string{
					"fizz": "buzz",
				},
			},
			want: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "bar",
				Labels: map[string]string{
					"fizz": "buzz",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBuilder(tt.fields.key)
			if got := b.WithLabels(tt.args.labels).Build(); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("MetadataBuilder.WithLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
