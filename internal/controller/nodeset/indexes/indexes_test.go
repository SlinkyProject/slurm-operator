// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package indexes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_getPodNodeName(t *testing.T) {
	tests := []struct {
		name string
		o    client.Object
		want []string
	}{
		{
			name: "Pod",
			o:    &corev1.Pod{},
			want: []string{""},
		},
		{
			name: "Pod, with nodeName",
			o: &corev1.Pod{
				Spec: corev1.PodSpec{
					NodeName: "foo",
				},
			},
			want: []string{"foo"},
		},
		{
			name: "invalid",
			o:    &corev1.Node{},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPodNodeName(tt.o)
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("getPodNodeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewFakeClientBuilderWithIndexes(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "smoke",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewFakeClientBuilderWithIndexes()
			if got == nil {
				t.Errorf("NewFakeClientBuilderWithIndexes() = %v", got)
			}
		})
	}
}
