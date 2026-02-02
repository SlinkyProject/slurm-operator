// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package config

import "testing"

func Test_configBuilder_Build(t *testing.T) {
	type fields struct {
		builder *configBuilder
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "empty",
			fields: fields{
				builder: NewBuilder(),
			},
			want: "",
		},
		{
			name: "with options",
			fields: fields{
				builder: NewBuilder().
					WithSeperator("=").
					WithFinalNewline(false).
					AddProperty(NewProperty("foo", "bar")).
					AddProperty(NewPropertyRaw("fizz ~ buzz")),
			},
			want: "foo=bar\nfizz ~ buzz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.fields.builder
			if got := b.Build(); got != tt.want {
				t.Errorf("configBuilder.Build() = %v, want %v", got, tt.want)
			}
		})
	}
}
