// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package structutils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReferenceList(t *testing.T) {
	var foo, bar any
	foo, bar = "foo", "bar"
	list := make([]*any, 0, 2)
	list = append(list, &foo, &bar)
	type args struct {
		items []any
	}
	tests := []struct {
		name string
		args args
		want []*any
	}{
		{
			name: "Test empty",
			args: args{
				items: []any{},
			},
			want: []*any{},
		},
		{
			name: "Test two elements",
			args: args{
				items: []any{foo, bar},
			},
			want: list,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ReferenceList(tt.args.items))
		})
	}
}

func TestDereferenceList(t *testing.T) {
	var foo, bar any
	foo, bar = "foo", "bar"
	list := make([]*any, 0, 2)
	list = append(list, &foo, &bar)
	nilList := make([]*any, 0, 1)
	nilList = append(nilList, nil)
	type args struct {
		items []*any
	}
	tests := []struct {
		name string
		args args
		want []any
	}{
		{
			name: "Test empty",
			args: args{
				items: []*any{},
			},
			want: []any{},
		},
		{
			name: "Test two elements",
			args: args{
				items: list,
			},
			want: []any{foo, bar},
		},
		{
			name: "Test nil element",
			args: args{
				items: nilList,
			},
			want: []any{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, DereferenceList(tt.args.items))
		})
	}
}
