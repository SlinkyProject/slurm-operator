// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	sinterceptor "github.com/SlinkyProject/slurm-client/pkg/client/interceptor"

	clientfake "github.com/SlinkyProject/slurm-client/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	builder "github.com/SlinkyProject/slurm-operator/internal/builder/restapibuilder"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newRestapiController(client client.Client, clientMap *clientmap.ClientMap) *RestapiReconciler {
	r := &RestapiReconciler{
		Client:        client,
		Scheme:        client.Scheme(),
		builder:       builder.New(client),
		refResolver:   refresolver.New(client),
		eventRecorder: events.NewFakeRecorder(10),
	}

	return r
}

func newClientMap(restapiName string, client slurmclient.Client) *clientmap.ClientMap {
	cm := clientmap.NewClientMap()
	key := types.NamespacedName{
		Namespace: corev1.NamespaceDefault,
		Name:      restapiName,
	}
	cm.Add(key, client)
	return cm
}

func TestRestapiReconciler_sync(t *testing.T) {
	slurmKey := testutils.NewSlurmKeyRef("slurmkey")
	jwtKey := testutils.NewJwtKeyRef("jwtkey")
	controller := testutils.NewController("slurm", slurmKey, jwtKey, nil)
	restapi := testutils.NewRestapi("restapi", controller)

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
				Client: fake.NewFakeClient(restapi.DeepCopy()),
				ClientMap: func() *clientmap.ClientMap {
					sclient := clientfake.NewClientBuilder().WithInterceptorFuncs(sinterceptor.Funcs{}).Build()
					return newClientMap(restapi.Name, sclient)
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
			r := newRestapiController(tt.fields.Client, tt.fields.ClientMap)
			err := r.Sync(tt.args.ctx, tt.args.request)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func BenchmarkRestapiReconciler_sync(b *testing.B) {
	slurmKeyRef := testutils.NewSlurmKeyRef("slurmkey")
	slurmKey := testutils.NewSlurmKeySecret(slurmKeyRef)
	jwtKeyRef := testutils.NewJwtKeyRef("jwtkey")
	jwtKey := testutils.NewJwtKeySecret(jwtKeyRef)
	passwordKeyRef := testutils.NewPasswordRef("password")
	passwordKey := testutils.NewPasswordSecret(passwordKeyRef)

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
				controller := testutils.NewController("slurm", slurmKeyRef, jwtKeyRef, nil)
				restapi := testutils.NewRestapi("restapi", controller)
				kubeClient := fake.NewClientBuilder().WithObjects(
					restapi.DeepCopy(),
					controller.DeepCopy(),
					slurmKey.DeepCopy(),
					jwtKey.DeepCopy(),
					passwordKey.DeepCopy(),
				).WithStatusSubresource(&slinkyv1beta1.RestApi{}).Build()
				sclient := clientfake.NewClientBuilder().WithInterceptorFuncs(sinterceptor.Funcs{}).Build()
				clientMap := newClientMap(restapi.Name, sclient)
				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      restapi.Name,
						Namespace: corev1.NamespaceDefault,
					},
				}
				r := newRestapiController(kubeClient, clientMap)
				b.StartTimer()

				if err := r.Sync(context.TODO(), request); (err != nil) != bb.wantErr {
					b.Errorf("RestapiReconciler.sync() error = %v, wantErr %v", err, bb.wantErr)
				}
			}
		})
	}
}
