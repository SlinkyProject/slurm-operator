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
	labelsbuilder "github.com/SlinkyProject/slurm-operator/internal/builder/labels"
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

func TestLoginSetReconciler_syncPodLabels(t *testing.T) {
	ctx := context.TODO()
	slurmKey := testutils.NewSlurmKeyRef("slurmkey")
	jwtKey := testutils.NewJwtKeyRef("jwtkey")
	sssdconfRef := testutils.NewSssdConfRef("sssd")
	controller := testutils.NewController("slurm", slurmKey, jwtKey, nil)
	loginset := testutils.NewLoginset("slurm", controller, sssdconfRef)

	// Build() returns a new map per call, so each pod gets its own label set.
	selectorLabels := func() map[string]string {
		return labelsbuilder.NewBuilder().WithLoginSelectorLabels(loginset).Build()
	}
	newPod := func(name string, podLabels map[string]string) *corev1.Pod {
		return &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: loginset.Namespace,
				Labels:    podLabels,
			},
		}
	}

	unlabeledPod := newPod("login-abc123", selectorLabels())

	staleLabels := selectorLabels()
	staleLabels[slinkyv1beta1.LabelLoginSetPodName] = "login-previous-name"
	stalePod := newPod("login-def456", staleLabels)

	otherPod := newPod("unrelated-pod", map[string]string{"app.kubernetes.io/name": "not-login"})

	kubeClient := fake.NewClientBuilder().WithObjects(
		loginset.DeepCopy(), unlabeledPod.DeepCopy(), stalePod.DeepCopy(), otherPod.DeepCopy(),
	).Build()
	r := newLoginsetController(kubeClient)

	require.NoError(t, r.syncPodLabels(ctx, loginset))

	// Pods selected by the LoginSet are labeled with their own name, including one whose
	// label carried a stale value.
	for _, want := range []*corev1.Pod{unlabeledPod, stalePod} {
		got := &corev1.Pod{}
		require.NoError(t, kubeClient.Get(ctx, client.ObjectKeyFromObject(want), got))
		require.Equal(t, want.Name, got.Labels[slinkyv1beta1.LabelLoginSetPodName])
	}

	// Pods outside the LoginSet's selector are left alone.
	gotOther := &corev1.Pod{}
	require.NoError(t, kubeClient.Get(ctx, client.ObjectKeyFromObject(otherPod), gotOther))
	require.NotContains(t, gotOther.Labels, slinkyv1beta1.LabelLoginSetPodName)

	// Re-running issues no write once the labels already match.
	before := &corev1.Pod{}
	require.NoError(t, kubeClient.Get(ctx, client.ObjectKeyFromObject(unlabeledPod), before))
	require.NoError(t, r.syncPodLabels(ctx, loginset))
	after := &corev1.Pod{}
	require.NoError(t, kubeClient.Get(ctx, client.ObjectKeyFromObject(unlabeledPod), after))
	require.Equal(t, before.ResourceVersion, after.ResourceVersion)
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
