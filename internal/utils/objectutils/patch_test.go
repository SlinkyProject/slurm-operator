// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	utilruntime.Must(slinkyv1beta1.AddToScheme(scheme.Scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme.Scheme))
}

func TestSyncObject(t *testing.T) {
	type args struct {
		c            client.Client
		ctx          context.Context
		newObj       client.Object
		shouldUpdate bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Create ConfigMap",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update ConfigMap",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
					Data: map[string]string{
						"foo": "bar",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update Immutable ConfigMap",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Immutable: ptr.To(true),
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
					Data: map[string]string{
						"foo": "bar",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create Secret",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update Secret",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update Immutable Secret",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
						Immutable: ptr.To(true),
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create Service",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update Service",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create Deployment",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update Deployment",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create StatefulSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update StatefulSet",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&appsv1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create Controller",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update Controller",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create Restapi",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update Restapi",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&slinkyv1beta1.RestApi{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create Accounting",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update Accounting",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&slinkyv1beta1.Accounting{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create NodeSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update NodeSet",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&slinkyv1beta1.NodeSet{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create LoginSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update LoginSet",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&slinkyv1beta1.LoginSet{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create PodDisruptionBuidget",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update PodDisruptionBuidget",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&policyv1.PodDisruptionBudget{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create ServiceMonitor",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Update ServiceMonitor",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&monitoringv1.ServiceMonitor{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx: context.TODO(),
				newObj: &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Create Replicaset",
			args: args{
				c:            fake.NewFakeClient(),
				ctx:          context.TODO(),
				newObj:       &appsv1.ReplicaSet{},
				shouldUpdate: true,
			},
			wantErr: true,
		},
		{
			name: "Update Replicaset",
			args: args{
				c: fake.NewClientBuilder().WithObjects(
					&appsv1.ReplicaSet{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				).Build(),
				ctx:          context.TODO(),
				newObj:       &appsv1.ReplicaSet{},
				shouldUpdate: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SyncObject(tt.args.c, tt.args.ctx, tt.args.newObj, tt.args.shouldUpdate); (err != nil) != tt.wantErr {
				t.Errorf("SyncObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkSyncObject(b *testing.B) {
	type args struct {
		c            client.Client
		ctx          context.Context
		newObj       client.Object
		shouldUpdate bool
	}
	benchmarks := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ConfigMap",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Secret",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Service",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Deployment",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "StatefulSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Controller",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Restapi",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Accounting",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "NodeSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "LoginSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				SyncObject(bb.args.c, bb.args.ctx, bb.args.newObj, bb.args.shouldUpdate) //nolint:errcheck
			}
		})
	}
}
