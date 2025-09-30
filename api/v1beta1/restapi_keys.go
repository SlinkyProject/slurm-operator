// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/SlinkyProject/slurm-operator/internal/utils/domainname"
)

func (o *RestApi) Key() types.NamespacedName {
	return types.NamespacedName{
		Name:      fmt.Sprintf("%s-restapi", o.Name),
		Namespace: o.Namespace,
	}
}

func (o *RestApi) ServiceKey() types.NamespacedName {
	key := o.Key()
	return types.NamespacedName{
		Name:      key.Name,
		Namespace: o.Namespace,
	}
}

func (o *RestApi) ServiceFQDN() string {
	s := o.ServiceKey()
	return domainname.Fqdn(s.Name, s.Namespace)
}

func (o *RestApi) ServiceFQDNShort() string {
	s := o.ServiceKey()
	return domainname.FqdnShort(s.Name, s.Namespace)
}
