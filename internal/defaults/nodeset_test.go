// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestSetNodeSetDefaults(t *testing.T) {
	t.Run("nil nodeset is a no-op", func(t *testing.T) {
		SetNodeSetDefaults(nil)
	})

	t.Run("zero value spec gets defaults", func(t *testing.T) {
		ns := &slinkyv1beta1.NodeSet{}
		SetNodeSetDefaults(ns)
		if ns.Spec.Replicas == nil || *ns.Spec.Replicas != DefaultNodeSetReplicas {
			t.Errorf("Replicas: want default %d, got %v", DefaultNodeSetReplicas, ns.Spec.Replicas)
		}
		if ns.Spec.ScalingMode != DefaultNodeSetScalingMode {
			t.Errorf("ScalingMode: want %q, got %q", DefaultNodeSetScalingMode, ns.Spec.ScalingMode)
		}
		if ns.Spec.WorkloadDisruptionProtection == nil || *ns.Spec.WorkloadDisruptionProtection != DefaultNodeSetWorkloadDisruptionProtection {
			t.Errorf("WorkloadDisruptionProtection: want %v, got %v", DefaultNodeSetWorkloadDisruptionProtection, ns.Spec.WorkloadDisruptionProtection)
		}
		if ns.Spec.UpdateStrategy.Type != DefaultNodeSetUpdateStrategyType {
			t.Errorf("UpdateStrategy.Type: want %q, got %q", DefaultNodeSetUpdateStrategyType, ns.Spec.UpdateStrategy.Type)
		}
		if ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable == nil {
			t.Error("UpdateStrategy.RollingUpdate.MaxUnavailable: want default, got nil")
		}
		if ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted != slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType {
			t.Errorf("PersistentVolumeClaimRetentionPolicy.WhenDeleted: want %q, got %q", slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted)
		}
		if ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled != slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType {
			t.Errorf("PersistentVolumeClaimRetentionPolicy.WhenScaled: want %q, got %q", slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled)
		}
		if ns.Spec.PruneSlurmNodeRecords != DefaultNodeSetPruneSlurmNodeRecordType {
			t.Errorf("PruneSlurmNodeRecords: want %q, got %q", DefaultNodeSetPruneSlurmNodeRecordType, ns.Spec.PruneSlurmNodeRecords)
		}
	})

	t.Run("explicit values are not overridden", func(t *testing.T) {
		ns := &slinkyv1beta1.NodeSet{}
		ns.Spec.Replicas = ptr.To(int32(3))
		ns.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset
		ns.Spec.UpdateStrategy.Type = slinkyv1beta1.OnDeleteNodeSetStrategyType
		maxUnavailable := intstr.FromString("57%")
		ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(maxUnavailable)
		ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		SetNodeSetDefaults(ns)
		if ptr.Deref(ns.Spec.Replicas, 0) != 3 {
			t.Errorf("Replicas: want 3, got %v", ptr.Deref(ns.Spec.Replicas, 0))
		}
		if ns.Spec.ScalingMode != slinkyv1beta1.ScalingModeDaemonset {
			t.Errorf("ScalingMode: want DaemonSet, got %q", ns.Spec.ScalingMode)
		}
		if !equality.Semantic.DeepEqual(ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable, ptr.To(maxUnavailable)) {
			t.Errorf("RollingUpdate.MaxUnavailable: want %q, got %q", maxUnavailable.String(), ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable)
		}
		if ns.Spec.UpdateStrategy.Type != slinkyv1beta1.OnDeleteNodeSetStrategyType {
			t.Errorf("UpdateStrategy.Type: want OnDelete, got %q", ns.Spec.UpdateStrategy.Type)
		}
		if ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted != slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType {
			t.Errorf("PersistentVolumeClaimRetentionPolicy.WhenDeleted: want %q, got %q", slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted)
		}
		if ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled != slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType {
			t.Errorf("PersistentVolumeClaimRetentionPolicy.WhenScaled: want %q, got %q", slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled)
		}
	})
}
