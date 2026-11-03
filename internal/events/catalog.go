// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package events

const (
	ActionCordon           Action = "Cordon"
	ActionCreate           Action = "Create"
	ActionDelete           Action = "Delete"
	ActionPatch            Action = "Patch"
	ActionResolveReference Action = "ResolveReference"
	ActionSync             Action = "Sync"
	ActionSyncFinalizer    Action = "SyncFinalizer"
	ActionUpdate           Action = "Update"
)

const (
	ReasonConflictingController  Reason = "ConflictingController"
	ReasonControllerRefFailed    Reason = "ControllerRefFailed"
	ReasonDefunctSlurmNodePruned Reason = "DefunctSlurmNodePruned"
	ReasonFailedCreate           Reason = "FailedCreate"
	ReasonFailedDaemonPod        Reason = "FailedDaemonPod"
	ReasonFailedDelete           Reason = "FailedDelete"
	ReasonFailedPatch            Reason = "FailedPatch"
	ReasonFailedUpdate           Reason = "FailedUpdate"
	ReasonHostnameMismatch       Reason = "HostnameMismatch"
	ReasonNodeCordon             Reason = "NodeCordon"
	ReasonSlurmNodeInvalidReg    Reason = "SlurmNodeInvalidReg"
	ReasonSlurmNodeNotRegistered Reason = "SlurmNodeNotRegistered"
	ReasonSucceededDaemonPod     Reason = "SucceededDaemonPod"
	ReasonSuccessfulCreate       Reason = "SuccessfulCreate"
	ReasonSuccessfulDelete       Reason = "SuccessfulDelete"
	ReasonSuccessfulUpdate       Reason = "SuccessfulUpdate"
	ReasonSyncFailed             Reason = "SyncFailed"
	ReasonSyncFinalizerFailed    Reason = "SyncFinalizerFailed"
)
