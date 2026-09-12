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

func TestEffectiveSlurmNodeNameMode(t *testing.T) {
	for _, test := range []struct {
		name                  string
		prefer                bool
		scaling               slinkyv1beta1.ScalingModeType
		pinned, oversubscribe bool
		want                  slinkyv1beta1.SlurmNodeNameModeType
	}{
		{name: "preferred pinned", prefer: true, pinned: true, want: slinkyv1beta1.SlurmNodeNameModeKubernetesNode},
		{name: "default pinned preserves legacy", pinned: true, want: slinkyv1beta1.SlurmNodeNameModePodHostname},
		{name: "preferred unpinned falls back", prefer: true, want: slinkyv1beta1.SlurmNodeNameModePodHostname},
		{name: "preferred oversubscribed falls back", prefer: true, pinned: true, oversubscribe: true, want: slinkyv1beta1.SlurmNodeNameModePodHostname},
		{name: "daemon default unchanged", scaling: slinkyv1beta1.ScalingModeDaemonset, want: slinkyv1beta1.SlurmNodeNameModeKubernetesNode},
		{name: "daemon preferred unchanged", prefer: true, scaling: slinkyv1beta1.ScalingModeDaemonset, want: slinkyv1beta1.SlurmNodeNameModeKubernetesNode},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := slinkyv1beta1.NodeSetSpec{ScalingMode: test.scaling, PreferKubernetesNodeName: test.prefer, PinToNode: test.pinned, OversubscribeNode: test.oversubscribe}
			require.Equal(t, test.want, spec.EffectiveSlurmNodeNameMode())
		})
	}
}

func TestSetNodeSetDefaults(t *testing.T) {
	t.Run("nil nodeset is a no-op", func(t *testing.T) {
		SetNodeSetDefaults(nil)
	})

	t.Run("zero value spec gets defaults", func(t *testing.T) {
		ns := &slinkyv1beta1.NodeSet{}
		SetNodeSetDefaults(ns)

		require.Equal(t, ptr.To(DefaultNodeSetReplicas), ns.Spec.Replicas)
		require.Equal(t, DefaultNodeSetScalingMode, ns.Spec.ScalingMode)
		require.False(t, ns.Spec.PreferKubernetesNodeName)
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
		ns.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset
		ns.Spec.PreferKubernetesNodeName = true
		ns.Spec.UpdateStrategy.Type = slinkyv1beta1.OnDeleteNodeSetStrategyType
		maxUnavailable := intstr.FromString("57%")
		ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(maxUnavailable)
		ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		ns.Spec.PruneSlurmNodeRecords = slinkyv1beta1.NodeSetPruneNodeRecordTypeNodeNotFound
		SetNodeSetDefaults(ns)

		require.Equal(t, ptr.To(int32(3)), ns.Spec.Replicas)
		require.Equal(t, slinkyv1beta1.ScalingModeDaemonset, ns.Spec.ScalingMode)
		require.True(t, ns.Spec.PreferKubernetesNodeName)
		require.Equal(t, ptr.To(maxUnavailable), ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable)
		require.Equal(t, slinkyv1beta1.OnDeleteNodeSetStrategyType, ns.Spec.UpdateStrategy.Type)
		require.Equal(t, slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted)
		require.Equal(t, slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled)
		require.Equal(t, slinkyv1beta1.NodeSetPruneNodeRecordTypeNodeNotFound, ns.Spec.PruneSlurmNodeRecords)
	})
}
