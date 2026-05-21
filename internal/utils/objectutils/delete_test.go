// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"context"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeleteObject(t *testing.T) {
	type args struct {
		c      client.Client
		ctx    context.Context
		newObj client.Object
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := DeleteObject(tt.args.c, tt.args.ctx, nil, nil, tt.args.newObj); (err != nil) != tt.wantErr {
				t.Errorf("DeleteObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// mockNoMatchClient mocks the controller-runtime Client to return a NoKindMatchError on Get
type mockNoMatchClient struct {
	client.Client
}

func (m *mockNoMatchClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return &meta.NoKindMatchError{
		GroupKind: schema.GroupKind{
			Group: "test.example.com",
			Kind:  "Foo",
		},
		SearchedVersions: []string{"v1"},
	}
}

func TestDeleteObject_NoKindMatch(t *testing.T) {
	ctx := context.Background()

	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-servicemonitor",
			Namespace: "default",
		},
	}

	c := &mockNoMatchClient{}

	err := DeleteObject(c, ctx, nil, nil, serviceMonitor)
	if err != nil {
		t.Fatalf("expected no error (NoMatchError should be ignored), got: %v", err)
	}
}
