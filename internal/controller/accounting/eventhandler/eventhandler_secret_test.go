// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	"context"
	"testing"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

func Test_SecretEventHandler_Create(t *testing.T) {
	name := "slurm"
	slurmKeyRef := testutils.NewSlurmKeyRef(name)
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
	slurmKeySecret := testutils.NewSlurmKeySecret(slurmKeyRef)
	jwtHs256KeySecret := testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
	controller := testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
	passwordRef := testutils.NewPasswordRef(name)
	passwordSecret := testutils.NewPasswordSecret(passwordRef)
	accounting := testutils.NewAccounting(name, slurmKeyRef, jwtHs256KeyRef, passwordRef)
	type fields struct {
		Reader client.Reader
	}
	type args struct {
		ctx context.Context
		evt event.CreateEvent
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "slurm key",
			fields: fields{
				Reader: fake.NewFakeClient(
					slurmKeySecret,
					jwtHs256KeySecret,
					controller,
					passwordSecret,
					accounting,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: slurmKeySecret,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "hs256 key",
			fields: fields{
				Reader: fake.NewFakeClient(
					slurmKeySecret,
					jwtHs256KeySecret,
					controller,
					passwordSecret,
					accounting,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.CreateEvent{
					Object: jwtHs256KeySecret,
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSecretEventHandler(tt.fields.Reader)
			h.Create(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("SecretEventHandler.Create() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_SecretEventHandler_Delete(t *testing.T) {
	name := "slurm"
	slurmKeyRef := testutils.NewSlurmKeyRef(name)
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
	slurmKeySecret := testutils.NewSlurmKeySecret(slurmKeyRef)
	jwtHs256KeySecret := testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
	controller := testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
	passwordRef := testutils.NewPasswordRef(name)
	passwordSecret := testutils.NewPasswordSecret(passwordRef)
	accounting := testutils.NewAccounting(name, slurmKeyRef, jwtHs256KeyRef, passwordRef)
	type fields struct {
		Reader client.Reader
	}
	type args struct {
		ctx context.Context
		evt event.DeleteEvent
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "slurm key",
			fields: fields{
				Reader: fake.NewFakeClient(
					slurmKeySecret,
					controller,
					passwordSecret,
					accounting,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: slurmKeySecret,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "hs256 key",
			fields: fields{
				Reader: fake.NewFakeClient(
					slurmKeySecret,
					jwtHs256KeySecret,
					controller,
					passwordSecret,
					accounting,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.DeleteEvent{
					Object: jwtHs256KeySecret,
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSecretEventHandler(tt.fields.Reader)
			h.Delete(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("SecretEventHandler.Delete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_SecretEventHandler_Generic(t *testing.T) {
	type fields struct {
		Reader client.Reader
	}
	type args struct {
		ctx context.Context
		evt event.GenericEvent
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "Empty",
			fields: fields{
				Reader: fake.NewFakeClient(),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.GenericEvent{},
				q:   newQueue(),
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSecretEventHandler(tt.fields.Reader)
			h.Generic(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("SecretEventHandler.Generic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_SecretEventHandler_Update(t *testing.T) {
	name := "slurm"
	slurmKeyRef := testutils.NewSlurmKeyRef(name)
	jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
	slurmKeySecret := testutils.NewSlurmKeySecret(slurmKeyRef)
	jwtHs256KeySecret := testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
	controller := testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
	passwordRef := testutils.NewPasswordRef(name)
	passwordSecret := testutils.NewPasswordSecret(passwordRef)
	accounting := testutils.NewAccounting(name, slurmKeyRef, jwtHs256KeyRef, passwordRef)
	type fields struct {
		Reader client.Reader
	}
	type args struct {
		ctx context.Context
		evt event.UpdateEvent
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   int
	}{
		{
			name: "slurm key",
			fields: fields{
				Reader: fake.NewFakeClient(
					slurmKeySecret,
					controller,
					passwordSecret,
					accounting,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: slurmKeySecret,
					ObjectNew: slurmKeySecret,
				},
				q: newQueue(),
			},
			want: 1,
		},
		{
			name: "hs256 key",
			fields: fields{
				Reader: fake.NewFakeClient(
					slurmKeySecret,
					jwtHs256KeySecret,
					controller,
					passwordSecret,
					accounting,
				),
			},
			args: args{
				ctx: context.TODO(),
				evt: event.UpdateEvent{
					ObjectOld: jwtHs256KeySecret,
					ObjectNew: jwtHs256KeySecret,
				},
				q: newQueue(),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSecretEventHandler(tt.fields.Reader)
			h.Update(tt.args.ctx, tt.args.evt, tt.args.q)
			if got := tt.args.q.Len(); got != tt.want {
				t.Errorf("SecretEventHandler.Update() = %v, want %v", got, tt.want)
			}
		})
	}
}
