// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestCreateObjectIfNotExists(t *testing.T) {
	key := client.ObjectKey{Namespace: "slurm", Name: "foo"}

	newSecret := func(value string) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: key.Namespace,
				Name:      key.Name,
			},
			Data: map[string][]byte{
				"key": []byte(value),
			},
		}
	}

	tests := []struct {
		name      string
		c         client.Client
		buildErr  error
		wantBuilt bool
		wantErr   bool
		wantData  string
	}{
		{
			name:      "builds and creates when object is absent",
			c:         fake.NewFakeClient(),
			wantBuilt: true,
			wantData:  "new",
		},
		{
			name:      "does not build when object already exists",
			c:         fake.NewClientBuilder().WithObjects(newSecret("old")).Build(),
			wantBuilt: false,
			wantData:  "old",
		},
		{
			name:      "returns build error",
			c:         fake.NewFakeClient(),
			buildErr:  errors.New("build failed"),
			wantBuilt: true,
			wantErr:   true,
		},
		{
			name: "does not build when the get fails",
			c: fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return apierrors.NewInternalError(errors.New("boom"))
				},
			}).Build(),
			wantBuilt: false,
			wantErr:   true,
		},
		{
			name: "tolerates a losing create race",
			c: fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					return apierrors.NewAlreadyExists(corev1.Resource("secrets"), key.Name)
				},
			}).Build(),
			wantBuilt: true,
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			built := false
			build := func() (*corev1.Secret, error) {
				built = true
				if tt.buildErr != nil {
					return nil, tt.buildErr
				}
				return newSecret("new"), nil
			}

			eventRecorder := events.NewFakeRecorder(10)
			err := CreateObjectIfNotExists(tt.c, context.TODO(), eventRecorder, newSecret("owner"), key, &corev1.Secret{}, build)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantBuilt, built, "build invoked")

			if tt.wantData == "" {
				return
			}
			got := &corev1.Secret{}
			require.NoError(t, tt.c.Get(context.TODO(), key, got))
			require.Equal(t, tt.wantData, string(got.Data["key"]))
		})
	}
}
