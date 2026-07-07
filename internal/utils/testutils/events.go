// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"testing"

	"k8s.io/client-go/tools/events"
)

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
