// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"maps"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
)

type MetadataBuilder struct {
	objMeta metav1.ObjectMeta
}

func (b *MetadataBuilder) WithMetadata(meta slinkyv1beta1.Metadata) *MetadataBuilder {
	b.WithAnnotations(meta.Annotations)
	b.WithLabels(meta.Labels)
	return b
}

func (b *MetadataBuilder) WithAnnotations(annotations map[string]string) *MetadataBuilder {
	maps.Copy(b.objMeta.Annotations, annotations)
	return b
}

func (b *MetadataBuilder) WithLabels(labels map[string]string) *MetadataBuilder {
	maps.Copy(b.objMeta.Labels, labels)
	return b
}

func (b *MetadataBuilder) Build() metav1.ObjectMeta {
	return b.objMeta
}

func NewBuilder(key types.NamespacedName) *MetadataBuilder {
	o := &MetadataBuilder{
		objMeta: metav1.ObjectMeta{
			Name:        key.Name,
			Namespace:   key.Namespace,
			Annotations: make(map[string]string),
			Labels:      make(map[string]string),
		},
	}
	return o
}
