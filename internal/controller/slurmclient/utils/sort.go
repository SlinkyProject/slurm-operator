// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

type RestapisByCreationTimestamp []slinkyv1beta1.RestApi

func (o RestapisByCreationTimestamp) Len() int {
	return len(o)
}

func (o RestapisByCreationTimestamp) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

func (o RestapisByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}
