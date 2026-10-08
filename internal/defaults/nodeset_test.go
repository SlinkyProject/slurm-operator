// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"testing"

	"github.com/stretchr/testify/require"
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

		require.Equal(t, new(DefaultNodeSetReplicas), ns.Spec.Replicas)
		require.Equal(t, DefaultNodeSetScalingMode, ns.Spec.ScaleStrategy.Mode)
		require.Equal(t, ptr.To(DefaultNodeSetSyncTopology), ns.Spec.SyncTopology)
		require.Equal(t, ptr.To(DefaultNodeSetWorkloadDisruptionProtection), ns.Spec.WorkloadDisruptionProtection)
		require.Equal(t, DefaultNodeSetUpdateStrategyType, ns.Spec.UpdateStrategy.Type)
		require.NotNil(t, ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable)
		require.Equal(t, slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted)
		require.Equal(t, slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled)
		require.Equal(t, DefaultNodeSetPruneSlurmNodeRecordType, ns.Spec.PruneSlurmNodeRecords)
	})

	t.Run("explicit values are not overridden", func(t *testing.T) {
		ns := &slinkyv1beta1.NodeSet{}
		ns.Spec.Replicas = ptr.To(int32(3))
		ns.Spec.ScaleStrategy.Mode = slinkyv1beta1.ScalingModeDaemonset
		ns.Spec.UpdateStrategy.Type = slinkyv1beta1.OnDeleteNodeSetStrategyType
		maxUnavailable := intstr.FromString("57%")
		ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(maxUnavailable)
		ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		ns.Spec.PruneSlurmNodeRecords = slinkyv1beta1.NodeSetPruneNodeRecordTypeNodeNotFound //nolint:staticcheck // SA1019
		ns.Spec.SyncTopology = ptr.To(false)
		SetNodeSetDefaults(ns)

		require.Equal(t, ptr.To(int32(3)), ns.Spec.Replicas)
		require.Equal(t, slinkyv1beta1.ScalingModeDaemonset, ns.Spec.ScaleStrategy.Mode)
		require.Equal(t, ptr.To(false), ns.Spec.SyncTopology)
		require.Equal(t, ptr.To(maxUnavailable), ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable)
		require.Equal(t, slinkyv1beta1.OnDeleteNodeSetStrategyType, ns.Spec.UpdateStrategy.Type)
		require.Equal(t, slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted)
		require.Equal(t, slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled)
		require.Equal(t, slinkyv1beta1.NodeSetPruneNodeRecordTypeNodeNotFound, ns.Spec.PruneSlurmNodeRecords) //nolint:staticcheck // SA1019
	})

	t.Run("new fields take precedence over deprecated fields", func(t *testing.T) {
		ns := &slinkyv1beta1.NodeSet{}
		ns.Spec.ScalingMode = ptr.To(slinkyv1beta1.ScalingModeDaemonset) //nolint:staticcheck // SA1019
		ns.Spec.ScaleStrategy.Mode = slinkyv1beta1.ScalingModeStatefulset
		ns.Spec.OrdinalPadding = ptr.To(uint(2)) //nolint:staticcheck // SA1019
		ns.Spec.ScaleStrategy.StatefulSet.OrdinalPadding = ptr.To(uint(0))
		ns.Spec.PinToNode = ptr.To(true) //nolint:staticcheck // SA1019
		ns.Spec.ScaleStrategy.StatefulSet.PinToNode = ptr.To(false)
		SetNodeSetDefaults(ns)

		require.Equal(t, slinkyv1beta1.ScalingModeStatefulset, ns.Spec.ScaleStrategy.Mode)
		require.Equal(t, ptr.To(uint(0)), ns.Spec.ScaleStrategy.StatefulSet.OrdinalPadding)
		require.Equal(t, ptr.To(false), ns.Spec.ScaleStrategy.StatefulSet.PinToNode)
	})

	t.Run("deprecated fields are honored when new fields are unset", func(t *testing.T) {
		ns := &slinkyv1beta1.NodeSet{}
		ns.Spec.ScalingMode = ptr.To(slinkyv1beta1.ScalingModeDaemonset) //nolint:staticcheck // SA1019
		ns.Spec.OrdinalPadding = new(uint(3))                            //nolint:staticcheck // SA1019
		ns.Spec.PinToNode = new(false)                                   //nolint:staticcheck // SA1019
		SetNodeSetDefaults(ns)

		require.Equal(t, slinkyv1beta1.ScalingModeDaemonset, ns.Spec.ScaleStrategy.Mode)
		require.Equal(t, ns.Spec.ScaleStrategy.StatefulSet.OrdinalPadding, ns.Spec.OrdinalPadding) //nolint:staticcheck // SA1019
		require.Equal(t, ns.Spec.ScaleStrategy.StatefulSet.PinToNode, ns.Spec.PinToNode)           //nolint:staticcheck // SA1019
	})
}
