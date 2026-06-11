// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestSetTokenDefaults(t *testing.T) {
	t.Run("nil token is a no-op", func(t *testing.T) {
		SetTokenDefaults(nil)
	})

	t.Run("zero value spec gets defaults", func(t *testing.T) {
		tok := &slinkyv1beta1.Token{}
		SetTokenDefaults(tok)

		require.Equal(t, ptr.To(DefaultTokenRefresh), tok.Spec.Refresh)
	})

	t.Run("explicit values are not overridden", func(t *testing.T) {
		tok := &slinkyv1beta1.Token{}
		tok.Spec.Refresh = ptr.To(true)
		SetTokenDefaults(tok)
		require.Equal(t, ptr.To(true), tok.Spec.Refresh)

		tok.Spec.Refresh = ptr.To(false)
		SetTokenDefaults(tok)
		// Explicit false is not changed (we only set when nil).
		require.Equal(t, ptr.To(false), tok.Spec.Refresh)
	})
}
