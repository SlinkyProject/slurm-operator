// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"testing"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	utilruntime.Must(slinkyv1beta1.AddToScheme(scheme.Scheme))
}

func TestSyncObject(t *testing.T) {
	type args struct {
		c            client.Client
		ctx          context.Context
		newObj       client.Object
		shouldUpdate bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ConfigMap",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Secret",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Service",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Deployment",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "StatefulSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Controller",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Restapi",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.RestApi{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "Accounting",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "NodeSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
		{
			name: "LoginSet",
			args: args{
				c:   fake.NewFakeClient(),
				ctx: context.TODO(),
				newObj: &slinkyv1beta1.LoginSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				shouldUpdate: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := SyncObject(tt.args.c, tt.args.ctx, tt.args.newObj, tt.args.shouldUpdate); (err != nil) != tt.wantErr {
				t.Errorf("SyncObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
