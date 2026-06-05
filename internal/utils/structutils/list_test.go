// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package structutils

import (
	"reflect"
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

func TestSortedDedup(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "empty", in: []string{}, want: []string{}},
		{name: "sorts", in: []string{"c", "a", "b"}, want: []string{"a", "b", "c"}},
		{name: "dedups", in: []string{"a", "a", "b"}, want: []string{"a", "b"}},
		{name: "trims whitespace", in: []string{" a ", "b\t"}, want: []string{"a", "b"}},
		{name: "drops empty and whitespace-only entries", in: []string{"a", "", "  "}, want: []string{"a"}},
		{name: "combined sort dedup trim", in: []string{" b ", "a", "b", ""}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SortedDedup(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SortedDedup() = %v, want %v", got, tt.want)
			}
		})
	}
}
