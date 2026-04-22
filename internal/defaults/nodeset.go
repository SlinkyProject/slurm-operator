// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package defaults

import (
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

// Default values for NodeSet Spec fields when unspecified.
const (
	DefaultNodeSetReplicas                     int32                                         = 1
	DefaultNodeSetWorkloadDisruptionProtection bool                                          = true
	DefaultNodeSetScalingMode                  slinkyv1beta1.ScalingModeType                 = slinkyv1beta1.ScalingModeStatefulset
	DefaultNodeSetUpdateStrategyType           slinkyv1beta1.NodeSetUpdateStrategyType       = slinkyv1beta1.RollingUpdateNodeSetStrategyType
	DefaultNodeSetPruneSlurmNodeRecordType     slinkyv1beta1.NodeSetPruneSlurmNodeRecordType = slinkyv1beta1.NodeSetPruneNodeRecordTypeNever
)

// Default values for NodeSet Spec fields when unspecified.
var (
	DefaultNodeSetRollingUpdateMaxUnavailable intstr.IntOrString = intstr.FromString("25%")
)

func SetNodeSetDefaults(nodeset *slinkyv1beta1.NodeSet) {
	if nodeset == nil {
		return
	}
	s := &nodeset.Spec

	if s.Replicas == nil {
		s.Replicas = ptr.To(DefaultNodeSetReplicas)
	}

	if s.ScalingMode == "" {
		s.ScalingMode = slinkyv1beta1.ScalingModeStatefulset
	}

	if s.WorkloadDisruptionProtection == nil {
		s.WorkloadDisruptionProtection = ptr.To(DefaultNodeSetWorkloadDisruptionProtection)
	}

	if s.UpdateStrategy.Type == "" {
		s.UpdateStrategy.Type = slinkyv1beta1.RollingUpdateNodeSetStrategyType
	}

	if s.UpdateStrategy.Type == slinkyv1beta1.RollingUpdateNodeSetStrategyType {
		if s.UpdateStrategy.RollingUpdate.MaxUnavailable == nil {
			s.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(DefaultNodeSetRollingUpdateMaxUnavailable)
		}
	}

	if s.PersistentVolumeClaimRetentionPolicy.WhenDeleted == "" {
		s.PersistentVolumeClaimRetentionPolicy.WhenDeleted = slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType
	}

	if s.PersistentVolumeClaimRetentionPolicy.WhenScaled == "" {
		s.PersistentVolumeClaimRetentionPolicy.WhenScaled = slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType
	}

	if s.PruneSlurmNodeRecords == "" {
		s.PruneSlurmNodeRecords = DefaultNodeSetPruneSlurmNodeRecordType
	}
}
