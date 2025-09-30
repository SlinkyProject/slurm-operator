// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package podcontrol

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/ktesting"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	nodesetutils "github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/utils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/podcontrol"
)

func newPodControl(client client.Client, recorder record.EventRecorder) *realPodControl {
	return &realPodControl{
		Client:     client,
		recorder:   recorder,
		podControl: podcontrol.NewPodControl(client, recorder),
	}
}

func newNodeSet(replicas int32) *slinkyv1beta1.NodeSet {
	petMounts := []corev1.VolumeMount{
		{Name: "datadir", MountPath: "/tmp/zookeeper"},
	}
	podMounts := []corev1.VolumeMount{
		{Name: "home", MountPath: "/home"},
	}
	return newNodeSetWithVolumes(replicas, "foo", petMounts, podMounts)
}

func newNodeSetWithVolumes(replicas int32, name string, petMounts []corev1.VolumeMount, podMounts []corev1.VolumeMount) *slinkyv1beta1.NodeSet {
	mounts := petMounts
	mounts = append(mounts, podMounts...)
	claims := []corev1.PersistentVolumeClaim{}
	for _, m := range petMounts {
		claims = append(claims, newPVC(m.Name))
	}

	vols := []corev1.Volume{}
	for _, m := range podMounts {
		vols = append(vols, corev1.Volume{
			Name: m.Name,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: fmt.Sprintf("/tmp/%v", m.Name),
				},
			},
		})
	}

	template := slinkyv1beta1.PodTemplate{
		PodMetadata: slinkyv1beta1.Metadata{
			Labels: map[string]string{"foo": "bar"},
		},
		PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
			PodSpec: corev1.PodSpec{
				Volumes: vols,
			},
		},
	}

	return &slinkyv1beta1.NodeSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       slinkyv1beta1.NodeSetKind,
			APIVersion: slinkyv1beta1.NodeSetAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: corev1.NamespaceDefault,
			UID:       types.UID("test"),
		},
		Spec: slinkyv1beta1.NodeSetSpec{
			Replicas: ptr.To(replicas),
			Slurmd: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image:        "nginx",
					VolumeMounts: mounts,
				},
			},
			Template:             template,
			VolumeClaimTemplates: claims,
			UpdateStrategy: slinkyv1beta1.NodeSetUpdateStrategy{
				Type: slinkyv1beta1.RollingUpdateNodeSetStrategyType,
			},
			PersistentVolumeClaimRetentionPolicy: &slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
			RevisionHistoryLimit: ptr.To[int32](2),
		},
	}
}

func newPVC(name string) corev1.PersistentVolumeClaim {
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *resource.NewQuantity(1, resource.BinarySI),
				},
			},
		},
	}
}

