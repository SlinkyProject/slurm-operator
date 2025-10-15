// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package podinfo

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"
)

func TestPodInfo_Equal(t *testing.T) {
	type fields struct {
		Namespace string
		PodName   string
		Node      string
	}
	type args struct {
		cmp PodInfo
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name:   "Empty",
			fields: fields{},
			args: args{
				cmp: PodInfo{},
			},
			want: true,
		},
		{
			name: "Populated",
			fields: fields{
				Namespace: corev1.NamespaceDefault,
				PodName:   "foo",
			},
			args: args{
				cmp: PodInfo{
					Namespace: corev1.NamespaceDefault,
					PodName:   "foo",
				},
			},
			want: true,
		},
		{
			name: "Mismatch",
			fields: fields{
				Namespace: corev1.NamespaceDefault,
				PodName:   "foo",
			},
			args: args{
				cmp: PodInfo{},
			},
			want: false,
		},
		{
			name: "Mismatch Name",
			fields: fields{
				Namespace: corev1.NamespaceDefault,
				PodName:   "foo",
			},
			args: args{
				cmp: PodInfo{
					Namespace: corev1.NamespaceDefault,
					PodName:   "bar",
				},
			},
			want: false,
		},
		{
			name: "Mismatch Namespace",
			fields: fields{
				Namespace: corev1.NamespaceDefault,
				PodName:   "foo",
			},
			args: args{
				cmp: PodInfo{
					Namespace: "bar",
					PodName:   "foo",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podInfo := &PodInfo{
				Namespace: tt.fields.Namespace,
				PodName:   tt.fields.PodName,
			}
			if got := podInfo.Equal(tt.args.cmp); got != tt.want {
				t.Errorf("PodInfo.Equal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPodInfo_ToString(t *testing.T) {
	type fields struct {
		Namespace string
		PodName   string
		Node      string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "Empty",
			fields: fields{},
			want:   `{"namespace":"","podName":"","node":""}`,
		},
		{
			name: "Populated",
			fields: fields{
				Namespace: corev1.NamespaceDefault,
				PodName:   "foo",
				Node:      "bar",
			},
			want: `{"namespace":"default","podName":"foo","node":"bar"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podInfo := &PodInfo{
				Namespace: tt.fields.Namespace,
				PodName:   tt.fields.PodName,
				Node:      tt.fields.Node,
			}
			if got := podInfo.ToString(); got != tt.want {
				t.Errorf("PodInfo.ToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseIntoPodInfo(t *testing.T) {
	type args struct {
		str *string
		out *PodInfo
	}
	tests := []struct {
		name    string
		args    args
		want    *PodInfo
		wantErr bool
	}{
		{
			name: "Empty string",
			args: args{
				str: ptr.To(""),
				out: &PodInfo{},
			},
			want:    &PodInfo{},
			wantErr: true,
		},
		{
			name: "Empty values",
			args: args{
				str: ptr.To(`{"namespace":"","podName":"","node":""}`),
				out: &PodInfo{},
			},
			want:    &PodInfo{},
			wantErr: false,
		},
		{
			name: "Overwrite PodInfo",
			args: args{
				str: ptr.To(`{"namespace":"default","podName":"foo","node":"foo"}`),
				out: &PodInfo{
					Namespace: "baz",
					PodName:   "bar",
					Node:      "bar",
				},
			},
			want: &PodInfo{
				Namespace: "default",
				PodName:   "foo",
				Node:      "foo",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ParseIntoPodInfo(tt.args.str, tt.args.out); (err != nil) != tt.wantErr {
				t.Errorf("ParseIntoPodInfo() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got := tt.args.out; !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("ParseIntoPodInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
