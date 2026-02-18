// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package historycontrol

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/kubernetes/pkg/controller/history"
)

func TestSetRevision(t *testing.T) {
	type args struct {
		labels   map[string]string
		revision string
	}
	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{
			name: "empty",
			args: args{
				labels:   nil,
				revision: "",
			},
			want: nil,
		},
		{
			name: "empty map",
			args: args{
				labels:   map[string]string{},
				revision: "",
			},
			want: map[string]string{},
		},
		{
			name: "hash",
			args: args{
				labels:   map[string]string{},
				revision: "00000",
			},
			want: map[string]string{
				history.ControllerRevisionHashLabel: "00000",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetRevision(tt.args.labels, tt.args.revision)
		})
		if diff := cmp.Diff(tt.want, tt.args.labels); diff != "" {
			t.Errorf("unexpected encoded configuration: (-want,+got)\n%s", diff)
		}
	}
}

func TestGetRevision(t *testing.T) {
	type args struct {
		labels map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty",
			args: args{
				labels: nil,
			},
			want: "",
		},
		{
			name: "empty map",
			args: args{
				labels: map[string]string{},
			},
			want: "",
		},
		{
			name: "hash",
			args: args{
				labels: map[string]string{
					history.ControllerRevisionHashLabel: "00000",
				},
			},
			want: "00000",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRevision(tt.args.labels); got != tt.want {
				t.Errorf("GetRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}