func Test_realPodControl_CreateNodeSetPod(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	nodeset := newNodeSet(2)
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
	type fields struct {
		Client   client.Client
		recorder record.EventRecorder
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Invalid pod",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset,
				pod:     &corev1.Pod{},
			},
			wantErr: true,
		},
		{
			name: "Valid pod",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset,
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "Duplicate pod",
			fields: fields{
				Client:   fake.NewFakeClient(pod.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset,
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewPodControl(tt.fields.Client, tt.fields.recorder)
			if err := r.CreateNodeSetPod(tt.args.ctx, tt.args.nodeset, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("realPodControl.CreateNodeSetPod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_realPodControl_DeleteNodeSetPod(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	nodeset := newNodeSet(2)
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
	type fields struct {
		Client   client.Client
		recorder record.EventRecorder
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Non-existent pod",
			fields: fields{
				Client:   fake.NewFakeClient(),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset,
				pod:     &corev1.Pod{},
			},
			wantErr: true,
		},
		{
			name: "Existing pod",
			fields: fields{
				Client:   fake.NewFakeClient(pod.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset,
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewPodControl(tt.fields.Client, tt.fields.recorder)
			if err := r.DeleteNodeSetPod(tt.args.ctx, tt.args.nodeset, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("realPodControl.DeleteNodeSetPod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_realPodControl_UpdateNodeSetPod(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	nodeset := newNodeSet(2)
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
	pvc := ptr.To(newPVC("datadir-foo-0"))
	type fields struct {
		Client   client.Client
		recorder record.EventRecorder
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Consistent pod",
			fields: fields{
				Client:   fake.NewFakeClient(pod.DeepCopy(), pvc.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset,
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "Inconsistent identity",
			fields: fields{
				Client:   fake.NewFakeClient(pod.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset,
				pod: func() *corev1.Pod {
					toUpdate := pod.DeepCopy()
					toUpdate.Namespace = "default-2"
					return toUpdate
				}(),
			},
			wantErr: false,
		},
		{
			name: "Inconsistent storage",
			fields: fields{
				Client:   fake.NewFakeClient(pod.DeepCopy(), pvc.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset,
				pod: func() *corev1.Pod {
					toUpdate := pod.DeepCopy()
					toUpdate.Spec.Volumes = nil
					return toUpdate
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewPodControl(tt.fields.Client, tt.fields.recorder)
			if err := r.UpdateNodeSetPod(tt.args.ctx, tt.args.nodeset, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("realPodControl.UpdateNodeSetPod() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_realPodControl_PodPVCsMatchRetentionPolicy(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	nodeset := newNodeSet(2)
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
	pvc := newPVC("datadir-foo-0")
	type fields struct {
		Client   client.Client
		recorder record.EventRecorder
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "PVC NotFound",
			fields: fields{
				Client:   fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "PVC Found",
			fields: fields{
				Client:   fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy(), pvc.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "Get Error",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							return http.ErrAbortHandler
						},
					}).
					WithRuntimeObjects(nodeset.DeepCopy(), pod.DeepCopy(), pvc.DeepCopy()).
					Build(),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewPodControl(tt.fields.Client, tt.fields.recorder)
			got, err := r.PodPVCsMatchRetentionPolicy(tt.args.ctx, tt.args.nodeset, tt.args.pod)
			if (err != nil) != tt.wantErr {
				t.Errorf("realPodControl.PodPVCsMatchRetentionPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("realPodControl.PodPVCsMatchRetentionPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_realPodControl_UpdatePodPVCsForRetentionPolicy(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	nodeset := newNodeSet(1)
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
	type fields struct {
		Client   client.Client
		recorder record.EventRecorder
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "NotFound",
			fields: fields{
				Client:   fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "Error",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							return http.ErrAbortHandler
						},
					}).
					WithRuntimeObjects(nodeset.DeepCopy(), pod.DeepCopy()).
					Build(),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewPodControl(tt.fields.Client, tt.fields.recorder)
			if err := r.UpdatePodPVCsForRetentionPolicy(tt.args.ctx, tt.args.nodeset, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("realPodControl.UpdatePodPVCsForRetentionPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_realPodControl_IsPodPVCsStale(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	const missing = "missing"
	const exists = "exists"
	const stale = "stale"
	const withRef = "with-ref"
	testCases := []struct {
		name        string
		claimStates []string
		expected    bool
		skipPodUID  bool
	}{
		{
			name:        "all missing",
			claimStates: []string{missing, missing},
			expected:    false,
		},
		{
			name:        "no claims",
			claimStates: []string{},
			expected:    false,
		},
		{
			name:        "exists",
			claimStates: []string{missing, exists},
			expected:    false,
		},
		{
			name:        "all refs",
			claimStates: []string{withRef, withRef},
			expected:    false,
		},
		{
			name:        "stale & exists",
			claimStates: []string{stale, exists},
			expected:    true,
		},
		{
			name:        "stale & missing",
			claimStates: []string{stale, missing},
			expected:    true,
		},
		{
			name:        "withRef & stale",
			claimStates: []string{withRef, stale},
			expected:    true,
		},
		{
			name:        "withRef, no UID",
			claimStates: []string{withRef},
			skipPodUID:  true,
			expected:    true,
		},
	}
	for _, tc := range testCases {
		nodeset := slinkyv1beta1.NodeSet{}
		nodeset.Name = "set"
		nodeset.Namespace = corev1.NamespaceDefault
		nodeset.Spec.PersistentVolumeClaimRetentionPolicy = &slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
			WhenDeleted: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
		}
		pvcList := &corev1.PersistentVolumeClaimList{}
		for i, claimState := range tc.claimStates {
			claim := corev1.PersistentVolumeClaim{}
			claim.Name = fmt.Sprintf("claim-%d", i)
			nodeset.Spec.VolumeClaimTemplates = append(nodeset.Spec.VolumeClaimTemplates, claim)
			claim.Name = fmt.Sprintf("%s-set-3", claim.Name)
			claim.Namespace = nodeset.Namespace
			switch claimState {
			case missing:
			// Do nothing, the claim shouldn't exist.
			case exists:
				pvcList.Items = append(pvcList.Items, claim)
			case stale:
				claim.SetOwnerReferences([]metav1.OwnerReference{
					{
						Name:       "set-3",
						UID:        types.UID("stale"),
						APIVersion: "v1",
						Kind:       "Pod",
					},
				})
				pvcList.Items = append(pvcList.Items, claim)
			case withRef:
				claim.SetOwnerReferences([]metav1.OwnerReference{
					{
						Name:       "set-3",
						UID:        types.UID("123"),
						APIVersion: "v1",
						Kind:       "Pod",
					},
				})
				pvcList.Items = append(pvcList.Items, claim)
			}
		}
		pod := corev1.Pod{}
		pod.Name = "set-3"
		if !tc.skipPodUID {
			pod.SetUID("123")
		}
		c := fake.NewFakeClient(nodeset.DeepCopy(), pod.DeepCopy(), pvcList.DeepCopy())
		r := NewPodControl(c, record.NewFakeRecorder(10))
		expected := tc.expected
		// Note that the error isn't / can't be tested.
		if stale, _ := r.IsPodPVCsStale(context.TODO(), &nodeset, &pod); stale != expected {
			t.Errorf("unexpected stale for %s", tc.name)
		}
	}
}

func Test_realPodControl_createPersistentVolumeClaims(t *testing.T) {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	nodeset := newNodeSet(1)
	pod := nodesetutils.NewNodeSetPod(nodeset, controller, 0, "")
	pvc := newPVC("datadir-foo-0")
	type fields struct {
		Client   client.Client
		recorder record.EventRecorder
	}
	type args struct {
		ctx     context.Context
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Create",
			fields: fields{
				Client:   fake.NewFakeClient(nodeset.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "Already Exists",
			fields: fields{
				Client:   fake.NewFakeClient(nodeset.DeepCopy(), pvc.DeepCopy()),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: false,
		},
		{
			name: "Deletion",
			fields: fields{
				Client: func() client.Client {
					pvc := pvc.DeepCopy()
					pvc.DeletionTimestamp = ptr.To(metav1.Now())
					pvc.Finalizers = append(pvc.Finalizers, "foo")
					return fake.NewFakeClient(nodeset.DeepCopy(), pvc)
				}(),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "Get Error",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							return http.ErrServerClosed
						},
					}).
					WithRuntimeObjects(nodeset.DeepCopy()).
					Build(),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
		{
			name: "Create Error",
			fields: fields{
				Client: fake.NewClientBuilder().
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, client client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							return http.ErrServerClosed
						},
					}).
					WithRuntimeObjects(nodeset.DeepCopy()).
					Build(),
				recorder: record.NewFakeRecorder(10),
			},
			args: args{
				ctx:     context.TODO(),
				nodeset: nodeset.DeepCopy(),
				pod:     pod.DeepCopy(),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newPodControl(tt.fields.Client, tt.fields.recorder)
			if err := r.createPersistentVolumeClaims(tt.args.ctx, tt.args.nodeset, tt.args.pod); (err != nil) != tt.wantErr {
				t.Errorf("realPodControl.createPersistentVolumeClaims() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_isClaimOwnerUpToDate(t *testing.T) {
	testCases := []struct {
		name            string
		scaleDownPolicy slinkyv1beta1.PersistentVolumeClaimRetentionPolicyType
		setDeletePolicy slinkyv1beta1.PersistentVolumeClaimRetentionPolicyType
		needsPodRef     bool
		needsSetRef     bool
		podCordon       bool
	}{
		{
			name:            "retain",
			scaleDownPolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			needsPodRef:     false,
			needsSetRef:     false,
		},
		{
			name:            "on nodeset delete",
			scaleDownPolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			needsPodRef:     false,
			needsSetRef:     true,
		},
		{
			name:            "on scaledown only, condemned",
			scaleDownPolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			needsPodRef:     true,
			needsSetRef:     false,
			podCordon:       true,
		},
		{
			name:            "on scaledown only, remains",
			scaleDownPolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			needsPodRef:     false,
			needsSetRef:     false,
		},
		{
			name:            "on both, condemned",
			scaleDownPolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			needsPodRef:     true,
			needsSetRef:     false,
			podCordon:       true,
		},
		{
			name:            "on both, remains",
			scaleDownPolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			needsPodRef:     false,
			needsSetRef:     true,
		},
	}

	for _, tc := range testCases {
		for _, useOtherRefs := range []bool{false, true} {
			for _, setPodRef := range []bool{false, true} {
				for _, setSetRef := range []bool{false, true} {
					_, ctx := ktesting.NewTestContext(t)
					logger := klog.FromContext(ctx)
					claim := corev1.PersistentVolumeClaim{}
					claim.Name = "target-claim"
					pod := corev1.Pod{}
					pod.Name = "pod-0"
					pod.GetObjectMeta().SetUID("pod-123")
					if tc.podCordon {
						pod.Annotations = map[string]string{
							slinkyv1beta1.AnnotationPodCordon: "true",
						}
					}
					nodeset := slinkyv1beta1.NodeSet{}
					nodeset.Name = "nodeset"
					nodeset.GetObjectMeta().SetUID("ss-456")
					nodeset.Spec.PersistentVolumeClaimRetentionPolicy = &slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
						WhenScaled:  tc.scaleDownPolicy,
						WhenDeleted: tc.setDeletePolicy,
					}
					claimRefs := claim.GetOwnerReferences()
					if setPodRef {
						claimRefs = addControllerRef(claimRefs, &pod, podGVK)
					}
					if setSetRef {
						claimRefs = addControllerRef(claimRefs, &nodeset, slinkyv1beta1.NodeSetGVK)
					}
					if useOtherRefs {
						claimRefs = append(
							claimRefs,
							metav1.OwnerReference{
								Name:       "rand1",
								APIVersion: "v1",
								Kind:       "Pod",
								UID:        "rand1-uid",
							},
							metav1.OwnerReference{
								Name:       "rand2",
								APIVersion: "v1",
								Kind:       "Pod",
								UID:        "rand2-uid",
							})
					}
					claim.SetOwnerReferences(claimRefs)
					shouldMatch := setPodRef == tc.needsPodRef && setSetRef == tc.needsSetRef
					if isClaimOwnerUpToDate(logger, &claim, &nodeset, &pod) != shouldMatch {
						t.Errorf("Bad match for %s with pod=%v,nodeset=%v,others=%v", tc.name, setPodRef, setSetRef, useOtherRefs)
					}
				}
			}
		}
	}
}

func TestEdgeCases_isClaimOwnerUpToDate(t *testing.T) {
	_, ctx := ktesting.NewTestContext(t)
	logger := klog.FromContext(ctx)

	testCases := []struct {
		name        string
		ownerRefs   []metav1.OwnerReference
		policy      slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy
		shouldMatch bool
	}{
		{
			name: "normal controller, pod",
			ownerRefs: []metav1.OwnerReference{
				{
					Name:       "pod-1",
					APIVersion: "v1",
					Kind:       "Pod",
					UID:        "pod-123",
					Controller: ptr.To(true),
				},
			},
			policy: slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			shouldMatch: true,
		},
		{
			name: "non-controller causes policy mismatch, pod",
			ownerRefs: []metav1.OwnerReference{
				{
					Name:       "pod-1",
					APIVersion: "v1",
					Kind:       "Pod",
					UID:        "pod-123",
				},
			},
			policy: slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			shouldMatch: false,
		},
		{
			name: "stale controller does not affect policy, pod",
			ownerRefs: []metav1.OwnerReference{
				{
					Name:       "pod-1",
					APIVersion: "v1",
					Kind:       "Pod",
					UID:        "pod-stale",
					Controller: ptr.To(true),
				},
			},
			policy: slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			shouldMatch: true,
		},
		{
			name: "unexpected controller causes policy mismatch, pod",
			ownerRefs: []metav1.OwnerReference{
				{
					Name:       "pod-1",
					APIVersion: "v1",
					Kind:       "Pod",
					UID:        "pod-123",
					Controller: ptr.To(true),
				},
				{
					Name:       "Random",
					APIVersion: "v1",
					Kind:       "Pod",
					UID:        "random",
					Controller: ptr.To(true),
				},
			},
			policy: slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			shouldMatch: false,
		},
		{
			name: "normal controller, nodeset",
			ownerRefs: []metav1.OwnerReference{
				{
					Name:       "nodeset",
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					UID:        "ss-456",
					Controller: ptr.To(true),
				},
			},
			policy: slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			shouldMatch: true,
		},
		{
			name: "non-controller causes policy mismatch, nodeset",
			ownerRefs: []metav1.OwnerReference{
				{
					Name:       "nodeset",
					APIVersion: "foo/v0",
					Kind:       slinkyv1beta1.NodeSetKind,
					UID:        "ss-456",
				},
			},
			policy: slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			shouldMatch: false,
		},
		{
			name: "stale controller ignored, nodeset",
			ownerRefs: []metav1.OwnerReference{
				{
					Name:       "nodeset",
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					UID:        "set-stale",
					Controller: ptr.To(true),
				},
			},
			policy: slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			shouldMatch: true,
		},
		{
			name: "unexpected controller causes policy mismatch, nodeset",
			ownerRefs: []metav1.OwnerReference{
				{
					Name:       "nodeset",
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					UID:        "ss-456",
					Controller: ptr.To(true),
				},
				{
					Name:       "Random",
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					UID:        "random",
					Controller: ptr.To(true),
				},
			},
			policy: slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
				WhenDeleted: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		claim := corev1.PersistentVolumeClaim{}
		claim.Name = "target-claim"
		pod := corev1.Pod{}
		pod.Name = "pod-1"
		pod.GetObjectMeta().SetUID("pod-123")
		pod.Annotations = map[string]string{
			slinkyv1beta1.AnnotationPodCordon: "true",
		}
		nodeset := slinkyv1beta1.NodeSet{}
		nodeset.Name = "nodeset"
		nodeset.GetObjectMeta().SetUID("ss-456")
		nodeset.Spec.PersistentVolumeClaimRetentionPolicy = &tc.policy
		claim.SetOwnerReferences(tc.ownerRefs)
		got := isClaimOwnerUpToDate(logger, &claim, &nodeset, &pod)
		if got != tc.shouldMatch {
			t.Errorf("Unexpected match for %s, got %t expected %t", tc.name, got, tc.shouldMatch)
		}
	}
}

func Test_hasUnexpectedController(t *testing.T) {
	// Each test case will be tested against a NodeSet named "set" and a Pod named "pod" with UIDs "123".
	testCases := []struct {
		name                             string
		refs                             []metav1.OwnerReference
		shouldReportUnexpectedController bool
	}{
		{
			name: "custom controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "chipmunks/v1",
					Kind:       "CustomController",
					Name:       "simon",
					UID:        "other-uid",
					Controller: ptr.To(true),
				},
			},
			shouldReportUnexpectedController: true,
		},
		{
			name: "custom non-controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "chipmunks/v1",
					Kind:       "CustomController",
					Name:       "simon",
					UID:        "other-uid",
					Controller: ptr.To(false),
				},
			},
			shouldReportUnexpectedController: false,
		},
		{
			name: "custom unspecified controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "chipmunks/v1",
					Kind:       "CustomController",
					Name:       "simon",
					UID:        "other-uid",
				},
			},
			shouldReportUnexpectedController: false,
		},
		{
			name: "other pod controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "simon",
					UID:        "other-uid",
					Controller: ptr.To(true),
				},
			},
			shouldReportUnexpectedController: true,
		},
		{
			name: "other set controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "simon",
					UID:        "other-uid",
					Controller: ptr.To(true),
				},
			},
			shouldReportUnexpectedController: true,
		},
		{
			name: "own set controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "set",
					UID:        "set-uid",
					Controller: ptr.To(true),
				},
			},
			shouldReportUnexpectedController: false,
		},
		{
			name: "own set controller, stale uid",
			refs: []metav1.OwnerReference{
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "set",
					UID:        "stale-uid",
					Controller: ptr.To(true),
				},
			},
			shouldReportUnexpectedController: true,
		},
		{
			name: "own pod controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod",
					UID:        "pod-uid",
					Controller: ptr.To(true),
				},
			},
			shouldReportUnexpectedController: false,
		},
		{
			name: "own pod controller, stale uid",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod",
					UID:        "stale-uid",
					Controller: ptr.To(true),
				},
			},
			shouldReportUnexpectedController: true,
		},
		{
			// API validation should prevent two controllers from being set,
			// but for completeness it is still tested.
			name: "own controller and another",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod",
					UID:        "pod-uid",
					Controller: ptr.To(true),
				},
				{
					APIVersion: "chipmunks/v1",
					Kind:       "CustomController",
					Name:       "simon",
					UID:        "other-uid",
					Controller: ptr.To(true),
				},
			},
			shouldReportUnexpectedController: true,
		},
		{
			name: "own controller and a non-controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod",
					UID:        "pod-uid",
					Controller: ptr.To(true),
				},
				{
					APIVersion: "chipmunks/v1",
					Kind:       "CustomController",
					Name:       "simon",
					UID:        "other-uid",
					Controller: ptr.To(false),
				},
			},
			shouldReportUnexpectedController: false,
		},
	}
	for _, tc := range testCases {
		target := &corev1.PersistentVolumeClaim{}
		target.SetOwnerReferences(tc.refs)
		nodeset := &slinkyv1beta1.NodeSet{}
		nodeset.SetName("set")
		nodeset.SetUID("set-uid")
		pod := &corev1.Pod{}
		pod.SetName("pod")
		pod.SetUID("pod-uid")
		nodeset.Spec.PersistentVolumeClaimRetentionPolicy = nil
		if hasUnexpectedController(target, nodeset, pod) {
			t.Errorf("Any controller should be allowed when no retention policy (retain behavior) is specified. Incorrectly identified unexpected controller at %s", tc.name)
		}
		const retainPolicy = slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType
		const deletePolicy = slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType
		for _, policy := range []slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
			// {WhenDeleted: retainPolicy, WhenScaled: retainPolicy},
			{WhenDeleted: retainPolicy, WhenScaled: deletePolicy},
			{WhenDeleted: deletePolicy, WhenScaled: retainPolicy},
			{WhenDeleted: deletePolicy, WhenScaled: deletePolicy},
		} {
			nodeset.Spec.PersistentVolumeClaimRetentionPolicy = &policy
			got := hasUnexpectedController(target, nodeset, pod)
			if got != tc.shouldReportUnexpectedController {
				t.Errorf("Unexpected controller mismatch at %s (policy %v)", tc.name, policy)
			}
		}
	}
}

func Test_hasNonControllerOwner(t *testing.T) {
	testCases := []struct {
		name string
		refs []metav1.OwnerReference
		// The set and pod objects will be created with names "set" and "pod", respectively.
		setUID        types.UID
		podUID        types.UID
		nonController bool
	}{
		{
			// API validation should prevent two controllers from being set,
			// but for completeness the semantics here are tested.
			name: "set and pod controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod",
					UID:        "pod",
					Controller: ptr.To(true),
				},
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "set",
					UID:        "set",
					Controller: ptr.To(true),
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: false,
		},
		{
			name: "set controller, pod noncontroller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod",
					UID:        "pod",
				},
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "set",
					UID:        "set",
					Controller: ptr.To(true),
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: true,
		},
		{
			name: "set noncontroller, pod controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod",
					UID:        "pod",
					Controller: ptr.To(true),
				},
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "set",
					UID:        "set",
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: true,
		},
		{
			name: "set controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "set",
					UID:        "set",
					Controller: ptr.To(true),
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: false,
		},
		{
			name: "pod controller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod",
					UID:        "pod",
					Controller: ptr.To(true),
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: false,
		},
		{
			name:          "nothing",
			refs:          []metav1.OwnerReference{},
			setUID:        "set",
			podUID:        "pod",
			nonController: false,
		},
		{
			name: "set noncontroller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "set",
					UID:        "set",
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: true,
		},
		{
			name: "set noncontroller with ptr",
			refs: []metav1.OwnerReference{
				{
					APIVersion: slinkyv1beta1.NodeSetAPIVersion,
					Kind:       slinkyv1beta1.NodeSetKind,
					Name:       "set",
					UID:        "set",
					Controller: ptr.To(false),
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: true,
		},
		{
			name: "pod noncontroller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "pod",
					Name:       "pod",
					UID:        "pod",
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: true,
		},
		{
			name: "other noncontroller",
			refs: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "pod",
					Name:       "pod",
					UID:        "not-matching",
				},
			},
			setUID:        "set",
			podUID:        "pod",
			nonController: false,
		},
	}

	for _, tc := range testCases {
		claim := corev1.PersistentVolumeClaim{}
		claim.SetOwnerReferences(tc.refs)
		pod := corev1.Pod{}
		pod.SetUID(tc.podUID)
		pod.SetName("pod")
		nodeset := slinkyv1beta1.NodeSet{}
		nodeset.SetUID(tc.setUID)
		nodeset.SetName("set")
		got := hasNonControllerOwner(&claim, &nodeset, &pod)
		if got != tc.nonController {
			t.Errorf("Failed %s: got %t, expected %t", tc.name, got, tc.nonController)
		}
	}
}

