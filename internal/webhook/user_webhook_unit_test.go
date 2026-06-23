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

func TestUserWebhookValidateCreate(t *testing.T) {
	w := &UserWebhook{}
	tests := []struct {
		name    string
		user    *slinkyv1beta1.User
		wantErr bool
	}{
		{
			name: "valid",
			user: &slinkyv1beta1.User{
				Spec: slinkyv1beta1.UserSpec{
					ControllerRef:  corev1.LocalObjectReference{Name: "slurm"},
					DefaultAccount: "research",
					Associations:   []slinkyv1beta1.UserAssociation{{Account: "research"}},
				},
			},
		},
		{
			name: "missing controllerRef",
			user: &slinkyv1beta1.User{
				Spec: slinkyv1beta1.UserSpec{
					DefaultAccount: "research",
					Associations:   []slinkyv1beta1.UserAssociation{{Account: "research"}},
				},
			},
			wantErr: true,
		},
		{
			name: "no associations",
			user: &slinkyv1beta1.User{
				Spec: slinkyv1beta1.UserSpec{
					ControllerRef: corev1.LocalObjectReference{Name: "slurm"},
				},
			},
			wantErr: true,
		},
		{
			name: "defaultAccount not in associations",
			user: &slinkyv1beta1.User{
				Spec: slinkyv1beta1.UserSpec{
					ControllerRef:  corev1.LocalObjectReference{Name: "slurm"},
					DefaultAccount: "missing",
					Associations:   []slinkyv1beta1.UserAssociation{{Account: "research"}},
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate association",
			user: &slinkyv1beta1.User{
				Spec: slinkyv1beta1.UserSpec{
					ControllerRef:  corev1.LocalObjectReference{Name: "slurm"},
					DefaultAccount: "research",
					Associations: []slinkyv1beta1.UserAssociation{
						{Account: "research"},
						{Account: "research"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "defaultQos not in qos",
			user: &slinkyv1beta1.User{
				Spec: slinkyv1beta1.UserSpec{
					ControllerRef:  corev1.LocalObjectReference{Name: "slurm"},
					DefaultAccount: "research",
					Associations: []slinkyv1beta1.UserAssociation{{
						Account: "research",
						Limits: slinkyv1beta1.AssociationLimits{
							QOS:        []string{"normal"},
							DefaultQOS: ptr.To("high"),
						},
					}},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.ValidateCreate(context.Background(), tt.user)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
