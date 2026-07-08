// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	sinterceptor "github.com/SlinkyProject/slurm-client/pkg/client/interceptor"

	clientfake "github.com/SlinkyProject/slurm-client/pkg/client/fake"
	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	builder "github.com/SlinkyProject/slurm-operator/internal/builder/controllerbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newControllerController(client client.Client, clientMap *clientmap.ClientMap) *ControllerReconciler {
	r := &ControllerReconciler{
		Client:        client,
		Scheme:        client.Scheme(),
		ClientMap:     clientMap,
		builder:       builder.New(client),
		refResolver:   refresolver.New(client),
		eventRecorder: events.NewFakeRecorder(10),
	}

	return r
}

func newClientMap(controllerName string, client slurmclient.Client) *clientmap.ClientMap {
	cm := clientmap.NewClientMap()
	key := types.NamespacedName{
		Namespace: corev1.NamespaceDefault,
		Name:      controllerName,
	}
	cm.Add(key, client)
	return cm
}

func TestControllerReconciler_sync(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "slurm",
		},
	}

	type fields struct {
		Client    client.Client
		ClientMap *clientmap.ClientMap
	}
	type args struct {
		ctx     context.Context
		request reconcile.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "default",
			fields: fields{
				Client: fake.NewFakeClient(controller.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					sclient := clientfake.NewClientBuilder().WithInterceptorFuncs(sinterceptor.Funcs{}).Build()
					return newClientMap(controller.Name, sclient)
				}(),
			},
			args: args{
				ctx: context.TODO(),
				request: reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: "slurm",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newControllerController(tt.fields.Client, tt.fields.ClientMap)
			if err := r.Sync(tt.args.ctx, tt.args.request); (err != nil) != tt.wantErr {
				t.Errorf("ControllerReconciler.sync() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkControllerReconciler_sync(b *testing.B) {
	benchmarks := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "default",
			wantErr: false,
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				b.StopTimer()
				controller := &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				}
				kubeClient := fake.NewFakeClient(controller.DeepCopy())
				sclient := clientfake.NewClientBuilder().WithInterceptorFuncs(sinterceptor.Funcs{}).Build()
				clientMap := newClientMap(controller.Name, sclient)
				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name: controller.Name,
					},
				}
				r := newControllerController(kubeClient, clientMap)
				b.StartTimer()

				if err := r.Sync(context.TODO(), request); (err != nil) != bb.wantErr {
					b.Errorf("ControllerReconciler.sync() error = %v, wantErr %v", err, bb.wantErr)
				}
			}
		})
	}
}