func Test_updateClaimOwnerRefForSetAndPod(t *testing.T) {
	testCases := []struct {
		name                 string
		scaleDownPolicy      slinkyv1beta1.PersistentVolumeClaimRetentionPolicyType
		setDeletePolicy      slinkyv1beta1.PersistentVolumeClaimRetentionPolicyType
		condemned            bool
		needsPodRef          bool
		needsSetRef          bool
		unexpectedController bool
	}{
		{
			name:            "retain",
			scaleDownPolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			condemned:       false,
			needsPodRef:     false,
			needsSetRef:     false,
		},
		{
			name:            "delete with nodeset",
			scaleDownPolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			condemned:       false,
			needsPodRef:     false,
			needsSetRef:     true,
		},
		{
			name:            "delete with scaledown, not condemned",
			scaleDownPolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			condemned:       false,
			needsPodRef:     false,
			needsSetRef:     false,
		},
		{
			name:            "delete on scaledown, condemned",
			scaleDownPolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
			condemned:       true,
			needsPodRef:     true,
			needsSetRef:     false,
		},
		{
			name:            "delete on both, not condemned",
			scaleDownPolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			condemned:       false,
			needsPodRef:     false,
			needsSetRef:     true,
		},
		{
			name:            "delete on both, condemned",
			scaleDownPolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy: slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			condemned:       true,
			needsPodRef:     true,
			needsSetRef:     false,
		},
		{
			name:                 "unexpected controller",
			scaleDownPolicy:      slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			setDeletePolicy:      slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
			condemned:            true,
			needsPodRef:          false,
			needsSetRef:          false,
			unexpectedController: true,
		},
	}
	for _, tc := range testCases {
		for variations := 0; variations < 8; variations++ {
			hasPodRef := (variations & 1) != 0
			hasSetRef := (variations & 2) != 0
			extraOwner := (variations & 3) != 0
			_, ctx := ktesting.NewTestContext(t)
			logger := klog.FromContext(ctx)
			nodeset := slinkyv1beta1.NodeSet{}
			nodeset.Name = "nss"
			numReplicas := int32(5)
			nodeset.Spec.Replicas = &numReplicas
			nodeset.SetUID("nss-123")
			nodeset.Spec.PersistentVolumeClaimRetentionPolicy = &slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenScaled:  tc.scaleDownPolicy,
				WhenDeleted: tc.setDeletePolicy,
			}
			pod := corev1.Pod{}
			if tc.condemned {
				pod.Name = "pod-8"
				pod.Annotations = map[string]string{
					slinkyv1beta1.AnnotationPodCordon: "true",
				}
			} else {
				pod.Name = "pod-1"
			}
			pod.SetUID("pod-456")
			claim := corev1.PersistentVolumeClaim{}
			claimRefs := claim.GetOwnerReferences()
			if hasPodRef {
				claimRefs = addControllerRef(claimRefs, &pod, podGVK)
			}
			if hasSetRef {
				claimRefs = addControllerRef(claimRefs, &nodeset, slinkyv1beta1.NodeSetGVK)
			}
			if extraOwner {
				// Note the extra owner should not affect our owner references.
				claimRefs = append(claimRefs, metav1.OwnerReference{
					APIVersion: "custom/v1",
					Kind:       "random",
					Name:       "random",
					UID:        "abc",
				})
			}
			if tc.unexpectedController {
				claimRefs = append(claimRefs, metav1.OwnerReference{
					APIVersion: "custom/v1",
					Kind:       "Unknown",
					Name:       "unknown",
					UID:        "xyz",
					Controller: ptr.To(true),
				})
			}
			claim.SetOwnerReferences(claimRefs)
			updateClaimOwnerRefForSetAndPod(logger, &claim, &nodeset, &pod)
			// Confirm that after the update, the specified owner is set as the only controller.
			// Any other controllers will be cleaned update by the update.
			check := func(target, owner metav1.Object) bool {
				for _, ref := range target.GetOwnerReferences() {
					if ref.UID == owner.GetUID() {
						return ref.Controller != nil && *ref.Controller
					}
				}
				return false
			}
			if check(&claim, &pod) != tc.needsPodRef {
				t.Errorf("Bad pod ref for %s hasPodRef=%v hasSetRef=%v", tc.name, hasPodRef, hasSetRef)
			}
			if check(&claim, &nodeset) != tc.needsSetRef {
				t.Errorf("Bad nodeset ref for %s hasPodRef=%v hasSetRef=%v", tc.name, hasPodRef, hasSetRef)
			}
		}
	}
}

