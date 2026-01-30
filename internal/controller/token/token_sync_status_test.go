// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
)

func TestTokenReconciler_syncStatus(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx   context.Context
		token *slinkyv1beta1.Token
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReconciler(tt.fields.Client)
			if err := r.syncStatus(tt.args.ctx, tt.args.token); (err != nil) != tt.wantErr {
				t.Errorf("TokenReconciler.syncStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTokenReconciler_updateStatus(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		ctx       context.Context
		token     *slinkyv1beta1.Token
		newStatus *slinkyv1beta1.TokenStatus
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReconciler(tt.fields.Client)
			if err := r.updateStatus(tt.args.ctx, tt.args.token, tt.args.newStatus); (err != nil) != tt.wantErr {
				t.Errorf("TokenReconciler.updateStatus() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
