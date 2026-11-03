// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package events

import (
	"fmt"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientevents "k8s.io/client-go/tools/events"
)

// MaxNoteLength is the maximum Event note size accepted by the Kubernetes API.
const MaxNoteLength = 1024

type Action string
type Reason string

func Normal(recorder clientevents.EventRecorder, regarding, related runtime.Object, reason Reason, action Action, note string, args ...any) {
	record(recorder, regarding, related, corev1.EventTypeNormal, reason, action, note, args...)
}

func Warning(recorder clientevents.EventRecorder, regarding, related runtime.Object, reason Reason, action Action, note string, args ...any) {
	record(recorder, regarding, related, corev1.EventTypeWarning, reason, action, note, args...)
}

func record(recorder clientevents.EventRecorder, regarding, related runtime.Object, eventType string, reason Reason, action Action, note string, args ...any) {
	if recorder == nil {
		return
	}

	if len(args) > 0 {
		note = fmt.Sprintf(note, args...)
	}
	if len(note) > MaxNoteLength {
		note = note[:MaxNoteLength]
		for !utf8.ValidString(note) {
			note = note[:len(note)-1]
		}
	}
	recorder.Eventf(regarding, related, eventType, string(reason), string(action), "%s", note)
}
