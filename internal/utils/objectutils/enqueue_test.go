// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package objectutils

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newQueue() workqueue.TypedRateLimitingInterface[reconcile.Request] {
	return workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
}

func TestEnqueueRequestAfter(t *testing.T) {
	type args struct {
		q        workqueue.TypedRateLimitingInterface[reconcile.Request]
		obj      client.Object
		duration time.Duration
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "immediate",
			args: args{
				q: newQueue(),
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
				duration: 0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			EnqueueRequestAfter(tt.args.q, tt.args.obj, tt.args.duration)
			if tt.args.q.Len() == 0 {
				t.Errorf("Len() = %d", tt.args.q.Len())
			}
		})
	}
}

func TestEnqueueRequest(t *testing.T) {
	type args struct {
		q   workqueue.TypedRateLimitingInterface[reconcile.Request]
		obj client.Object
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "immediate",
			args: args{
				q: newQueue(),
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			EnqueueRequest(tt.args.q, tt.args.obj)
			if tt.args.q.Len() == 0 {
				t.Errorf("Len() = %d", tt.args.q.Len())
			}
		})
	}
}
