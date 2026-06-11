// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"testing"

	"github.com/stretchr/testify/require"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestSetAccountingDefaults(t *testing.T) {
	t.Run("nil accounting is a no-op", func(t *testing.T) {
		SetAccountingDefaults(nil)
	})

	t.Run("zero value spec gets defaults", func(t *testing.T) {
		a := &slinkyv1beta1.Accounting{}
		SetAccountingDefaults(a)

		require.Equal(t, DefaultAccountingStoragePort, a.Spec.StorageConfig.Port)
		require.Equal(t, DefaultAccountingStorageDB, a.Spec.StorageConfig.Database)
	})

	t.Run("explicit values are not overridden", func(t *testing.T) {
		a := &slinkyv1beta1.Accounting{}
		a.Spec.StorageConfig.Port = 9999
		a.Spec.StorageConfig.Database = "mydb"
		SetAccountingDefaults(a)

		require.Equal(t, 9999, a.Spec.StorageConfig.Port)
		require.Equal(t, "mydb", a.Spec.StorageConfig.Database)
	})
}
