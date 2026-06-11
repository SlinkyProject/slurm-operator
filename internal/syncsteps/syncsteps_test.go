// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package syncsteps

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.NoError(t, Sync(ctx, rec, obj, steps))
	assertNoEvents(t, rec)
}

func TestSync_EmptySteps_ReturnsNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rec := events.NewFakeRecorder(10)
	obj := &corev1.ConfigMap{}
	require.NoError(t, Sync(ctx, rec, obj, nil))
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
	require.ErrorIs(t, err, wantErr)
	require.ErrorContains(t, err, `failed "bad" step`)
	ev := readOneEvent(t, rec)
	require.Contains(t, ev, "Warning")
	require.Contains(t, ev, failedReason)
	require.Contains(t, ev, `Failed "bad" step`)
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
	require.ErrorAs(t, err, &agg)
	require.Len(t, agg.Errors(), 3)
	require.True(t, thirdRan, "expected third step to Sync when StopOnError is false")
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
	require.Error(t, err)
	require.False(t, after, "step after StopOnError failure should not Sync")
	var agg utilerrors.Aggregate
	require.ErrorAs(t, err, &agg)
	require.Len(t, agg.Errors(), 1)
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
	require.ErrorContains(t, err, `failed "x" step`)
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
	require.Error(t, err)
	readOneEvent(t, rec)
	assertNoEvents(t, rec)
}
