// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildSecret(t *testing.T) {
	type args struct {
		opts  SecretOpts
		owner metav1.Object
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "empty",
			args: args{
				owner: &appsv1.Deployment{},
			},
		},
		{
			name:    "bad owner",
			wantErr: true,
		},
		{
			name: "with options",
			args: args{
				opts: SecretOpts{
					Key: types.NamespacedName{
						Name:      "foo",
						Namespace: "bar",
					},
					Metadata: slinkyv1beta1.Metadata{
						Annotations: map[string]string{
							"foo": "bar",
						},
						Labels: map[string]string{
							"fizz": "buzz",
						},
					},
					Data: map[string][]byte{
						"foo": []byte("bar"),
					},
					StringData: map[string]string{
						"fizz": "buzz",
					},
					Immutable: true,
				},
				owner: &appsv1.Deployment{},
			},
		},
	}
	normSS := func(m map[string]string) map[string]string {
		if m == nil {
			return map[string]string{}
		}
		return m
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(fake.NewFakeClient())
			got, err := b.BuildSecret(tt.args.opts, tt.args.owner)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.args.opts.Key.String(), objectutils.KeyFunc(got))
			require.Equal(t, normSS(tt.args.opts.Metadata.Annotations), got.Annotations)
			require.Equal(t, normSS(tt.args.opts.Metadata.Labels), got.Labels)
			require.Equal(t, tt.args.opts.Immutable, ptr.Deref(got.Immutable, false))
			require.Equal(t, tt.args.opts.Data, got.Data)
			require.Equal(t, tt.args.opts.StringData, got.StringData)
		})
	}
}
