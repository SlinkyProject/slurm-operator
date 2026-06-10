// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package clientmap

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/SlinkyProject/slurm-client/pkg/client"
	"github.com/SlinkyProject/slurm-client/pkg/client/fake"
)

func TestNewClientMap(t *testing.T) {
	tests := []struct {
		name string
		want *ClientMap
	}{
		{
			name: "Test new clusters",
			want: &ClientMap{
				clients: make(map[string]client.Client),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, NewClientMap())
		})
	}
}

func TestClientMap_Get(t *testing.T) {
	testClient := fake.NewFakeClient()
	c := make(map[string]client.Client)
	c["default/foo"] = testClient
	type fields struct {
		clients map[string]client.Client
	}
	type args struct {
		name types.NamespacedName
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   client.Client
	}{
		{
			name: "existing namespaced name",
			fields: fields{
				clients: c,
			},
			args: args{
				name: types.NamespacedName{
					Namespace: "default",
					Name:      "foo",
				},
			},
			want: testClient,
		},
		{
			name: "incorrect namespaced name",
			fields: fields{
				clients: c,
			},
			args: args{
				name: types.NamespacedName{
					Namespace: "default",
					Name:      "bar",
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClientMap{
				lock:    sync.RWMutex{},
				clients: tt.fields.clients,
			}

			require.Equal(t, tt.want, c.Get(tt.args.name))
		})
	}
}

func TestClientMap_add(t *testing.T) {
	testClient := fake.NewFakeClient()
	c := make(map[string]client.Client)
	c["default/foo"] = testClient
	type fields struct {
		clients map[string]client.Client
	}
	type args struct {
		name   types.NamespacedName
		client client.Client
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Already has NamespacedName",
			fields: fields{
				clients: c,
			},
			args: args{
				name: types.NamespacedName{
					Name:      "foo",
					Namespace: "default",
				},
				client: testClient,
			},
			want: false,
		},
		{
			name: "Add a new NamespacedName",
			fields: fields{
				clients: c,
			},
			args: args{
				name: types.NamespacedName{
					Name:      "bar",
					Namespace: "default",
				},
				client: testClient,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClientMap{
				lock:    sync.RWMutex{},
				clients: tt.fields.clients,
			}

			require.Equal(t, tt.want, c.add(tt.args.name, tt.args.client))
		})
	}
}

func TestClientMap_Add(t *testing.T) {
	testClient := fake.NewFakeClient()
	c := make(map[string]client.Client)
	c["default/foo"] = testClient
	type fields struct {
		clients map[string]client.Client
	}
	type args struct {
		name   types.NamespacedName
		client client.Client
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Already has NamespacedName",
			fields: fields{
				clients: c,
			},
			args: args{
				name: types.NamespacedName{
					Name:      "foo",
					Namespace: "default",
				},
				client: testClient,
			},
			want: true,
		},
		{
			name: "Add a new NamespacedName",
			fields: fields{
				clients: c,
			},
			args: args{
				name: types.NamespacedName{
					Name:      "bar",
					Namespace: "default",
				},
				client: testClient,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClientMap{
				lock:    sync.RWMutex{},
				clients: tt.fields.clients,
			}

			require.Equal(t, tt.want, c.Add(tt.args.name, tt.args.client))
		})
	}
}

func TestClientMap_Has(t *testing.T) {
	testClient := fake.NewFakeClient()
	c := make(map[string]client.Client)
	foo := types.NamespacedName{
		Namespace: "default",
		Name:      "foo",
	}
	bar := types.NamespacedName{
		Namespace: "default",
		Name:      "bar",
	}
	c["default/foo"] = testClient

	type fields struct {
		clients map[string]client.Client
	}
	type args struct {
		names []types.NamespacedName
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Does not have NamespacedName",
			fields: fields{
				clients: c,
			},
			args: args{
				names: append([]types.NamespacedName{}, bar),
			},
			want: false,
		},
		{
			name: "Has NamespacedName",
			fields: fields{
				clients: c,
			},
			args: args{
				names: append([]types.NamespacedName{}, bar, foo),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClientMap{
				lock:    sync.RWMutex{},
				clients: tt.fields.clients,
			}

			require.Equal(t, tt.want, c.Has(tt.args.names...))
		})
	}
}

func TestClientMap_Remove(t *testing.T) {
	testClient := fake.NewFakeClient()
	c := make(map[string]client.Client)
	c["default/foo"] = testClient
	type fields struct {
		clients map[string]client.Client
	}
	type args struct {
		name types.NamespacedName
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "Remove client that exists",
			fields: fields{
				clients: c,
			},
			args: args{
				name: types.NamespacedName{
					Name:      "foo",
					Namespace: "default",
				},
			},
			want: true,
		},
		{
			name: "Remove client that does not exists",
			fields: fields{
				clients: c,
			},
			args: args{
				name: types.NamespacedName{
					Name:      "bar",
					Namespace: "default",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClientMap{
				lock:    sync.RWMutex{},
				clients: tt.fields.clients,
			}

			require.Equal(t, tt.want, c.Remove(tt.args.name))
		})
	}
}
