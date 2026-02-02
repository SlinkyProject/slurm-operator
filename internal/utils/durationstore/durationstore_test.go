// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package durationstore

import (
	"testing"
	"time"
)

func TestDurationStore_Push(t *testing.T) {
	type args struct {
		key    string
		newDur time.Duration
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "[foo] Push time.Second",
			args: args{
				key:    "foo",
				newDur: time.Second,
			},
		},
		{
			name: "[foo] Push (-1 *time.Minute)",
			args: args{
				key:    "foo",
				newDur: (-1 * time.Minute),
			},
		},
		{
			name: "[bar] Push (-1 * time.Hour)",
			args: args{
				key:    "bar",
				newDur: (-1 * time.Hour),
			},
		},
		{
			name: "[bar] Push (time.Minute)",
			args: args{
				key:    "bar",
				newDur: time.Minute,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := NewDurationStore(Greater)
			ds.Push(tt.args.key, tt.args.newDur)
			if got := ds.Pop(tt.args.key); got != tt.args.newDur {
				t.Errorf("ds.Pop() = %v, want %v", got, tt.args.newDur)
			}
		})
	}
}

func TestDurationStore_Peek(t *testing.T) {
	ds := NewDurationStore(Greater)
	type args struct {
		key    string
		newDur time.Duration
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "[foo] Push time.Second",
			args: args{
				key:    "foo",
				newDur: time.Second,
			},
			want: time.Second,
		},
		{
			name: "[foo] Push (-1 *time.Minute)",
			args: args{
				key:    "foo",
				newDur: (-1 * time.Minute),
			},
			want: time.Second,
		},
		{
			name: "[foo] Push time.Minute",
			args: args{
				key:    "foo",
				newDur: time.Minute,
			},
			want: time.Minute,
		},
		{
			name: "[bar] Push (-1 * time.Hour)",
			args: args{
				key:    "bar",
				newDur: (-1 * time.Hour),
			},
			want: (-1 * time.Hour),
		},
		{
			name: "[bar] Push (-1 * time.Minute)",
			args: args{
				key:    "bar",
				newDur: (-1 * time.Minute),
			},
			want: (-1 * time.Minute),
		},
		{
			name: "[bar] Push (-1 * time.Second)",
			args: args{
				key:    "bar",
				newDur: (-1 * time.Second),
			},
			want: (-1 * time.Second),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds.Push(tt.args.key, tt.args.newDur)
			if got := ds.Peek(tt.args.key); got != tt.want {
				t.Errorf("ds.Peek() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDurationStore_Pop(t *testing.T) {
	durationStore := NewDurationStore(Greater)
	durationStore.Push("bar", time.Minute)
	durationStore.Push("baz", (-1 * time.Hour))

	type args struct {
		key string
	}
	tests := []struct {
		name string
		ds   *DurationStore
		args args
		want time.Duration
	}{
		{
			name: "[foo] Pop empty",
			ds:   durationStore,
			args: args{
				key: "foo",
			},
			want: 0,
		},
		{
			name: "[foo] Pop again",
			ds:   durationStore,
			args: args{
				key: "foo",
			},
			want: 0,
		},
		{
			name: "[bar] Pop time.Minute",
			ds:   durationStore,
			args: args{
				key: "bar",
			},
			want: time.Minute,
		},
		{
			name: "[bar] Pop again",
			ds:   durationStore,
			args: args{
				key: "bar",
			},
			want: 0,
		},
		{
			name: "[baz] Pop (-1 * time.Hour)",
			ds:   durationStore,
			args: args{
				key: "baz",
			},
			want: (-1 * time.Hour),
		},
		{
			name: "[baz] Pop again",
			ds:   durationStore,
			args: args{
				key: "baz",
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ds.Pop(tt.args.key); got != tt.want {
				t.Errorf("DurationStore.Pop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_duration_Update(t *testing.T) {
	type fields struct {
		dur time.Duration
	}
	type args struct {
		newDur time.Duration
		eval   func(dur1, dur2 time.Duration) bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   time.Duration
	}{
		{
			name: "[Greater] max(0, time.Minute)",
			fields: fields{
				dur: 0,
			},
			args: args{
				newDur: time.Second,
				eval:   Greater,
			},
			want: time.Second,
		},
		{
			name: "[Greater] max(time.Minute, 0)",
			fields: fields{
				dur: time.Second,
			},
			args: args{
				newDur: 0,
				eval:   Greater,
			},
			want: time.Second,
		},
		{
			name: "[Less] min(0, time.Second)",
			fields: fields{
				dur: 0,
			},
			args: args{
				newDur: time.Second,
				eval:   Less,
			},
			want: 0,
		},
		{
			name: "[Less] min(time.Second, 0)",
			fields: fields{
				dur: time.Second,
			},
			args: args{
				newDur: 0,
				eval:   Less,
			},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &duration{
				dur: tt.fields.dur,
			}
			d.Update(tt.args.newDur, tt.args.eval)
			if got := d.dur; got != tt.want {
				t.Errorf("DurationStore.Pop() = %v, want %v", got, tt.want)
			}
		})
	}
}
