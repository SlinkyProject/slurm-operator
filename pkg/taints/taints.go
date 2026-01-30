// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package taints

import (
	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

const (
	TaintKeyWorkerNode = slinkyv1beta1.NodeSetPrefix + "worker"
)

var (
	TaintNodeWorker = corev1.Taint{
		Key:    TaintKeyWorkerNode,
		Effect: corev1.TaintEffectNoExecute,
	}
)

var (
	TolerationWorkerNode = corev1.Toleration{
		Key:      TaintNodeWorker.Key,
		Operator: corev1.TolerationOpEqual,
		Effect:   TaintNodeWorker.Effect,
	}
)
