// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	sinterceptor "github.com/SlinkyProject/slurm-client/pkg/client/interceptor"
	slurmobject "github.com/SlinkyProject/slurm-client/pkg/object"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
)

func TestNodeSetReconciler_registrationChangeRefreshesNodeRecordAfterPodStops(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{ObjectMeta: metav1.ObjectMeta{
		Namespace: corev1.NamespaceDefault,
		Name:      "slurm",
	}}

	oldNodeSet := newNodeSet("foo", controller.Name, 1)
	oldNodeSet.Spec.Slurmd.Resources.Limits = corev1.ResourceList{
		corev1.ResourceMemory: resource.MustParse("1Gi"),
	}
	oldPod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), oldNodeSet, controller, 0, "old-revision")

	nodeset := oldNodeSet.DeepCopy()
	nodeset.Spec.Slurmd.Resources.Limits[corev1.ResourceMemory] = resource.MustParse("2Gi")
	kubeClient := fake.NewFakeClient(controller, nodeset)

	deleteCalls := 0
	deletedNode := ""
	slurmClient := newFakeClientList(sinterceptor.Funcs{
		Delete: func(_ context.Context, obj slurmobject.Object, _ ...slurmclient.DeleteOption) error {
			deleteCalls++
			deletedNode = string(obj.GetKey())
			return nil
		},
	})
	r := newNodeSetController(kubeClient, newClientMap(controller.Name, slurmClient))

	if err := r.markPendingNodeRecordRefreshes(ctx, nodeset, []*corev1.Pod{oldPod}, "new-revision"); err != nil {
		t.Fatalf("markPendingNodeRecordRefreshes() error = %v", err)
	}
	nodeName := nodesetutils.GetSlurmNodeName(oldPod)
	if !pendingNodeRecordRefreshes(nodeset).Has(nodeName) {
		t.Fatalf("pending refreshes = %v, want %q", pendingNodeRecordRefreshes(nodeset), nodeName)
	}

	// The old pod still exists, so its slurmd can re-register and the record
	// must not be deleted yet.
	if err := r.syncPendingNodeRecordRefreshes(ctx, nodeset, []*corev1.Pod{oldPod}); err != nil {
		t.Fatalf("syncPendingNodeRecordRefreshes() with live pod error = %v", err)
	}
	if deleteCalls != 0 {
		t.Fatalf("DeleteNode() calls with live pod = %d, want 0", deleteCalls)
	}

	// Once the pod is terminating and terminal, slurmd has stopped. Delete the
	// stale record and clear the persisted marker before a replacement starts.
	now := metav1.Now()
	oldPod.DeletionTimestamp = &now
	oldPod.Status.Phase = corev1.PodFailed
	if err := r.syncPendingNodeRecordRefreshes(ctx, nodeset, []*corev1.Pod{oldPod}); err != nil {
		t.Fatalf("syncPendingNodeRecordRefreshes() after pod stopped error = %v", err)
	}
	if deleteCalls != 1 || deletedNode != nodeName {
		t.Fatalf("DeleteNode() = (%d, %q), want (1, %q)", deleteCalls, deletedNode, nodeName)
	}
	if pendingNodeRecordRefreshes(nodeset).Len() != 0 {
		t.Fatalf("pending refreshes = %v, want empty", pendingNodeRecordRefreshes(nodeset))
	}
}

func TestNodeSetReconciler_imageChangeDoesNotRefreshNodeRecord(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{ObjectMeta: metav1.ObjectMeta{
		Namespace: corev1.NamespaceDefault,
		Name:      "slurm",
	}}
	oldNodeSet := newNodeSet("foo", controller.Name, 1)
	oldPod := nodesetutils.NewNodeSetStatefulSetPod(fake.NewFakeClient(), oldNodeSet, controller, 0, "old-revision")

	nodeset := oldNodeSet.DeepCopy()
	nodeset.Spec.Slurmd.Image = "different-image"
	kubeClient := fake.NewFakeClient(controller, nodeset)
	r := newNodeSetController(kubeClient, newClientMap(controller.Name, newFakeClientList(sinterceptor.Funcs{})))

	if err := r.markPendingNodeRecordRefreshes(ctx, nodeset, []*corev1.Pod{oldPod}, "new-revision"); err != nil {
		t.Fatalf("markPendingNodeRecordRefreshes() error = %v", err)
	}
	if pendingNodeRecordRefreshes(nodeset).Len() != 0 {
		t.Fatalf("pending refreshes = %v, want empty", pendingNodeRecordRefreshes(nodeset))
	}
}

func TestNodeSetReconciler_nodeRecordDeleteFailureKeepsRefreshPending(t *testing.T) {
	ctx := context.Background()
	controller := &slinkyv1beta1.Controller{ObjectMeta: metav1.ObjectMeta{
		Namespace: corev1.NamespaceDefault,
		Name:      "slurm",
	}}
	nodeset := newNodeSet("foo", controller.Name, 1)
	setPendingNodeRecordRefreshes(nodeset, set.New("foo-0"))
	kubeClient := fake.NewFakeClient(nodeset)
	slurmClient := newFakeClientList(sinterceptor.Funcs{
		Delete: func(_ context.Context, _ slurmobject.Object, _ ...slurmclient.DeleteOption) error {
			return errors.New("node is part of a reservation")
		},
	})
	r := newNodeSetController(kubeClient, newClientMap(controller.Name, slurmClient))

	if err := r.syncPendingNodeRecordRefreshes(ctx, nodeset, nil); err == nil {
		t.Fatal("syncPendingNodeRecordRefreshes() error = nil, want deletion error")
	}
	if !pendingNodeRecordRefreshes(nodeset).Has("foo-0") {
		t.Fatalf("pending refreshes = %v, want foo-0 retained", pendingNodeRecordRefreshes(nodeset))
	}
}
