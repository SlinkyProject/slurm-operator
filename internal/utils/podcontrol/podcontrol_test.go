// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2014 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package podcontrol

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/events"
	"k8s.io/kubernetes/pkg/securitycontext"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newReplicationController(replicas int) *corev1.ReplicationController {
	rc := &corev1.ReplicationController{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			UID:             uuid.NewUUID(),
			Name:            "foobar",
			Namespace:       metav1.NamespaceDefault,
			ResourceVersion: "18",
		},
		Spec: corev1.ReplicationControllerSpec{
			Replicas: ptr.To(int32(replicas)), //nolint:gosec // disable G115
			Selector: map[string]string{"foo": "bar"},
			Template: &corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "foo",
						"type": "production",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:                  "foo/bar",
							TerminationMessagePath: corev1.TerminationMessagePathDefault,
							ImagePullPolicy:        corev1.PullIfNotPresent,
							SecurityContext:        securitycontext.ValidSecurityContextWithContainerDefaults(),
						},
					},
					RestartPolicy: corev1.RestartPolicyAlways,
					DNSPolicy:     corev1.DNSDefault,
					NodeSelector: map[string]string{
						"baz": "blah",
					},
				},
			},
		},
	}
	return rc
}

func Test_realPodControl_CreatePods(t *testing.T) {
	ctx := context.Background()
	defaultNamespace := metav1.NamespaceDefault
	controllerSpec := newReplicationController(1)
	controllerRef := metav1.NewControllerRef(controllerSpec, corev1.SchemeGroupVersion.WithKind("Foo"))

	type fields struct {
		Client   client.Client
		recorder events.EventRecorder
	}
	type args struct {
		ctx           context.Context
		namespace     string
		template      *corev1.PodTemplateSpec
		object        runtime.Object
		controllerRef *metav1.OwnerReference
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Create pod",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx:           ctx,
				namespace:     defaultNamespace,
				template:      controllerSpec.Spec.Template,
				object:        controllerSpec,
				controllerRef: controllerRef,
			},
			wantErr: false,
		},
		{
			name: "Invalid pod template spec",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx:           ctx,
				namespace:     defaultNamespace,
				template:      &corev1.PodTemplateSpec{},
				object:        controllerSpec,
				controllerRef: controllerRef,
			},
			wantErr: true,
		},
		{
			name: "Invalid controller ref",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx:           ctx,
				namespace:     defaultNamespace,
				template:      controllerSpec.Spec.Template,
				object:        controllerSpec,
				controllerRef: &metav1.OwnerReference{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realPodControl{
				Client:   tt.fields.Client,
				recorder: tt.fields.recorder,
			}
			err := r.CreatePods(tt.args.ctx, tt.args.namespace, tt.args.template, tt.args.object, tt.args.controllerRef)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_realPodControl_CreatePodsWithGenerateName(t *testing.T) {
	ctx := context.Background()
	defaultNamespace := metav1.NamespaceDefault
	controllerSpec := newReplicationController(1)
	controllerRef := metav1.NewControllerRef(controllerSpec, corev1.SchemeGroupVersion.WithKind("Foo"))

	type fields struct {
		Client   client.Client
		recorder events.EventRecorder
	}
	type args struct {
		ctx           context.Context
		namespace     string
		template      *corev1.PodTemplateSpec
		object        runtime.Object
		controllerRef *metav1.OwnerReference
		generateName  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Create pod",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx:           ctx,
				namespace:     defaultNamespace,
				template:      controllerSpec.Spec.Template,
				object:        controllerSpec,
				controllerRef: controllerRef,
			},
			wantErr: false,
		},
		{
			name: "Invalid pod template spec",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx:           ctx,
				namespace:     defaultNamespace,
				template:      &corev1.PodTemplateSpec{},
				object:        controllerSpec,
				controllerRef: controllerRef,
			},
			wantErr: true,
		},
		{
			name: "Invalid controller ref",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx:           ctx,
				namespace:     defaultNamespace,
				template:      controllerSpec.Spec.Template,
				object:        controllerSpec,
				controllerRef: &metav1.OwnerReference{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realPodControl{
				Client:   tt.fields.Client,
				recorder: tt.fields.recorder,
			}
			err := r.CreatePodsWithGenerateName(tt.args.ctx, tt.args.namespace, tt.args.template, tt.args.object, tt.args.controllerRef, tt.args.generateName)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_realPodControl_CreateThisPod(t *testing.T) {
	ctx := context.Background()
	defaultNamespace := metav1.NamespaceDefault
	controllerSpec := newReplicationController(1)

	type fields struct {
		Client   client.Client
		recorder events.EventRecorder
	}
	type args struct {
		ctx    context.Context
		pod    *corev1.Pod
		object runtime.Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Create pod",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx: ctx,
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: defaultNamespace,
						Name:      "foo",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: corev1.PodSpec{},
				},
				object: controllerSpec,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realPodControl{
				Client:   tt.fields.Client,
				recorder: tt.fields.recorder,
			}
			err := r.CreateThisPod(tt.args.ctx, tt.args.pod, tt.args.object)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_realPodControl_createPods(t *testing.T) {
	ctx := context.Background()
	defaultNamespace := metav1.NamespaceDefault
	controllerSpec := newReplicationController(1)

	type fields struct {
		Client   client.Client
		recorder events.EventRecorder
	}
	type args struct {
		ctx    context.Context
		pod    *corev1.Pod
		object runtime.Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Create pod",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx: ctx,
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: defaultNamespace,
						Name:      "foo",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
				object: controllerSpec,
			},
			wantErr: false,
		},
		{
			name: "No Namespace/Name",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx: ctx,
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
				object: controllerSpec,
			},
			wantErr: true,
		},
		{
			name: "No Labels",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx: ctx,
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: defaultNamespace,
						Name:      "foo",
					},
				},
				object: controllerSpec,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := realPodControl{
				Client:   tt.fields.Client,
				recorder: tt.fields.recorder,
			}
			err := r.createPods(tt.args.ctx, tt.args.pod, tt.args.object)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_realPodControl_DeletePod(t *testing.T) {
	ctx := context.Background()
	defaultNamespace := metav1.NamespaceDefault
	controllerSpec := newReplicationController(1)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: defaultNamespace,
			Name:      "foo",
		},
	}

	type fields struct {
		Client   client.Client
		recorder events.EventRecorder
	}
	type args struct {
		ctx       context.Context
		namespace string
		podName   string
		object    runtime.Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Delete pod",
			fields: fields{
				Client:   fake.NewClientBuilder().WithObjects(pod).Build(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx:       ctx,
				namespace: defaultNamespace,
				podName:   pod.Name,
				object:    controllerSpec,
			},
			wantErr: false,
		},
		{
			name: "Pod Not Found",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(10),
			},
			args: args{
				ctx:       ctx,
				namespace: defaultNamespace,
				podName:   pod.Name,
				object:    controllerSpec,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realPodControl{
				Client:   tt.fields.Client,
				recorder: tt.fields.recorder,
			}
			err := r.DeletePod(tt.args.ctx, tt.args.namespace, tt.args.podName, tt.args.object)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_realPodControl_PatchPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "foo",
		},
	}
	type fields struct {
		Client   client.Client
		recorder events.EventRecorder
	}
	type args struct {
		ctx       context.Context
		namespace string
		name      string
		data      []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "strategic merge",
			fields: fields{
				Client:   fake.NewFakeClient(pod.DeepCopy()),
				recorder: events.NewFakeRecorder(5),
			},
			args: args{
				ctx:       context.TODO(),
				namespace: corev1.NamespaceDefault,
				name:      "foo",
				data: func() []byte {
					newPod := pod.DeepCopy()
					newPod.DeletionTimestamp = ptr.To(metav1.Now())
					patch := client.StrategicMergeFrom(newPod)
					data, err := patch.Data(pod)
					if err != nil {
						panic(err)
					}
					return data
				}(),
			},
			wantErr: false,
		},
		{
			name: "failed merge",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: events.NewFakeRecorder(5),
			},
			args: args{
				ctx:       context.TODO(),
				namespace: corev1.NamespaceDefault,
				name:      "foo",
				data: func() []byte {
					newPod := pod.DeepCopy()
					patch := client.StrategicMergeFrom(newPod)
					data, err := patch.Data(pod)
					if err != nil {
						panic(err)
					}
					return data
				}(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &realPodControl{
				Client:   tt.fields.Client,
				recorder: tt.fields.recorder,
			}
			err := r.PatchPod(tt.args.ctx, tt.args.namespace, tt.args.name, tt.args.data)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}
