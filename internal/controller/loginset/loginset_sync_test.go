// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	builder "github.com/SlinkyProject/slurm-operator/internal/builder/loginbuilder"
	"github.com/SlinkyProject/slurm-operator/internal/utils/refresolver"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

func newLoginsetController(client client.Client) *LoginSetReconciler {
	r := &LoginSetReconciler{
		Client:        client,
		Scheme:        client.Scheme(),
		builder:       builder.New(client),
		refResolver:   refresolver.New(client),
		eventRecorder: events.NewFakeRecorder(10),
	}

	return r
}

func TestLoginsetReconciler_sync(t *testing.T) {
	slurmKey := testutils.NewSlurmKeyRef("slurmkey")
	jwtKey := testutils.NewJwtKeyRef("jwtkey")
	sssdconfRef := testutils.NewSssdConfRef("sssd")
	controller := testutils.NewController("slurm", slurmKey, jwtKey, nil)
	loginset := testutils.NewLoginset("slurm", controller, sssdconfRef)

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
				Client: fake.NewFakeClient(loginset.DeepCopy()),
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
			r := newLoginsetController(tt.fields.Client)
			err := r.Sync(tt.args.ctx, tt.args.request)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func BenchmarkLoginsetReconciler_sync(b *testing.B) {
	slurmKeyRef := testutils.NewSlurmKeyRef("slurmkey")
	slurmKey := testutils.NewSlurmKeySecret(slurmKeyRef)
	jwtKeyRef := testutils.NewJwtKeyRef("jwtkey")
	jwtKey := testutils.NewJwtKeySecret(jwtKeyRef)
	sssdconfRef := testutils.NewSssdConfRef("sssd")

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
				loginset := testutils.NewLoginset("slurm", controller, sssdconfRef)
				kubeClient := fake.NewClientBuilder().WithObjects(
					loginset.DeepCopy(),
					controller.DeepCopy(),
					slurmKey.DeepCopy(),
					jwtKey.DeepCopy(),
				).WithStatusSubresource(&slinkyv1beta1.LoginSet{}).Build()
				request := reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      loginset.Name,
						Namespace: corev1.NamespaceDefault,
					},
				}
				r := newLoginsetController(kubeClient)
				b.StartTimer()

				if err := r.Sync(context.TODO(), request); (err != nil) != bb.wantErr {
					b.Errorf("LoginReconciler.sync() error = %v, wantErr %v", err, bb.wantErr)
				}
			}
		})
	}
}

func TestLoginSetReconciler_syncSshHostKeys(t *testing.T) {
	slurmKey := testutils.NewSlurmKeyRef("slurmkey")
	jwtKey := testutils.NewJwtKeyRef("jwtkey")
	sssdconfRef := testutils.NewSssdConfRef("sssd")
	controller := testutils.NewController("slurm", slurmKey, jwtKey, nil)
	loginset := testutils.NewLoginset("slurm", controller, sssdconfRef)
	key := loginset.SshHostKeys()

	t.Run("creates the Secret when it is absent", func(t *testing.T) {
		c := fake.NewFakeClient(loginset.DeepCopy())
		r := newLoginsetController(c)

		require.NoError(t, r.syncSshHostKeys(context.TODO(), loginset))

		got := &corev1.Secret{}
		require.NoError(t, c.Get(context.TODO(), key, got))
		for _, file := range []string{
			builder.SshHostRsaKeyFile,
			builder.SshHostEd25519KeyFile,
			builder.SshHostEcdsaKeyFile,
		} {
			require.NotEmpty(t, got.Data[file], file)
		}
	})

	// Host keys must survive reconciles even when the live Secret is not marked
	// immutable, e.g. one restored from a backup that dropped the field.
	t.Run("keeps the keys of a mutable existing Secret", func(t *testing.T) {
		existing := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: key.Namespace,
				Name:      key.Name,
			},
			Data: map[string][]byte{
				builder.SshHostRsaKeyFile: []byte("original-host-key"),
			},
		}
		c := fake.NewFakeClient(loginset.DeepCopy(), existing)
		r := newLoginsetController(c)

		require.NoError(t, r.syncSshHostKeys(context.TODO(), loginset))

		got := &corev1.Secret{}
		require.NoError(t, c.Get(context.TODO(), key, got))
		require.Equal(t, "original-host-key", string(got.Data[builder.SshHostRsaKeyFile]))
	})
}
