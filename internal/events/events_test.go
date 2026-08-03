// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

func TestCatalog(t *testing.T) {
	actions := []Action{
		ActionCordon, ActionCreate, ActionDelete, ActionPatch,
		ActionResolveReference, ActionSync, ActionSyncFinalizer, ActionUpdate,
	}
	reasons := []Reason{
		ReasonConflictingController, ReasonControllerRefFailed, ReasonDefunctSlurmNodePruned,
		ReasonFailedCreate, ReasonFailedDaemonPod, ReasonFailedDelete, ReasonFailedPatch, ReasonFailedUpdate,
		ReasonHostnameMismatch, ReasonNodeCordon, ReasonSlurmNodeInvalidReg, ReasonSlurmNodeNotRegistered,
		ReasonSucceededDaemonPod, ReasonSuccessfulCreate, ReasonSuccessfulDelete, ReasonSuccessfulUpdate,
		ReasonSyncFailed, ReasonSyncFinalizerFailed,
	}
	validName := regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	for _, value := range actions {
		require.LessOrEqual(t, len(value), 128)
		require.True(t, validName.MatchString(string(value)), "invalid action %q", value)
	}
	for _, value := range reasons {
		require.LessOrEqual(t, len(value), 128)
		require.True(t, validName.MatchString(string(value)), "invalid reason %q", value)
	}
}

func TestNoteIsTruncatedAtUTF8Boundary(t *testing.T) {
	recorder := testutils.NewEventRecorder(1)
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

	Normal(recorder, obj, nil, ReasonSuccessfulCreate, ActionCreate, "%s", strings.Repeat("a", MaxNoteLength-1)+"界")

	event := testutils.ReadOneRecordedEvent(t, recorder)
	require.LessOrEqual(t, len(event.Note), MaxNoteLength)
	require.True(t, utf8.ValidString(event.Note))
	require.Equal(t, strings.Repeat("a", MaxNoteLength-1), event.Note)
}

func TestLiteralNoteIsNotFormatted(t *testing.T) {
	recorder := testutils.NewEventRecorder(1)
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

	Warning(recorder, obj, nil, ReasonSyncFailed, ActionSync, "progress is 50%")

	event := testutils.ReadOneRecordedEvent(t, recorder)
	require.Equal(t, "progress is 50%", event.Note)
}