func Test_hasOwnerRef(t *testing.T) {
	target := corev1.Pod{}
	target.SetOwnerReferences([]metav1.OwnerReference{
		{UID: "123", Controller: ptr.To(true)},
		{UID: "456", Controller: ptr.To(false)},
		{UID: "789"},
	})
	testCases := []struct {
		uid    types.UID
		hasRef bool
	}{
		{
			uid:    "123",
			hasRef: true,
		},
		{
			uid:    "456",
			hasRef: true,
		},
		{
			uid:    "789",
			hasRef: true,
		},
		{
			uid:    "012",
			hasRef: false,
		},
	}
	for _, tc := range testCases {
		owner := corev1.Pod{}
		owner.GetObjectMeta().SetUID(tc.uid)
		got := hasOwnerRef(&target, &owner)
		if got != tc.hasRef {
			t.Errorf("Expected %t for %s, got %t", tc.hasRef, tc.uid, got)
		}
	}
}

func Test_getPersistentVolumeClaimRetentionPolicy(t *testing.T) {
	retainPolicy := slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
		WhenScaled:  slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
		WhenDeleted: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
	}
	scaledownPolicy := slinkyv1beta1.NodeSetPersistentVolumeClaimRetentionPolicy{
		WhenScaled:  slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType,
		WhenDeleted: slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType,
	}

	nodeset := slinkyv1beta1.NodeSet{}
	nodeset.Spec.PersistentVolumeClaimRetentionPolicy = &retainPolicy
	got := getPersistentVolumeClaimRetentionPolicy(&nodeset)
	if got.WhenScaled != slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType || got.WhenDeleted != slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType {
		t.Errorf("Expected retain policy")
	}
	nodeset.Spec.PersistentVolumeClaimRetentionPolicy = &scaledownPolicy
	got = getPersistentVolumeClaimRetentionPolicy(&nodeset)
	if got.WhenScaled != slinkyv1beta1.DeletePersistentVolumeClaimRetentionPolicyType || got.WhenDeleted != slinkyv1beta1.RetainPersistentVolumeClaimRetentionPolicyType {
		t.Errorf("Expected scaledown policy")
	}
}

