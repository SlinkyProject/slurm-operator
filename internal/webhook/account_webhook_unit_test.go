// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestAccountWebhookValidateCreate(t *testing.T) {
	w := &AccountWebhook{}
	tests := []struct {
		name    string
		account *slinkyv1beta1.Account
		wantErr bool
	}{
		{
			name: "valid",
			account: &slinkyv1beta1.Account{
				Spec: slinkyv1beta1.AccountSpec{
					ControllerRef: corev1.LocalObjectReference{Name: "slurm"},
				},
			},
		},
		{
			name:    "missing controllerRef",
			account: &slinkyv1beta1.Account{},
			wantErr: true,
		},
		{
			name: "defaultQos not in qos",
			account: &slinkyv1beta1.Account{
				Spec: slinkyv1beta1.AccountSpec{
					ControllerRef: corev1.LocalObjectReference{Name: "slurm"},
					Limits: slinkyv1beta1.AssociationLimits{
						QOS:        []string{"normal"},
						DefaultQOS: ptr.To("high"),
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.ValidateCreate(context.Background(), tt.account)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAccountWebhookValidateUpdateControllerRefImmutable(t *testing.T) {
	w := &AccountWebhook{}
	old := &slinkyv1beta1.Account{
		Spec: slinkyv1beta1.AccountSpec{ControllerRef: corev1.LocalObjectReference{Name: "slurm"}},
	}
	updated := &slinkyv1beta1.Account{
		Spec: slinkyv1beta1.AccountSpec{ControllerRef: corev1.LocalObjectReference{Name: "other"}},
	}
	if _, err := w.ValidateUpdate(context.Background(), old, updated); err == nil {
		t.Fatal("expected error changing controllerRef, got nil")
	}
}
