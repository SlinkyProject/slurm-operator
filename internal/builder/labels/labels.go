// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package labels

import (
	"maps"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
)

// Ref: https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
const (
	appLabel       = "app.kubernetes.io/name"
	instanceLabel  = "app.kubernetes.io/instance"
	componentLabel = "app.kubernetes.io/component"
	partOfLabel    = "app.kubernetes.io/part-of"
	managedbyLabel = "app.kubernetes.io/managed-by"
)

type Builder struct {
	labels map[string]string
}

func NewBuilder() *Builder {
	return &Builder{
		labels: map[string]string{},
	}
}

func (b *Builder) WithApp(app string) *Builder {
	b.labels[appLabel] = app
	return b
}

func (b *Builder) WithInstance(instance string) *Builder {
	b.labels[instanceLabel] = instance
	return b
}

func (b *Builder) WithComponent(component string) *Builder {
	b.labels[componentLabel] = component
	return b
}

func (b *Builder) WithPartOf(instance string) *Builder {
	b.labels[partOfLabel] = instance
	return b
}

func (b *Builder) WithManagedBy(component string) *Builder {
	b.labels[managedbyLabel] = component
	return b
}

func (b *Builder) WithLabels(labels map[string]string) *Builder {
	maps.Copy(b.labels, labels)
	return b
}

const (
	ControllerApp  = "slurmctld"
	ControllerComp = "controller"

	RestapiApp  = "slurmrestd"
	RestapiComp = "restapi"

	AccountingApp  = "slurmdbd"
	AccountingComp = "accounting"

	ComputeApp  = "slurmd"
	ComputeComp = "compute"

	LoginApp  = "login"
	LoginComp = "login"
)

func (b *Builder) WithControllerSelectorLabels(obj *slinkyv1alpha1.Controller) *Builder {
	return b.
		WithApp(ControllerApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithControllerLabels(obj *slinkyv1alpha1.Controller) *Builder {
	return b.
		WithControllerSelectorLabels(obj).
		WithComponent(ControllerComp)
}

func (b *Builder) WithRestapiSelectorLabels(obj *slinkyv1alpha1.RestApi) *Builder {
	return b.
		WithApp(RestapiApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithRestapiLabels(obj *slinkyv1alpha1.RestApi) *Builder {
	return b.
		WithRestapiSelectorLabels(obj).
		WithComponent(RestapiComp)
}

func (b *Builder) WithAccountingSelectorLabels(obj *slinkyv1alpha1.Accounting) *Builder {
	return b.
		WithApp(AccountingApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithAccountingLabels(obj *slinkyv1alpha1.Accounting) *Builder {
	return b.
		WithAccountingSelectorLabels(obj).
		WithComponent(AccountingComp)
}

func (b *Builder) WithComputeSelectorLabels(obj *slinkyv1alpha1.NodeSet) *Builder {
	return b.
		WithApp(ComputeApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithComputeLabels(obj *slinkyv1alpha1.NodeSet) *Builder {
	return b.
		WithComputeSelectorLabels(obj).
		WithComponent(ComputeComp)
}

func (b *Builder) WithLoginSelectorLabels(obj *slinkyv1alpha1.LoginSet) *Builder {
	return b.
		WithApp(LoginApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithLoginLabels(obj *slinkyv1alpha1.LoginSet) *Builder {
	return b.
		WithLoginSelectorLabels(obj).
		WithComponent(LoginComp)
}

func (b *Builder) Build() map[string]string {
	return b.labels
}
