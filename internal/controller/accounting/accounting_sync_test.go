// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	builder "github.com/SlinkyProject/slurm-operator/internal/builder/accountingbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newAccountingController(client client.Client) *AccountingReconciler {
	r := &AccountingReconciler{
		Client:        client,
		Scheme:        client.Scheme(),
		builder:       builder.New(client),
		refResolver:   refresolver.New(client),
		eventRecorder: events.NewFakeRecorder(10),
	}

	return r
}

func TestAccountingReconciler_sync(t *testing.T) {
	slurmKey := testutils.NewSlurmKeyRef("slurmkey")
	jwtKey := testutils.NewJwtKeyRef("jwtkey")
	password := testutils.NewPasswordRef("password")
	accounting := testutils.NewAccounting("slurm", slurmKey, jwtKey, password)

	type fields struct {
		Client client.Client
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
				Client: fake.NewFakeClient(accounting.DeepCopy()),
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
			r := newAccountingController(tt.fields.Client)
			if err := r.Sync(tt.args.ctx, tt.args.request); (err != nil) != tt.wantErr {
				t.Errorf("AccountingReconciler.sync() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkAccountingReconciler_sync(b *testing.B) {
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
				accounting := testutils.NewAccounting("slurm", slurmKeyRef, jwtKeyRef, passwordKeyRef)
				kubeClient := fake.NewClientBuilder().WithObjects(
					accounting.DeepCopy(),
					slurmKey.DeepCopy(),
					jwtKey.DeepCopy(),
					passwordKey.DeepCopy(),
				).WithStatusSubresource(&slinkyv1beta1.Accounting{}).Build()
				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      accounting.Name,
						Namespace: corev1.NamespaceDefault,
					},
				}
				r := newAccountingController(kubeClient)
				b.StartTimer()

				if err := r.Sync(context.TODO(), request); (err != nil) != bb.wantErr {
					b.Errorf("AccountingReconciler.sync() error = %v, wantErr %v", err, bb.wantErr)
				}
			}
		})
	}
}
