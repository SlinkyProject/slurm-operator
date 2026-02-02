// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package structutils

import (
	"reflect"
	"testing"
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
			if got := ReferenceList(tt.args.items); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReferenceList() = %v, want %v", got, tt.want)
			}
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
			if got := DereferenceList(tt.args.items); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DereferenceList() = %v, want %v", got, tt.want)
			}
		})
	}
}
