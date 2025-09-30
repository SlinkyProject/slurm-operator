// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package refresolver

import (
	"context"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(slinkyv1beta1.AddToScheme(scheme))
}

func TestRefResolver_GetController(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		ctx context.Context
		ref slinkyv1beta1.ObjectReference
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
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				ref: slinkyv1beta1.ObjectReference{
					Name:      "slurm",
					Namespace: metav1.NamespaceDefault,
				},
			},
			wantErr: true,
		},
		{
			name: "found",
			fields: fields{
				client: fake.NewClientBuilder().
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
				ref: slinkyv1beta1.ObjectReference{
					Name:      "slurm",
					Namespace: metav1.NamespaceDefault,
				},
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
			r := &RefResolver{
				client: tt.fields.client,
			}
			got, err := r.GetController(tt.args.ctx, tt.args.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefResolver.GetController() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && objectutils.KeyFunc(got) != objectutils.KeyFunc(tt.want) {
				t.Errorf("RefResolver.GetController() = %v, want %v", objectutils.KeyFunc(got), objectutils.KeyFunc(tt.want))
			}
		})
	}
}

func TestRefResolver_GetAccounting(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		ctx context.Context
		ref slinkyv1beta1.ObjectReference
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
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				ref: slinkyv1beta1.ObjectReference{
					Name:      "slurm",
					Namespace: metav1.NamespaceDefault,
				},
			},
			wantErr: true,
		},
		{
			name: "found",
			fields: fields{
				client: fake.NewClientBuilder().
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
				ref: slinkyv1beta1.ObjectReference{
					Name:      "slurm",
					Namespace: metav1.NamespaceDefault,
				},
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
			r := &RefResolver{
				client: tt.fields.client,
			}
			got, err := r.GetAccounting(tt.args.ctx, tt.args.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefResolver.GetAccounting() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil && objectutils.KeyFunc(got) != objectutils.KeyFunc(tt.want) {
				t.Errorf("RefResolver.GetAccounting() = %v, want %v", objectutils.KeyFunc(got), objectutils.KeyFunc(tt.want))
			}
		})
	}
}

func TestRefResolver_GetNodeSetsForController(t *testing.T) {
	type fields struct {
		client client.Client
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
				client: fake.NewClientBuilder().
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
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.NodeSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm-foo",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.NodeSetSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name:      "slurm",
								Namespace: metav1.NamespaceDefault,
							},
						},
					}).
					WithObjects(&slinkyv1beta1.NodeSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm1",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.NodeSetSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name:      "slurm1",
								Namespace: metav1.NamespaceDefault,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RefResolver{
				client: tt.fields.client,
			}
			got, err := r.GetNodeSetsForController(tt.args.ctx, tt.args.controller)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefResolver.GetNodeSetsForController() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got.Items) != tt.want {
				t.Errorf("RefResolver.GetNodeSetsForController() = %v, want %v", len(got.Items), tt.want)
			}
		})
	}
}

func TestRefResolver_GetLoginSetsForController(t *testing.T) {
	type fields struct {
		client client.Client
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
				client: fake.NewClientBuilder().
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
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.LoginSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm-foo",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.LoginSetSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name:      "slurm",
								Namespace: metav1.NamespaceDefault,
							},
						},
					}).
					WithObjects(&slinkyv1beta1.LoginSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm1",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.LoginSetSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name:      "slurm1",
								Namespace: metav1.NamespaceDefault,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RefResolver{
				client: tt.fields.client,
			}
			got, err := r.GetLoginSetsForController(tt.args.ctx, tt.args.controller)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefResolver.GetLoginSetsForController() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got.Items) != tt.want {
				t.Errorf("RefResolver.GetLoginSetsForController() = %v, want %v", len(got.Items), tt.want)
			}
		})
	}
}

func TestRefResolver_GetRestapisForController(t *testing.T) {
	type fields struct {
		client client.Client
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
				client: fake.NewClientBuilder().
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
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.RestApi{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm-foo",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.RestApiSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name:      "slurm",
								Namespace: metav1.NamespaceDefault,
							},
						},
					}).
					WithObjects(&slinkyv1beta1.RestApi{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm1",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.RestApiSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name:      "slurm1",
								Namespace: metav1.NamespaceDefault,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RefResolver{
				client: tt.fields.client,
			}
			got, err := r.GetRestapisForController(tt.args.ctx, tt.args.controller)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefResolver.GetRestapisForController() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got.Items) != tt.want {
				t.Errorf("RefResolver.GetRestapisForController() = %v, want %v", len(got.Items), tt.want)
			}
		})
	}
}

func TestRefResolver_GetControllersForAccounting(t *testing.T) {
	type fields struct {
		client client.Client
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
				client: fake.NewClientBuilder().
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
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm-foo",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.ControllerSpec{
							AccountingRef: slinkyv1beta1.ObjectReference{
								Name:      "slurm",
								Namespace: metav1.NamespaceDefault,
							},
						},
					}).
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "slurm1",
							Namespace: metav1.NamespaceDefault,
						},
						Spec: slinkyv1beta1.ControllerSpec{
							AccountingRef: slinkyv1beta1.ObjectReference{
								Name:      "slurm1",
								Namespace: metav1.NamespaceDefault,
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &RefResolver{
				client: tt.fields.client,
			}
			got, err := r.GetControllersForAccounting(tt.args.ctx, tt.args.accounting)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefResolver.GetControllersForAccounting() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got.Items) != tt.want {
				t.Errorf("RefResolver.GetControllersForAccounting() = %v, want %v", len(got.Items), tt.want)
			}
		})
	}
}

func TestRefResolver_GetSecretKeyRef(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		ctx       context.Context
		selector  *corev1.SecretKeySelector
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
				client: fake.NewClientBuilder().
					WithScheme(scheme).
					Build(),
			},
			args: args{
				ctx: context.TODO(),
				selector: &corev1.SecretKeySelector{
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
				client: fake.NewClientBuilder().
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
				selector: &corev1.SecretKeySelector{
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
			r := &RefResolver{
				client: tt.fields.client,
			}
			got, err := r.GetSecretKeyRef(tt.args.ctx, tt.args.selector, tt.args.namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("RefResolver.GetSecretKeyRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("RefResolver.GetSecretKeyRef() = %v, want %v", got, tt.want)
			}
		})
	}
}
