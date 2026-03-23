// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// KeyFunc gets the namespacedName string for the meta object. Can be used as the key in a map.
func KeyFunc(obj metav1.Object) string {
	return NamespacedName(obj).String()
}

// NamespacedName gets the namespacedName for the meta object.
func NamespacedName(obj metav1.Object) types.NamespacedName {
	namespacedName := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}
	return namespacedName
}
