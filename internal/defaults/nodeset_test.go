// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func TestEffectiveSlurmNodeNameMode(t *testing.T) {
	for _, test := range []struct {
		name                  string
		prefer                *bool
		scaling               slinkyv1beta1.ScalingModeType
		pinned, oversubscribe bool
		want                  slinkyv1beta1.SlurmNodeNameModeType
	}{
		{name: "preferred pinned", prefer: ptr.To(true), pinned: true, want: slinkyv1beta1.SlurmNodeNameModeKubernetesNode},
		{name: "default pinned uses node naming", pinned: true, want: slinkyv1beta1.SlurmNodeNameModeKubernetesNode},
		{name: "explicit false preserves pod naming", prefer: ptr.To(false), pinned: true, want: slinkyv1beta1.SlurmNodeNameModePodHostname},
		{name: "default unpinned uses pod naming", want: slinkyv1beta1.SlurmNodeNameModePodHostname},
		{name: "preferred unpinned falls back", prefer: ptr.To(true), want: slinkyv1beta1.SlurmNodeNameModePodHostname},
		{name: "default oversubscribed falls back", pinned: true, oversubscribe: true, want: slinkyv1beta1.SlurmNodeNameModePodHostname},
		{name: "preferred oversubscribed falls back", prefer: ptr.To(true), pinned: true, oversubscribe: true, want: slinkyv1beta1.SlurmNodeNameModePodHostname},
		{name: "daemon default unchanged", scaling: slinkyv1beta1.ScalingModeDaemonset, want: slinkyv1beta1.SlurmNodeNameModeKubernetesNode},
		{name: "daemon preferred unchanged", prefer: ptr.To(true), scaling: slinkyv1beta1.ScalingModeDaemonset, want: slinkyv1beta1.SlurmNodeNameModeKubernetesNode},
		{name: "daemon explicit false unchanged", prefer: ptr.To(false), scaling: slinkyv1beta1.ScalingModeDaemonset, want: slinkyv1beta1.SlurmNodeNameModeKubernetesNode},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := slinkyv1beta1.NodeSetSpec{ScalingMode: test.scaling, PreferKubernetesNodeName: test.prefer, PinToNode: test.pinned, OversubscribeNode: test.oversubscribe}
			require.Equal(t, test.want, spec.EffectiveSlurmNodeNameMode())
		})
	}
}

func TestPreferKubernetesNodeNameDefaulting(t *testing.T) {
	for _, test := range []struct {
		name, input string
		want        bool
	}{
		{name: "omitted", input: `{"spec":{}}`, want: true},
		{name: "null", input: `{"spec":{"preferKubernetesNodeName":null}}`, want: true},
		{name: "true", input: `{"spec":{"preferKubernetesNodeName":true}}`, want: true},
		{name: "false", input: `{"spec":{"preferKubernetesNodeName":false}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, json.Unmarshal([]byte(test.input), nodeset))
			for range 2 {
				SetNodeSetDefaults(nodeset)
				require.Equal(t, ptr.To(test.want), nodeset.Spec.PreferKubernetesNodeName)
			}
			data, err := json.Marshal(nodeset)
			require.NoError(t, err)
			roundTripped := &slinkyv1beta1.NodeSet{}
			require.NoError(t, json.Unmarshal(data, roundTripped))
			require.Equal(t, ptr.To(test.want), roundTripped.Spec.PreferKubernetesNodeName)
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
		require.Equal(t, ptr.To(DefaultNodeSetPreferKubernetesNodeName), ns.Spec.PreferKubernetesNodeName)
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
		ns.Spec.PreferKubernetesNodeName = ptr.To(false)
		ns.Spec.UpdateStrategy.Type = slinkyv1beta1.OnDeleteNodeSetStrategyType
		maxUnavailable := intstr.FromString("57%")
		ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(maxUnavailable)
		ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		ns.Spec.PruneSlurmNodeRecords = slinkyv1beta1.NodeSetPruneNodeRecordTypeNodeNotFound
		SetNodeSetDefaults(ns)

		require.Equal(t, ptr.To(int32(3)), ns.Spec.Replicas)
		require.Equal(t, slinkyv1beta1.ScalingModeDaemonset, ns.Spec.ScalingMode)
		require.Equal(t, ptr.To(false), ns.Spec.PreferKubernetesNodeName)
		require.Equal(t, ptr.To(maxUnavailable), ns.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable)
		require.Equal(t, slinkyv1beta1.OnDeleteNodeSetStrategyType, ns.Spec.UpdateStrategy.Type)
		require.Equal(t, slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenDeleted)
		require.Equal(t, slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType, ns.Spec.PersistentVolumeClaimRetentionPolicy.WhenScaled)
		require.Equal(t, slinkyv1beta1.NodeSetPruneNodeRecordTypeNodeNotFound, ns.Spec.PruneSlurmNodeRecords)
	})
}
