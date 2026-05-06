// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package syncsteps

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/events"
)

func readOneEvent(t *testing.T, rec *events.FakeRecorder) string {
	t.Helper()
	select {
	case ev := <-rec.Events:
		return ev
	default:
		t.Fatal("expected one event on channel")
		return ""
	}
}

func assertNoEvents(t *testing.T, rec *events.FakeRecorder) {
	t.Helper()
	select {
	case ev := <-rec.Events:
		t.Fatalf("unexpected event: %q", ev)
	default:
	}
}

func TestSync_AllStepsSucceed_ReturnsNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rec := events.NewFakeRecorder(10)
	obj := &corev1.ConfigMap{}
	steps := []Step[*corev1.ConfigMap]{
		{Name: "a", SyncFn: func(context.Context, *corev1.ConfigMap) error { return nil }},
		{Name: "b", SyncFn: func(context.Context, *corev1.ConfigMap) error { return nil }},
	}
	if err := Sync(ctx, rec, obj, steps); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	assertNoEvents(t, rec)
}

func TestSync_EmptySteps_ReturnsNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rec := events.NewFakeRecorder(10)
	obj := &corev1.ConfigMap{}
	if err := Sync(ctx, rec, obj, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	assertNoEvents(t, rec)
}

func TestSync_SingleFailure_OneEventAndWrappedError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rec := events.NewFakeRecorder(10)
	obj := &corev1.ConfigMap{}
	wantErr := errors.New("boom")
	steps := []Step[*corev1.ConfigMap]{
		{Name: "ok", SyncFn: func(context.Context, *corev1.ConfigMap) error { return nil }},
		{Name: "bad", SyncFn: func(context.Context, *corev1.ConfigMap) error { return wantErr }},
	}
	err := Sync(ctx, rec, obj, steps)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("errors.Is aggregate leaf: %v", err)
	}
	if !strings.Contains(err.Error(), `failed "bad" step`) {
		t.Fatalf("error text: %v", err)
	}
	ev := readOneEvent(t, rec)
	if !strings.Contains(ev, "Warning") || !strings.Contains(ev, failedReason) || !strings.Contains(ev, `Failed "bad" step`) {
		t.Fatalf("event: %q", ev)
	}
	assertNoEvents(t, rec)
}

func TestSync_TwoFailuresWithoutStop_BothRecordedAndContinues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rec := events.NewFakeRecorder(10)
	obj := &corev1.ConfigMap{}
	var thirdRan bool
	steps := []Step[*corev1.ConfigMap]{
		{Name: "e1", SyncFn: func(context.Context, *corev1.ConfigMap) error { return errors.New("one") }},
		{Name: "e2", SyncFn: func(context.Context, *corev1.ConfigMap) error { return errors.New("two") }},
		{Name: "e3", SyncFn: func(context.Context, *corev1.ConfigMap) error {
			thirdRan = true
			return errors.New("three")
		}},
	}
	err := Sync(ctx, rec, obj, steps)
	var agg utilerrors.Aggregate
	ok := errors.As(err, &agg)
	if !ok {
		t.Fatalf("want Aggregate, got %T", err)
	}
	if len(agg.Errors()) != 3 {
		t.Fatalf("want 3 errors, got %d: %v", len(agg.Errors()), agg.Errors())
	}
	if !thirdRan {
		t.Fatal("expected third step to Sync when StopOnError is false")
	}
	readOneEvent(t, rec)
	readOneEvent(t, rec)
	readOneEvent(t, rec)
	assertNoEvents(t, rec)
}

func TestSync_StopOnError_SkipsFollowingSteps(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rec := events.NewFakeRecorder(10)
	obj := &corev1.ConfigMap{}
	var after bool
	steps := []Step[*corev1.ConfigMap]{
		{Name: "halt", StopOnError: true, SyncFn: func(context.Context, *corev1.ConfigMap) error { return errors.New("stop") }},
		{Name: "after", SyncFn: func(context.Context, *corev1.ConfigMap) error {
			after = true
			return nil
		}},
	}
	err := Sync(ctx, rec, obj, steps)
	if err == nil {
		t.Fatal("expected error")
	}
	if after {
		t.Fatal("step after StopOnError failure should not Sync")
	}
	var agg utilerrors.Aggregate
	ok := errors.As(err, &agg)
	if !ok || len(agg.Errors()) != 1 {
		t.Fatalf("want single error in aggregate, got %v", err)
	}
	readOneEvent(t, rec)
	assertNoEvents(t, rec)
}

func TestSync_NilRecorder_NoPanicStillAggregates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	obj := &corev1.ConfigMap{}
	steps := []Step[*corev1.ConfigMap]{
		{Name: "x", SyncFn: func(context.Context, *corev1.ConfigMap) error { return errors.New("oops") }},
	}
	err := Sync(ctx, nil, obj, steps)
	if err == nil || !strings.Contains(err.Error(), `failed "x" step`) {
		t.Fatalf("Sync(nil recorder): %v", err)
	}
}

func TestSync_FirstSucceedsSecondFails_OneError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rec := events.NewFakeRecorder(10)
	obj := &corev1.ConfigMap{}
	steps := []Step[*corev1.ConfigMap]{
		{Name: "a", SyncFn: func(context.Context, *corev1.ConfigMap) error { return nil }},
		{Name: "b", SyncFn: func(context.Context, *corev1.ConfigMap) error { return errors.New("bad") }},
	}
	err := Sync(ctx, rec, obj, steps)
	if err == nil {
		t.Fatal("expected error")
	}
	readOneEvent(t, rec)
	assertNoEvents(t, rec)
}
