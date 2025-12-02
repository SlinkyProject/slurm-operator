// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

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
	base     corev1.PodSpec
	merge    corev1.PodSpec
}

func (b *Builder) buildPodTemplate(opts PodTemplateOpts) corev1.PodTemplateSpec {
	// Handle non `patchStrategy=merge` fields as if they were.
	opts.base.Containers = structutils.MergeList(opts.base.Containers, opts.merge.Containers)
	opts.merge.Containers = []corev1.Container{}
	opts.base.InitContainers = structutils.MergeList(opts.base.InitContainers, opts.merge.InitContainers)
	opts.merge.InitContainers = []corev1.Container{}

	base := &corev1.PodTemplateSpec{
		ObjectMeta: metadata.NewBuilder(opts.Key).
			WithMetadata(opts.Metadata).
			Build(),
		Spec: opts.base,
	}

	merge := &corev1.PodTemplateSpec{
		Spec: opts.merge,
	}

	out := structutils.StrategicMergePatch(base, merge)

	return ptr.Deref(out, corev1.PodTemplateSpec{})
}
