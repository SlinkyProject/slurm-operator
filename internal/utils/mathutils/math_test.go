// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package mathutils

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

func Test_clamp(t *testing.T) {
	type args struct {
		val int
		min int
		max int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "min < val < max",
			args: args{
				val: 0,
				min: -10,
				max: 10,
			},
			want: 0,
		},
		{
			name: "val < min",
			args: args{
				val: -10,
				min: 0,
				max: 10,
			},
			want: 0,
		},
		{
			name: "val > max",
			args: args{
				val: 10,
				min: -10,
				max: 0,
			},
			want: 0,
		},
		{
			name: "min == val == max",
			args: args{
				val: 0,
				min: 0,
				max: 0,
			},
			want: 0,
		},
		{
			name: "val == min",
			args: args{
				val: 0,
				min: 0,
				max: 10,
			},
			want: 0,
		},
		{
			name: "val == max",
			args: args{
				val: 0,
				min: -10,
				max: 0,
			},
			want: 0,
		},
		{
			name: "max < min",
			args: args{
				val: 0,
				min: 10,
				max: -10,
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clamp(tt.args.val, tt.args.min, tt.args.max); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("clamp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Benchmark_clamp(b *testing.B) {
	type args struct {
		val int
		min int
		max int
	}
	benchmarks := []struct {
		name string
		args args
	}{
		{
			name: "min < val < max",
			args: args{
				val: 0,
				min: -10,
				max: 10,
			},
		},
		{
			name: "val < min",
			args: args{
				val: -10,
				min: 0,
				max: 10,
			},
		},
		{
			name: "val > max",
			args: args{
				val: 10,
				min: -10,
				max: 0,
			},
		},
		{
			name: "min == val == max",
			args: args{
				val: 0,
				min: 0,
				max: 0,
			},
		},
		{
			name: "val == min",
			args: args{
				val: 0,
				min: 0,
				max: 10,
			},
		},
		{
			name: "val == max",
			args: args{
				val: 0,
				min: -10,
				max: 0,
			},
		},
		{
			name: "max < min",
			args: args{
				val: 0,
				min: 10,
				max: -10,
			},
		},
	}

	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				Clamp(bb.args.val, bb.args.min, bb.args.max)
			}
		})
	}
}

func Test_GetScaledValueFromIntOrPercent(t *testing.T) {
	type args struct {
		intOrPercent *intstr.IntOrString
		total        int
		roundUp      bool
		defaultValue int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "default",
			args: args{
				intOrPercent: nil,
				total:        0,
				roundUp:      true,
				defaultValue: 5,
			},
			want: 5,
		},
		{
			name: "50%",
			args: args{
				intOrPercent: ptr.To(intstr.FromString("50%")),
				total:        10,
				roundUp:      true,
				defaultValue: 0,
			},
			want: 5,
		},
		{
			name: "5",
			args: args{
				intOrPercent: ptr.To(intstr.FromInt32(5)),
				total:        10,
				roundUp:      true,
				defaultValue: 0,
			},
			want: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetScaledValueFromIntOrPercent(tt.args.intOrPercent, tt.args.total, tt.args.roundUp, tt.args.defaultValue); got != tt.want {
				t.Errorf("GetScaledValueFromIntOrPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Benchmark_GetScaledValueFromIntOrPercent(b *testing.B) {
	type args struct {
		intOrPercent *intstr.IntOrString
		total        int
		roundUp      bool
		defaultValue int
	}
	benchmarks := []struct {
		name string
		args args
	}{
		{
			name: "default",
			args: args{
				intOrPercent: nil,
				total:        0,
				roundUp:      true,
				defaultValue: 5,
			},
		},
		{
			name: "50%",
			args: args{
				intOrPercent: ptr.To(intstr.FromString("50%")),
				total:        10,
				roundUp:      true,
				defaultValue: 0,
			},
		},
		{
			name: "5",
			args: args{
				intOrPercent: ptr.To(intstr.FromInt32(5)),
				total:        10,
				roundUp:      true,
				defaultValue: 0,
			},
		},
	}

	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			for b.Loop() {
				GetScaledValueFromIntOrPercent(bb.args.intOrPercent, bb.args.total, bb.args.roundUp, bb.args.defaultValue)
			}
		})
	}
}
