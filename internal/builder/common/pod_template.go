// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/metadata"
	"github.com/SlinkyProject/slurm-operator/internal/utils/structutils"
)

type PodTemplateOpts struct {
	Key      types.NamespacedName
	Metadata slinkyv1beta1.Metadata
	Base     corev1.PodSpec
	Merge    corev1.PodSpec
}

func (b *CommonBuilder) BuildPodTemplate(opts PodTemplateOpts) corev1.PodTemplateSpec {
	// Handle non `patchStrategy=Merge` fields as if they were.
	opts.Base.Containers = structutils.MergeList(opts.Base.Containers, opts.Merge.Containers)
	opts.Merge.Containers = []corev1.Container{}
	opts.Base.InitContainers = structutils.MergeList(opts.Base.InitContainers, opts.Merge.InitContainers)
	opts.Merge.InitContainers = []corev1.Container{}

	Base := &corev1.PodTemplateSpec{
		ObjectMeta: metadata.NewBuilder(opts.Key).
			WithMetadata(opts.Metadata).
			Build(),
		Spec: opts.Base,
	}
	Merge := &corev1.PodTemplateSpec{
		Spec: opts.Merge,
	}

	out := structutils.StrategicMergePatch(Base, Merge)

	return ptr.Deref(out, corev1.PodTemplateSpec{})
}
