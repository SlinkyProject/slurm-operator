// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package k8sutils

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// SetNodeUnschedulable cordons or uncordons the Kubernetes Node, retrying
// failed reads and updates for up to 30 seconds.
func SetNodeUnschedulable(ctx context.Context, t *testing.T, crClient crclient.Client, nodeName string, unschedulable bool) {
	t.Helper()

	require.Eventually(t, func() bool {
		node := &corev1.Node{}
		if err := crClient.Get(ctx, crclient.ObjectKey{Name: nodeName}, node); err != nil {
			t.Logf("failed to get Node %s: %v", nodeName, err)
			return false
		}
		if node.Spec.Unschedulable == unschedulable {
			return true
		}
		node.Spec.Unschedulable = unschedulable
		if err := crClient.Update(ctx, node); err != nil {
			t.Logf("failed to update Node %s unschedulable=%t: %v", nodeName, unschedulable, err)
			return false
		}
		return true
	}, 30*time.Second, 500*time.Millisecond, "timed out setting Node %s unschedulable=%t", nodeName, unschedulable)
}
