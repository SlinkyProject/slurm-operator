// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package refresolver

import (
	"context"
	"errors"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(slinkyv1beta1.AddToScheme(scheme))
}

func TestRefResolver_GetController(t *testing.T) {
	type fields struct {
		reader client.Reader
	}
	type args struct {
		ctx       context.Context
		ref       corev1.LocalObjectReference
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *slinkyv1beta1.Controller
		wantErr bool
	}{
		{
			name: "not found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				ref: corev1.LocalObjectReference{
					Name: "slurm",
				},
				namespace: metav1.NamespaceDefault,
			},
			wantErr: true,
		},
		{
			name: "found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm",
							Namespace: metav1.NamespaceDefault,
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				ref: corev1.LocalObjectReference{
					Name: "slurm",
				},
				namespace: metav1.NamespaceDefault,
			},
			want: &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slurm",
					Namespace: metav1.NamespaceDefault,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fields.reader)
			got, err := r.GetController(tt.args.ctx, tt.args.ref, tt.args.namespace)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if got != nil {
				require.Equal(t, objectutils.KeyFunc(tt.want), objectutils.KeyFunc(got))
			}
		})
	}
}

func TestRefResolver_GetAccounting(t *testing.T) {
	type fields struct {
		reader client.Reader
	}
	type args struct {
		ctx       context.Context
		ref       corev1.LocalObjectReference
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *slinkyv1beta1.Accounting
		wantErr bool
	}{
		{
			name: "not found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				ref: corev1.LocalObjectReference{
					Name: "slurm",
				},
				namespace: metav1.NamespaceDefault,
			},
			wantErr: true,
		},
		{
			name: "found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.Accounting{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm",
							Namespace: metav1.NamespaceDefault,
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				ref: corev1.LocalObjectReference{
					Name: "slurm",
				},
				namespace: metav1.NamespaceDefault,
			},
			want: &slinkyv1beta1.Accounting{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "slurm",
					Namespace: metav1.NamespaceDefault,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fields.reader)
			got, err := r.GetAccounting(tt.args.ctx, tt.args.ref, tt.args.namespace)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if got != nil {
				require.Equal(t, objectutils.KeyFunc(tt.want), objectutils.KeyFunc(got))
			}
		})
	}
}

func TestRefResolver_GetNodeSetsForController(t *testing.T) {
	type fields struct {
		reader client.Reader
	}
	type args struct {
		ctx        context.Context
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "empty",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want: 0,
		},
		{
			name: "found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.NodeSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm-foo",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.NodeSetSpec{
							ControllerRef: corev1.LocalObjectReference{
								Name: "slurm",
							},
						},
					}).
					WithObjects(&slinkyv1beta1.NodeSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm1",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.NodeSetSpec{
							ControllerRef: corev1.LocalObjectReference{
								Name: "slurm1",
							},
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want: 1,
		},
		{
			name: "list error",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return errors.New("list failed")
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fields.reader)
			got, err := r.GetNodeSetsForController(tt.args.ctx, tt.args.controller)

			if tt.wantErr {
				require.Error(t, err)
				require.NotNil(t, got)
				return
			}

			require.NoError(t, err)
			require.Len(t, got.Items, tt.want)
		})
	}
}

func TestRefResolver_GetLoginSetsForController(t *testing.T) {
	type fields struct {
		reader client.Reader
	}
	type args struct {
		ctx        context.Context
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "empty",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want: 0,
		},
		{
			name: "found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.LoginSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm-foo",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.LoginSetSpec{
							ControllerRef: corev1.LocalObjectReference{
								Name: "slurm",
							},
						},
					}).
					WithObjects(&slinkyv1beta1.LoginSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm1",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.LoginSetSpec{
							ControllerRef: corev1.LocalObjectReference{
								Name: "slurm1",
							},
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want: 1,
		},
		{
			name: "list error",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return errors.New("list failed")
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fields.reader)
			got, err := r.GetLoginSetsForController(tt.args.ctx, tt.args.controller)

			if tt.wantErr {
				require.Error(t, err)
				require.NotNil(t, got)
				return
			}

			require.NoError(t, err)
			require.Len(t, got.Items, tt.want)
		})
	}
}

func TestRefResolver_GetRestapisForController(t *testing.T) {
	type fields struct {
		reader client.Reader
	}
	type args struct {
		ctx        context.Context
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "empty",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want: 0,
		},
		{
			name: "found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.RestApi{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm-foo",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.RestApiSpec{
							ControllerRef: corev1.LocalObjectReference{
								Name: "slurm",
							},
						},
					}).
					WithObjects(&slinkyv1beta1.RestApi{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm1",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.RestApiSpec{
							ControllerRef: corev1.LocalObjectReference{
								Name: "slurm1",
							},
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want: 1,
		},
		{
			name: "list error",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return errors.New("list failed")
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fields.reader)
			got, err := r.GetRestapisForController(tt.args.ctx, tt.args.controller)

			if tt.wantErr {
				require.Error(t, err)
				require.NotNil(t, got)
				return
			}

			require.NoError(t, err)
			require.Len(t, got.Items, tt.want)
		})
	}
}

func TestRefResolver_GetControllersForAccounting(t *testing.T) {
	type fields struct {
		reader client.Reader
	}
	type args struct {
		ctx        context.Context
		accounting *slinkyv1beta1.Accounting
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "empty",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				accounting: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want: 0,
		},
		{
			name: "found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm-foo",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.ControllerSpec{
							AccountingRef: &corev1.LocalObjectReference{
								Name: "slurm",
							},
						},
					}).
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm1",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.ControllerSpec{
							AccountingRef: &corev1.LocalObjectReference{
								Name: "slurm1",
							},
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				accounting: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want: 1,
		},
		{
			name: "list error",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithInterceptorFuncs(interceptor.Funcs{
						List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
							return errors.New("list failed")
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				accounting: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "slurm",
						Namespace: metav1.NamespaceDefault,
					},
				},
			},
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fields.reader)
			got, err := r.GetControllersForAccounting(tt.args.ctx, tt.args.accounting)

			if tt.wantErr {
				require.Error(t, err)
				require.NotNil(t, got)
				return
			}

			require.NoError(t, err)
			require.Len(t, got.Items, tt.want)
		})
	}
}

func TestRefResolver_GetSecretKeyRef(t *testing.T) {
	type fields struct {
		reader client.Reader
	}
	type args struct {
		ctx       context.Context
		selector  corev1.SecretKeySelector
		namespace string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "empty",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				selector: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "secret",
					},
					Key: "password",
				},
				namespace: metav1.NamespaceDefault,
			},
			wantErr: true,
		},
		{
			name: "found",
			fields: fields{
				reader: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "secret",
							Namespace: metav1.NamespaceDefault,
						},
						Data: map[string][]byte{
							"password": []byte("password1"),
						},
					}).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				selector: corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "secret",
					},
					Key: "password",
				},
				namespace: metav1.NamespaceDefault,
			},
			want: []byte("password1"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.fields.reader)
			got, err := r.GetSecretKeyRef(tt.args.ctx, tt.args.selector, tt.args.namespace)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
