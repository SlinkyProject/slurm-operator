// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestSetControllerDefaults(t *testing.T) {
	t.Run("nil controller is a no-op", func(t *testing.T) {
		SetControllerDefaults(nil)
	})

	t.Run("zero value spec gets defaults", func(t *testing.T) {
		c := &slinkyv1beta1.Controller{}
		SetControllerDefaults(c)
		require.Equal(t, new(DefaultControllerPersistenceEnabled), c.Spec.Persistence.Enabled)
		if c.Spec.HighAvailability.Enabled {
			require.Equal(t, new(DefaultControllerHighAvailabilityBackups), c.Spec.HighAvailability.Backups)
		}
	})

	t.Run("explicit values are not overridden", func(t *testing.T) {
		c := &slinkyv1beta1.Controller{}
		c.Spec.Persistence.Enabled = ptr.To(true)
		c.Spec.HighAvailability.Enabled = true
		const HABackups int32 = 2
		c.Spec.HighAvailability.Backups = ptr.To(HABackups)
		SetControllerDefaults(c)
		require.Equal(t, new(true), c.Spec.Persistence.Enabled)
		if c.Spec.HighAvailability.Enabled {
			require.Equal(t, new(HABackups), c.Spec.HighAvailability.Backups)
		}
		c.Spec.Persistence.Enabled = ptr.To(false)
		SetControllerDefaults(c)
		require.Equal(t, new(false), c.Spec.Persistence.Enabled)
	})
}
