// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

func (o *NodeSet) Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      o.Name,
		Namespace: o.Namespace,
	}
}

func (o *NodeSet) HeadlessServiceKey() types.NamespacedName {
	key := o.Key()
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-headless", key.Name),
		Namespace: o.Namespace,
	}
}
