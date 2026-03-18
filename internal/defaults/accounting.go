// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// Default values for Accounting Spec fields when unspecified.
const (
	DefaultAccountingStoragePort int    = 3306
	DefaultAccountingStorageDB   string = "slurm_acct_db"
)

func SetAccountingDefaults(accounting *slinkyv1beta1.Accounting) {
	if accounting == nil {
		return
	}
	s := &accounting.Spec

	if s.StorageConfig.Port == 0 {
		s.StorageConfig.Port = DefaultAccountingStoragePort
	}
	if s.StorageConfig.Database == "" {
		s.StorageConfig.Database = DefaultAccountingStorageDB
	}
}
