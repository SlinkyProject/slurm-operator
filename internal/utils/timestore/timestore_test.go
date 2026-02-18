// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package timestore

import (
	"testing"
	"time"
)

func TestTimeStore_Push(t *testing.T) {
	now := time.Now()
	type args struct {
		key     string
		newTime time.Time
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "[foo] Push time.Now",
			args: args{
				key:     "foo",
				newTime: time.Now(),
			},
		},
		{
			name: "[foo] Push (-1 *time.Minute)",
			args: args{
				key:     "foo",
				newTime: now.Add(-1 * time.Minute),
			},
		},
		{
			name: "[bar] Push (-1 * time.Hour)",
			args: args{
				key:     "bar",
				newTime: now.Add(-1 * time.Hour),
			},
		},
		{
			name: "[bar] Push (time.Minute)",
			args: args{
				key:     "bar",
				newTime: now.Add(time.Minute),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := NewTimeStore(Greater)
			ts.Push(tt.args.key, tt.args.newTime)
			if got := ts.Pop(tt.args.key); got != tt.args.newTime {
				t.Errorf("ts.Pop() = %v, want %v", got, tt.args.newTime)
			}
		})
	}
}

func TestTimeStore_Peek(t *testing.T) {
	now := time.Now()
	ts := NewTimeStore(Greater)
	type args struct {
		key     string
		newTime time.Time
	}
	tests := []struct {
		name string
		args args
		want time.Time
	}{
		{
			name: "[foo] Push time.Second",
			args: args{
				key:     "foo",
				newTime: now.Add(time.Second),
			},
			want: now.Add(time.Second),
		},
		{
			name: "[foo] Push (-1 *time.Minute)",
			args: args{
				key:     "foo",
				newTime: now.Add(-1 * time.Minute),
			},
			want: now.Add(time.Second),
		},
		{
			name: "[foo] Push time.Minute",
			args: args{
				key:     "foo",
				newTime: now.Add(time.Minute),
			},
			want: now.Add(time.Minute),
		},
		{
			name: "[bar] Push (-1 * time.Hour)",
			args: args{
				key:     "bar",
				newTime: now.Add(-1 * time.Hour),
			},
			want: now.Add(-1 * time.Hour),
		},
		{
			name: "[bar] Push (-1 * time.Minute)",
			args: args{
				key:     "bar",
				newTime: now.Add(-1 * time.Minute),
			},
			want: now.Add(-1 * time.Minute),
		},
		{
			name: "[bar] Push (-1 * time.Second)",
			args: args{
				key:     "bar",
				newTime: now.Add(-1 * time.Second),
			},
			want: now.Add(-1 * time.Second),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts.Push(tt.args.key, tt.args.newTime)
			if got := ts.Peek(tt.args.key); got != tt.want {
				t.Errorf("ts.Peek() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimeStore_Pop(t *testing.T) {
	now := time.Now()
	timeStore := NewTimeStore(Greater)
	timeStore.Push("bar", now.Add(time.Minute))
	timeStore.Push("baz", now.Add(-1*time.Hour))

	type args struct {
		key string
	}
	tests := []struct {
		name string
		ts   *TimeStore
		args args
		want time.Time
	}{
		{
			name: "[foo] Pop empty",
			ts:   timeStore,
			args: args{
				key: "foo",
			},
			want: time.Time{},
		},
		{
			name: "[foo] Pop again",
			ts:   timeStore,
			args: args{
				key: "foo",
			},
			want: time.Time{},
		},
		{
			name: "[bar] Pop time.Minute",
			ts:   timeStore,
			args: args{
				key: "bar",
			},
			want: now.Add(time.Minute),
		},
		{
			name: "[bar] Pop again",
			ts:   timeStore,
			args: args{
				key: "bar",
			},
			want: time.Time{},
		},
		{
			name: "[baz] Pop (-1 * time.Hour)",
			ts:   timeStore,
			args: args{
				key: "baz",
			},
			want: now.Add(-1 * time.Hour),
		},
		{
			name: "[baz] Pop again",
			ts:   timeStore,
			args: args{
				key: "baz",
			},
			want: time.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ts.Pop(tt.args.key); got != tt.want {
				t.Errorf("TimeStore.Pop() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_duration_Update(t *testing.T) {
	now := time.Now()
	type fielts struct {
		t time.Time
	}
	type args struct {
		newTime time.Time
		eval    func(dur1, dur2 time.Time) bool
	}
	tests := []struct {
		name   string
		fielts fielts
		args   args
		want   time.Time
	}{
		{
			name: "[Greater] max(0, time.Minute)",
			fielts: fielts{
				t: time.Time{},
			},
			args: args{
				newTime: now.Add(time.Second),
				eval:    Greater,
			},
			want: now.Add(time.Second),
		},
		{
			name: "[Greater] max(now.Add(time.Minute), 0)",
			fielts: fielts{
				t: now.Add(time.Second),
			},
			args: args{
				newTime: time.Time{},
				eval:    Greater,
			},
			want: now.Add(time.Second),
		},
		{
			name: "[Less] min(0, time.Second)",
			fielts: fielts{
				t: time.Time{},
			},
			args: args{
				newTime: now.Add(time.Second),
				eval:    Less,
			},
			want: time.Time{},
		},
		{
			name: "[Less] min(now.Add(time.Second), 0)",
			fielts: fielts{
				t: now.Add(time.Second),
			},
			args: args{
				newTime: time.Time{},
				eval:    Less,
			},
			want: time.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &timestore{
				t: tt.fielts.t,
			}
			d.Update(tt.args.newTime, tt.args.eval)
			if got := d.t; got != tt.want {
				t.Errorf("TimeStore.Pop() = %v, want %v", got, tt.want)
			}
		})
	}
}
