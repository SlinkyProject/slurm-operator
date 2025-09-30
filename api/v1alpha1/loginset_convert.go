// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// ConvertTo converts this object to the hub object.
func (src *LoginSet) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*slinkyv1beta1.LoginSet)

	// Convert metadata diff
	dst.ObjectMeta = src.ObjectMeta

	// Convert spec diff

	return nil
}

// ConvertFrom converts the hub object to this object.
func (dst *LoginSet) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*slinkyv1beta1.LoginSet)

	// Convert metadata diff
	dst.ObjectMeta = src.ObjectMeta

	// Convert spec diff

	return nil
}
