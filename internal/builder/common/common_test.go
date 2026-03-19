// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	_ "embed"
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_mergeEnvVar(t *testing.T) {
	type args struct {
		envVarList1 []corev1.EnvVar
		envVarList2 []corev1.EnvVar
		sep         string
	}
	tests := []struct {
		name string
		args args
		want []corev1.EnvVar
	}{
		{
			name: "empty",
			args: args{},
			want: []corev1.EnvVar{},
		},
		{
			name: "list 1",
			args: args{
				envVarList1: []corev1.EnvVar{
					{Name: "foo", Value: "bar"},
				},
				envVarList2: []corev1.EnvVar{},
				sep:         ",",
			},
			want: []corev1.EnvVar{
				{Name: "foo", Value: "bar"},
			},
		},
		{
			name: "list 2",
			args: args{
				envVarList1: []corev1.EnvVar{},
				envVarList2: []corev1.EnvVar{
					{Name: "fizz", Value: "buzz"},
				},
				sep: ",",
			},
			want: []corev1.EnvVar{
				{Name: "fizz", Value: "buzz"},
			},
		},
		{
			name: "both",
			args: args{
				envVarList1: []corev1.EnvVar{
					{Name: "foo", Value: "bar"},
				},
				envVarList2: []corev1.EnvVar{
					{Name: "fizz", Value: "buzz"},
				},
				sep: ",",
			},
			want: []corev1.EnvVar{
				{Name: "fizz", Value: "buzz"},
				{Name: "foo", Value: "bar"},
			},
		},
		{
			name: "append",
			args: args{
				envVarList1: []corev1.EnvVar{
					{Name: "foo", Value: "bar"},
					{Name: "fizz", Value: "buzz"},
				},
				envVarList2: []corev1.EnvVar{
					{Name: "foo", Value: "baz"},
				},
				sep: ",",
			},
			want: []corev1.EnvVar{
				{Name: "fizz", Value: "buzz"},
				{Name: "foo", Value: "bar,baz"},
			},
		},
		{
			name: "overwrite",
			args: args{
				envVarList1: []corev1.EnvVar{
					{Name: "foo", Value: "bar"},
					{Name: "foo", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "config",
							},
							Key: "key",
						},
					}},
				},
				envVarList2: []corev1.EnvVar{
					{Name: "fizz", Value: "buzz"},
					{Name: "foo", ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "config",
							},
							Key: "key",
						},
					}},
				},
				sep: ",",
			},
			want: []corev1.EnvVar{
				{Name: "fizz", Value: "buzz"},
				{Name: "foo", ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "config",
						},
						Key: "key",
					},
				}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeEnvVar(tt.args.envVarList1, tt.args.envVarList2, tt.args.sep)
			sort.SliceStable(got, func(i, j int) bool {
				item1 := got[i]
				item2 := got[j]
				return item1.Name < item2.Name
			})
			sort.SliceStable(tt.want, func(i, j int) bool {
				item1 := tt.want[i]
				item2 := tt.want[j]
				return item1.Name < item2.Name
			})
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("mergeEnvVar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommonBuilder_GetContainerResourceLimits(t *testing.T) {
	client := fake.NewFakeClient()

	cpu1, err := resource.ParseQuantity("1")
	if err != nil {
		t.Fatalf("Failed to call resource.ParseQuantity")
	}

	mem1g, err := resource.ParseQuantity("1Gi")
	if err != nil {
		t.Fatalf("Failed to call resource.ParseQuantity")
	}

	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		container corev1.Container
		want      int64
		want2     int64
	}{
		{
			name:      "default",
			container: corev1.Container{},
			want:      0,
			want2:     0,
		},
		{
			name: "limits set",
			container: corev1.Container{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"cpu":    cpu1,
						"memory": mem1g,
					},
				},
			},
			want:  1,
			want2: 1073741824,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(client)
			got, got2 := b.GetContainerResourceLimits(tt.container)
			if got != tt.want {
				t.Errorf("GetContainerResourceLimits() = %v, want %v", got, tt.want)
			}
			if got2 != tt.want2 {
				t.Errorf("GetContainerResourceLimits() = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func TestCommonBuilder_GetPodResourceLimits(t *testing.T) {
	client := fake.NewFakeClient()

	cpu1, err := resource.ParseQuantity("1")
	if err != nil {
		t.Fatalf("Failed to call resource.ParseQuantity")
	}

	mem1g, err := resource.ParseQuantity("1Gi")
	if err != nil {
		t.Fatalf("Failed to call resource.ParseQuantity")
	}

	tests := []struct {
		name  string // description of this test case
		pod   corev1.PodSpec
		want  int64
		want2 int64
	}{
		{
			name:  "default",
			pod:   corev1.PodSpec{},
			want:  0,
			want2: 0,
		},
		{
			name: "limits set",
			pod: corev1.PodSpec{
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"cpu":    cpu1,
						"memory": mem1g,
					},
				},
			},
			want:  1,
			want2: 1073741824,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(client)
			got, got2 := b.GetPodResourceLimits(tt.pod)
			if got != tt.want {
				t.Errorf("GetPodResourceLimits() = %v, want %v", got, tt.want)
			}
			if got2 != tt.want2 {
				t.Errorf("GetPodResourceLimits() = %v, want %v", got2, tt.want2)
			}
		})
	}
}

