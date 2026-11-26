// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package k8sutils

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// NodeSetPods returns the pods that the NodeSet controls.
func NodeSetPods(ctx context.Context, crClient crclient.Client, nodeset *slinkyv1beta1.NodeSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := crClient.List(ctx, podList, crclient.InNamespace(nodeset.Namespace)); err != nil {
		return nil, err
	}

	pods := make([]corev1.Pod, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		if metav1.IsControlledBy(pod, nodeset) {
			pods = append(pods, *pod)
		}
	}
	return pods, nil
}

// MutateNodeSet applies mutate to a fresh read of the NodeSet and updates it,
// retrying on update conflicts. Other API errors fail the test immediately.
func MutateNodeSet(ctx context.Context, t *testing.T, crClient crclient.Client, nodesetKey crclient.ObjectKey, mutate func(*slinkyv1beta1.NodeSet)) {
	t.Helper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nodeset := &slinkyv1beta1.NodeSet{}
		if err := crClient.Get(ctx, nodesetKey, nodeset); err != nil {
			return err
		}
		mutate(nodeset)
		return crClient.Update(ctx, nodeset)
	})
	require.NoError(t, err, "failed to update NodeSet %s/%s", nodesetKey.Namespace, nodesetKey.Name)
}

// PodDeadline returns the time the NodeSet controller expects the pod's
// running Slurm jobs to end, from the pod's deadline annotation.
func PodDeadline(ctx context.Context, crClient crclient.Client, podKey crclient.ObjectKey) (time.Time, error) {
	pod := &corev1.Pod{}
	if err := crClient.Get(ctx, podKey, pod); err != nil {
		return time.Time{}, fmt.Errorf("get pod %s: %w", podKey, err)
	}
	value, ok := pod.Annotations[slinkyv1beta1.AnnotationPodDeadline]
	if !ok {
		return time.Time{}, fmt.Errorf("pod %s has no %s annotation", podKey, slinkyv1beta1.AnnotationPodDeadline)
	}
	return time.Parse(time.RFC3339, value)
}
