// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
)

type RecordedEvent struct {
	Regarding runtime.Object
	Related   runtime.Object
	Type      string
	Reason    string
	Action    string
	Note      string
}

type EventRecorder struct {
	Events chan RecordedEvent
}

func NewEventRecorder(bufferSize int) *EventRecorder {
	return &EventRecorder{Events: make(chan RecordedEvent, bufferSize)}
}

func (r *EventRecorder) Eventf(regarding, related runtime.Object, eventType, reason, action, note string, args ...any) {
	event := RecordedEvent{
		Regarding: regarding.DeepCopyObject(),
		Type:      eventType,
		Reason:    reason,
		Action:    action,
		Note:      fmt.Sprintf(note, args...),
	}
	if related != nil {
		event.Related = related.DeepCopyObject()
	}
	r.Events <- event
}

func ReadOneRecordedEvent(t *testing.T, recorder *EventRecorder) RecordedEvent {
	t.Helper()
	select {
	case event := <-recorder.Events:
		return event
	default:
		t.Fatal("expected one event on channel")
		return RecordedEvent{}
	}
}

// ReadOneEvent returns the next event buffered on rec, failing t if there is none.
func ReadOneEvent(t *testing.T, rec *events.FakeRecorder) string {
	t.Helper()
	select {
	case ev := <-rec.Events:
		return ev
	default:
		t.Fatal("expected one event on channel")
		return ""
	}
}

// AssertNoEvents fails t if rec has any buffered events.
func AssertNoEvents(t *testing.T, rec *events.FakeRecorder) {
	t.Helper()
	select {
	case ev := <-rec.Events:
		t.Fatalf("unexpected event: %q", ev)
	default:
	}
}
