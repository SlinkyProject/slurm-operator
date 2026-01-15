// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginbuilder

import (
	"strings"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildLoginSshConfig(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		loginset *slinkyv1beta1.LoginSet
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
				client: fake.NewFakeClient(),
			},
			args: args{
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.LoginSetSpec{
						RootSshAuthorizedKeys: strings.Join([]string{
							"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx user@example.com",
						}, "\n"),
						ExtraSshdConfig: `LoginGraceTime 600`,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildLoginSshConfig(tt.args.loginset)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildLoginSshConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case got.Data[authorizedKeysFile] == "" && got.BinaryData[authorizedKeysFile] == nil:
				t.Errorf("got.Data[%s] = %v", authorizedKeysFile, got.Data[authorizedKeysFile])

			case got.Data[SshdConfigFile] == "" && got.BinaryData[SshdConfigFile] == nil:
				t.Errorf("got.Data[%s] = %v", SshdConfigFile, got.Data[SshdConfigFile])
			}
		})
	}
}

func BenchmarkBuilder_BuildLoginSshConfig(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		loginset *slinkyv1beta1.LoginSet
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				loginset: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.LoginSetSpec{
						RootSshAuthorizedKeys: strings.Join([]string{
							"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx user@example.com",
						}, "\n"),
						ExtraSshdConfig: `LoginGraceTime 600`,
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			build := New(bb.fields.client)

			for b.Loop() {
				build.BuildLoginSshConfig(bb.args.loginset) //nolint:errcheck
			}
		})
	}
}