func Test_BuildMergedConfig(t *testing.T) {
	tests := []struct {
		name            string
		confRaw         string
		mergeParameters map[string][]string
		want            string
	}{
		{
			name:            "empty",
			confRaw:         ``,
			mergeParameters: make(map[string][]string),
			want:            ``,
		},
		{
			name:    "empty, with mergeParams",
			confRaw: ``,
			mergeParameters: map[string][]string{
				"Foo": {"bar", "baz"},
			},
			want: ``,
		},
		{
			name:    "merge with mergeParams",
			confRaw: `Foo=fizz,buzz`,
			mergeParameters: map[string][]string{
				"Foo": {"bar", "baz"},
			},
			want: `Foo=bar,baz,buzz,fizz`,
		},
		{
			name:    "trailing comma",
			confRaw: `Foo=fizz,buzz,`,
			mergeParameters: map[string][]string{
				"Foo": {"bar", "baz"},
			},
			want: `Foo=bar,baz,buzz,fizz`,
		},
		{
			name:    "merge, with overlap",
			confRaw: `Foo=fizz,overlap,buzz`,
			mergeParameters: map[string][]string{
				"Foo": {"bar", "overlap", "baz"},
			},
			want: `Foo=bar,baz,buzz,fizz,overlap`,
		},
		{
			name: "merge multiple, with overlap",
			confRaw: `Stuff0=junk
Foo=bar,overlap
Stuff1=junk
Fizz=buzz,overlap
Stuff2=junk`,
			mergeParameters: map[string][]string{
				"Foo":   {"thing", "overlap"},
				"Fizz":  {"thing", "overlap"},
				"Other": {"thing"},
			},
			want: `Fizz=buzz,overlap,thing
Foo=bar,overlap,thing`,
		},
		{
			name: "overlap, mixed case",
			confRaw: `Stuff0=junk
foo=bar,overlap
Stuff1=junk
fizz=buzz,overlap
Stuff2=junk`,
			mergeParameters: map[string][]string{
				"Foo":   {"Thing", "Overlap"},
				"Fizz":  {"Thing", "Overlap"},
				"Other": {"Thing"},
			},
			want: `Fizz=Overlap,Thing,buzz
Foo=Overlap,Thing,bar`,
		},
		{
			name:    "kv parameters",
			confRaw: `Foo=bar,opt=1`,
			mergeParameters: map[string][]string{
				"Foo": {"this", "that=2"},
			},
			want: `Foo=bar,opt=1,that=2,this`,
		},
		{
			name:    "kv parameters overlap",
			confRaw: `Foo=overlap=1`,
			mergeParameters: map[string][]string{
				"Foo": {"overlap=0"},
			},
			want: `Foo=overlap=0`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildMergedConfig(tt.confRaw, tt.mergeParameters)
			if got != tt.want {
				t.Errorf("buildMergedParameters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isOptionOverlap(t *testing.T) {
	tests := []struct {
		name  string
		item1 string
		item2 string
		want  bool
	}{
		{
			name:  "empty",
			item1: "",
			item2: "",
			want:  true,
		},
		{
			name:  "identical",
			item1: "foo=bar",
			item2: "foo=bar",
			want:  true,
		},
		{
			name:  "different",
			item1: "foo=bar",
			item2: "fizz=buzz",
			want:  false,
		},
		{
			name:  "overlap",
			item1: "foo=bar",
			item2: "foo=baz",
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOptionOverlap(tt.item1, tt.item2)
			if got != tt.want {
				t.Errorf("isOptionOverlap() = %v, want %v", got, tt.want)
			}
		})
	}
}
