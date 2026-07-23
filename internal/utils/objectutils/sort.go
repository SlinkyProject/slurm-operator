// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectsByName sorts a list of objects by names.
type ObjectsByName []client.Object

func (o ObjectsByName) Len() int {
	return len(o)
}

func (o ObjectsByName) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

func (o ObjectsByName) Less(i, j int) bool {
	return o[i].GetName() < o[j].GetName()
}

// PodsByName sorts a list of Pods by names.
type PodsByName []corev1.Pod

func (o PodsByName) Len() int {
	return len(o)
}

func (o PodsByName) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

func (o PodsByName) Less(i, j int) bool {
	return o[i].Name < o[j].Name
}
