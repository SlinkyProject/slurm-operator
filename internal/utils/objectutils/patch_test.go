// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"sync/atomic"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
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
			if err := SyncObject(tt.args.c, tt.args.ctx, nil, nil, tt.args.newObj, tt.args.shouldUpdate); (err != nil) != tt.wantErr {
				t.Errorf("SyncObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyncObject_Service_appliesOwnerReferencesOnPatch(t *testing.T) {
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "slurm", Name: "slurm-workers-slurm"}

	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       key.Namespace,
			Name:            key.Name,
			ResourceVersion: "1",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                corev1.ClusterIPNone,
			PublishNotReadyAddresses: true,
			Selector: map[string]string{
				"app.kubernetes.io/name": "slurm-worker",
			},
			Ports: []corev1.ServicePort{
				{Name: "slurmd", Port: 6818, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(existing.DeepCopy()).Build()

	desired := existing.DeepCopy()
	desired.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "slinky.slurm.net/v1beta1",
			Kind:       "NodeSet",
			Name:       "workers-a",
			UID:        "uid-a",
			Controller: ptr.To(false),
		},
	}

	if err := SyncObject(c, ctx, nil, nil, desired, true); err != nil {
		t.Fatalf("SyncObject: %v", err)
	}

	got := &corev1.Service{}
	if err := c.Get(ctx, key, got); err != nil {
		t.Fatal(err)
	}
	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].Name != "workers-a" {
		t.Fatalf("OwnerReferences = %+v", got.OwnerReferences)
	}
}

func TestSyncObject_CreateAlreadyExists_loadsAndPatches(t *testing.T) {
	ctx := context.Background()
	key := client.ObjectKey{Namespace: "slurm", Name: "slurm-workers-slurm"}

	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       key.Namespace,
			Name:            key.Name,
			ResourceVersion: "42",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP:                corev1.ClusterIPNone,
			PublishNotReadyAddresses: true,
			Selector: map[string]string{
				"app.kubernetes.io/name": "slurm-worker",
			},
			Ports: []corev1.ServicePort{
				{Name: "slurmd", Port: 6818, Protocol: corev1.ProtocolTCP},
			},
		},
	}

	delegate := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(existing.DeepCopy()).Build()

	var getCalls int32
	c := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, objKey client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if atomic.AddInt32(&getCalls, 1) == 1 {
				return apierrors.NewNotFound(corev1.Resource("services"), objKey.Name)
			}
			return delegate.Get(ctx, objKey, obj, opts...)
		},
		Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			return apierrors.NewAlreadyExists(corev1.Resource("services"), obj.GetName())
		},
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			return delegate.Patch(ctx, obj, patch, opts...)
		},
	}).Build()

	desired := existing.DeepCopy()
	desired.ResourceVersion = ""
	desired.OwnerReferences = []metav1.OwnerReference{
		{
			APIVersion: "slinky.slurm.net/v1beta1",
			Kind:       "NodeSet",
			Name:       "workers-b",
			UID:        "uid-b",
			Controller: ptr.To(false),
		},
	}
	desired.Spec.Selector["cluster"] = "slurm"

	if err := SyncObject(c, ctx, nil, nil, desired, true); err != nil {
		t.Fatalf("SyncObject: %v", err)
	}
	if getCalls != 2 {
		t.Fatalf("expected 2 Get calls (stale NotFound + load after AlreadyExists), got %d", getCalls)
	}

	got := &corev1.Service{}
	if err := delegate.Get(ctx, key, got); err != nil {
		t.Fatal(err)
	}
	if len(got.OwnerReferences) != 1 || got.OwnerReferences[0].Name != "workers-b" {
		t.Fatalf("OwnerReferences = %+v", got.OwnerReferences)
	}
	if got.Spec.Selector["cluster"] != "slurm" {
		t.Fatalf("Spec.Selector[cluster] = %q", got.Spec.Selector["cluster"])
	}
}
