// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func SetSlurmQosDefaults(qos *slinkyv1beta1.SlurmQos) {
	if qos == nil {
		return
	}
}
