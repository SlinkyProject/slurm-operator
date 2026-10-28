// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseColumn(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   []string
	}{
		{name: "empty", output: "", want: nil},
		{name: "one value", output: "test\n", want: []string{"test"}},
		{name: "several values", output: "root\ntest\n", want: []string{"root", "test"}},
		{name: "extra fields", output: "test|slurm\n", want: []string{"test"}},
		{name: "blank lines and padding", output: "\n  test  \n\n", want: []string{"test"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, parseColumn(tt.output))
		})
	}
}
