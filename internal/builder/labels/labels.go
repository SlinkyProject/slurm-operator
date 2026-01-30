// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package labels

import (
	"maps"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
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

const (
	clusterLabel = "slinky.slurm.net/cluster"
)

func (b *Builder) WithCluster(cluster string) *Builder {
	b.labels[clusterLabel] = cluster
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

	WorkerApp  = "slurmd"
	WorkerComp = "worker"

	LoginApp  = "login"
	LoginComp = "login"
)

func (b *Builder) WithControllerSelectorLabels(obj *slinkyv1beta1.Controller) *Builder {
	return b.
		WithApp(ControllerApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithControllerLabels(obj *slinkyv1beta1.Controller) *Builder {
	return b.
		WithControllerSelectorLabels(obj).
		WithComponent(ControllerComp)
}

func (b *Builder) WithRestapiSelectorLabels(obj *slinkyv1beta1.RestApi) *Builder {
	return b.
		WithApp(RestapiApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithRestapiLabels(obj *slinkyv1beta1.RestApi) *Builder {
	return b.
		WithRestapiSelectorLabels(obj).
		WithComponent(RestapiComp)
}

func (b *Builder) WithAccountingSelectorLabels(obj *slinkyv1beta1.Accounting) *Builder {
	return b.
		WithApp(AccountingApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithAccountingLabels(obj *slinkyv1beta1.Accounting) *Builder {
	return b.
		WithAccountingSelectorLabels(obj).
		WithComponent(AccountingComp)
}

func (b *Builder) WithWorkerSelectorLabels(obj *slinkyv1beta1.NodeSet) *Builder {
	return b.
		WithApp(WorkerApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithWorkerLabels(obj *slinkyv1beta1.NodeSet) *Builder {
	return b.
		WithWorkerSelectorLabels(obj).
		WithComponent(WorkerComp).
		WithCluster(obj.Spec.ControllerRef.Name)
}

func (b *Builder) WithLoginSelectorLabels(obj *slinkyv1beta1.LoginSet) *Builder {
	return b.
		WithApp(LoginApp).
		WithInstance(obj.Name)
}

func (b *Builder) WithLoginLabels(obj *slinkyv1beta1.LoginSet) *Builder {
	return b.
		WithLoginSelectorLabels(obj).
		WithComponent(LoginComp)
}

func (b *Builder) WithPodProtect() *Builder {
	b.labels[slinkyv1beta1.LabelNodeSetPodProtect] = "true"
	return b
}

func (b *Builder) Build() map[string]string {
	return b.labels
}
