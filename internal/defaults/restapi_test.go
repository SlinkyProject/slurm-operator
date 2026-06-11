// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestSetRestApiDefaults(t *testing.T) {
	t.Run("nil restapi is a no-op", func(t *testing.T) {
		SetRestApiDefaults(nil)
	})

	t.Run("zero value spec gets defaults", func(t *testing.T) {
		ra := &slinkyv1beta1.RestApi{}
		SetRestApiDefaults(ra)

		require.Equal(t, ptr.To(DefaultRestApiReplicas), ra.Spec.Replicas)
	})

	t.Run("explicit values are not overridden", func(t *testing.T) {
		ra := &slinkyv1beta1.RestApi{}
		ra.Spec.Replicas = ptr.To(int32(5))
		SetRestApiDefaults(ra)

		require.Equal(t, ptr.To(int32(5)), ra.Spec.Replicas)
	})
}