func Test_hasStaleOwnerRef(t *testing.T) {
	target := corev1.PersistentVolumeClaim{}
	target.SetOwnerReferences([]metav1.OwnerReference{
		{Name: "bob", UID: "123", APIVersion: "v1", Kind: "Pod"},
		{Name: "shirley", UID: "456", APIVersion: "v1", Kind: "Pod"},
	})
	ownerA := corev1.Pod{}
	ownerA.SetUID("123")
	ownerA.Name = "bob"
	ownerB := corev1.Pod{}
	ownerB.Name = "shirley"
	ownerB.SetUID("789")
	ownerC := corev1.Pod{}
	ownerC.Name = "yvonne"
	ownerC.SetUID("345")
	if hasStaleOwnerRef(&target, &ownerA, podGVK) {
		t.Error("ownerA should not be stale")
	}
	if !hasStaleOwnerRef(&target, &ownerB, podGVK) {
		t.Error("ownerB should be stale")
	}
	if hasStaleOwnerRef(&target, &ownerC, podGVK) {
		t.Error("ownerC should not be stale")
	}
}

func Test_matchesRef(t *testing.T) {
	testCases := []struct {
		name        string
		ref         metav1.OwnerReference
		obj         metav1.ObjectMeta
		schema      schema.GroupVersionKind
		shouldMatch bool
	}{
		{
			name: "full match",
			ref: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "fred",
				UID:        "abc",
			},
			obj: metav1.ObjectMeta{
				Name: "fred",
				UID:  "abc",
			},
			schema:      podGVK,
			shouldMatch: true,
		},
		{
			name: "match without UID",
			ref: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "fred",
				UID:        "abc",
			},
			obj: metav1.ObjectMeta{
				Name: "fred",
				UID:  "not-matching",
			},
			schema:      podGVK,
			shouldMatch: true,
		},
		{
			name: "mismatch name",
			ref: metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "fred",
				UID:        "abc",
			},
			obj: metav1.ObjectMeta{
				Name: "joan",
				UID:  "abc",
			},
			schema:      podGVK,
			shouldMatch: false,
		},
		{
			name: "wrong schema",
			ref: metav1.OwnerReference{
				APIVersion: "beta2",
				Kind:       "Pod",
				Name:       "fred",
				UID:        "abc",
			},
			obj: metav1.ObjectMeta{
				Name: "fred",
				UID:  "abc",
			},
			schema:      podGVK,
			shouldMatch: false,
		},
	}
	for _, tc := range testCases {
		got := matchesRef(&tc.ref, &tc.obj, tc.schema)
		if got != tc.shouldMatch {
			t.Errorf("Failed %s: got %t, expected %t", tc.name, got, tc.shouldMatch)
		}
	}
}

func Test_addControllerRef(t *testing.T) {
	nodeset := newNodeSet(1)
	type args struct {
		refs  []metav1.OwnerReference
		owner metav1.Object
		gvk   schema.GroupVersionKind
	}
	tests := []struct {
		name string
		args args
		want []metav1.OwnerReference
	}{
		{
			name: "Empty refs",
			args: args{
				refs:  []metav1.OwnerReference{},
				owner: nodeset,
				gvk:   slinkyv1beta1.NodeSetGVK,
			},
			want: []metav1.OwnerReference{
				*metav1.NewControllerRef(nodeset, slinkyv1beta1.NodeSetGVK),
			},
		},
		{
			name: "Already owned",
			args: args{
				refs: []metav1.OwnerReference{
					*metav1.NewControllerRef(nodeset, slinkyv1beta1.NodeSetGVK),
				},
				owner: nodeset,
				gvk:   slinkyv1beta1.NodeSetGVK,
			},
			want: []metav1.OwnerReference{
				*metav1.NewControllerRef(nodeset, slinkyv1beta1.NodeSetGVK),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addControllerRef(tt.args.refs, tt.args.owner, tt.args.gvk); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addControllerRef() = %v, want %v", got, tt.want)
			}
		})
	}
}
