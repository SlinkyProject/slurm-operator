// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildWorkerSshConfig(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		nodeset *slinkyv1beta1.NodeSet
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
				nodeset: &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.NodeSetSpec{
						Ssh: slinkyv1beta1.NodeSetSsh{
							ExtraSshdConfig: `LoginGraceTime 600`,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildWorkerSshConfig(tt.args.nodeset)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildWorkerSshConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case got.Data[sshdConfigFile] == "" && got.BinaryData[sshdConfigFile] == nil:
				t.Errorf("got.Data[%s] = %v", sshdConfigFile, got.Data[sshdConfigFile])
			}
		})
	}
}
