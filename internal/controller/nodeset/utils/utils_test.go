// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
)

func newNodeSet(name string) *slinkyv1beta1.NodeSet {
	petMounts := []corev1.VolumeMount{
		{Name: "datadir", MountPath: "/tmp/zookeeper"},
	}
	podMounts := []corev1.VolumeMount{
		{Name: "home", MountPath: "/home"},
	}
	return newNodeSetWithVolumes(name, petMounts, podMounts)
}

func newNodeSetWithVolumes(name string, petMounts []corev1.VolumeMount, podMounts []corev1.VolumeMount) *slinkyv1beta1.NodeSet {
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
		Metadata: slinkyv1beta1.Metadata{
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
			Replicas: ptr.To[int32](1),
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

func TestIsPodFromNodeSet(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	type args struct {
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "From NodeSet",
			args: args{
				nodeset: newNodeSet("foo"),
				pod:     NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, ""),
			},
			want: true,
		},
		{
			name: "Not From NodeSet",
			args: args{
				nodeset: newNodeSet("foo"),
				pod:     NewNodeSetPod(fake.NewFakeClient(), newNodeSet("bar"), controller, 1, ""),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPodFromNodeSet(tt.args.nodeset, tt.args.pod); got != tt.want {
				t.Errorf("IsPodFromNodeSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsPodFromNodeSet(b *testing.B) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	nodeset := newNodeSet("foo")
	pod := NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, "")

	for b.Loop() {
		IsPodFromNodeSet(nodeset, pod)
	}
}

func TestGetParentName(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "foo-0",
			args: args{
				pod: NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, ""),
			},
			want: "foo",
		},
		{
			name: "bar-1",
			args: args{
				pod: NewNodeSetPod(fake.NewFakeClient(), newNodeSet("bar"), controller, 1, ""),
			},
			want: "bar",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetParentName(tt.args.pod); got != tt.want {
				t.Errorf("GetParentName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGetParentName(b *testing.B) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	pod := NewNodeSetPod(fake.NewFakeClient(), newNodeSet("bar"), controller, 1, "")

	for b.Loop() {
		GetParentName(pod)
	}
}

func TestGetOrdinal(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "foo-0",
			args: args{
				pod: NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, ""),
			},
			want: 0,
		},
		{
			name: "bar-1",
			args: args{
				pod: NewNodeSetPod(fake.NewFakeClient(), newNodeSet("bar"), controller, 1, ""),
			},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOrdinal(tt.args.pod); got != tt.want {
				t.Errorf("GetOrdinal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGetOrdinal(b *testing.B) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	pod := NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, "")

	for b.Loop() {
		GetOrdinal(pod)
	}
}

func TestGetParentNameAndOrdinal(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 int
	}{
		{
			name: "foo-0",
			args: args{
				pod: NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, ""),
			},
			want:  "foo",
			want1: 0,
		},
		{
			name: "bar-1",
			args: args{
				pod: NewNodeSetPod(fake.NewFakeClient(), newNodeSet("bar"), controller, 1, ""),
			},
			want:  "bar",
			want1: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := GetParentNameAndOrdinal(tt.args.pod)
			if got != tt.want {
				t.Errorf("GetParentNameAndOrdinal() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetParentNameAndOrdinal() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func BenchmarkGetParentNameAndOrdinal(b *testing.B) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	pod := NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, "")

	for b.Loop() {
		GetParentNameAndOrdinal(pod)
	}
}

func TestGetPodName(t *testing.T) {
	type args struct {
		nodeset *slinkyv1beta1.NodeSet
		ordinal int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "foo-0",
			args: args{
				nodeset: newNodeSet("foo"),
				ordinal: 0,
			},
			want: "foo-0",
		},
		{
			name: "bar-1",
			args: args{
				nodeset: newNodeSet("bar"),
				ordinal: 1,
			},
			want: "bar-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPodName(tt.args.nodeset, tt.args.ordinal); got != tt.want {
				t.Errorf("GetPodName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGetPodName(b *testing.B) {
	nodeset := newNodeSet("foo")
	ordinal := 0

	for b.Loop() {
		GetPodName(nodeset, ordinal)
	}
}

func TestGetNodeName(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "foo-0",
			args: args{
				pod: NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, ""),
			},
			want: "foo-0",
		},
		{
			name: "bar-1",
			args: args{
				pod: NewNodeSetPod(fake.NewFakeClient(), newNodeSet("bar"), controller, 1, ""),
			},
			want: "bar-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetNodeName(tt.args.pod); got != tt.want {
				t.Errorf("GetNodeName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGetNodeName(b *testing.B) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	pod := NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, "")

	for b.Loop() {
		GetNodeName(pod)
	}
}

func TestIsIdentityMatch(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	type args struct {
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Match",
			args: args{
				nodeset: newNodeSet("foo"),
				pod:     NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, ""),
			},
			want: true,
		},
		{
			name: "Not Match",
			args: args{
				nodeset: newNodeSet("foo"),
				pod:     NewNodeSetPod(fake.NewFakeClient(), newNodeSet("bar"), controller, 1, ""),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsIdentityMatch(tt.args.nodeset, tt.args.pod); got != tt.want {
				t.Errorf("IsIdentityMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsIdentityMatch(b *testing.B) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	nodeset := newNodeSet("foo")
	pod := NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, "")

	for b.Loop() {
		IsIdentityMatch(nodeset, pod)
	}
}

func TestIsStorageMatch(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	type args struct {
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Match",
			args: args{
				nodeset: newNodeSet("foo"),
				pod:     NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, ""),
			},
			want: true,
		},
		{
			name: "Not Match",
			args: args{
				nodeset: newNodeSet("foo"),
				pod:     NewNodeSetPod(fake.NewFakeClient(), newNodeSet("bar"), controller, 1, ""),
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsStorageMatch(tt.args.nodeset, tt.args.pod); got != tt.want {
				t.Errorf("IsStorageMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkIsStorageMatch(b *testing.B) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	nodeset := newNodeSet("foo")
	pod := NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, "")

	for b.Loop() {
		IsStorageMatch(nodeset, pod)
	}
}

func TestGetPersistentVolumeClaims(t *testing.T) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}
	type args struct {
		nodeset *slinkyv1beta1.NodeSet
		pod     *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want map[string]corev1.PersistentVolumeClaim
	}{
		{
			name: "Without Claims",
			args: func() args {
				nodeset := &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "foo",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				}
				return args{
					nodeset: nodeset,
					pod:     NewNodeSetPod(fake.NewFakeClient(), nodeset, controller, 0, ""),
				}
			}(),
			want: map[string]corev1.PersistentVolumeClaim{},
		},
		{
			name: "With Claims",
			args: args{
				nodeset: newNodeSet("foo"),
				pod:     NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, ""),
			},
			want: map[string]corev1.PersistentVolumeClaim{
				"datadir": {
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "datadir-foo-0",
						Labels:    labels.NewBuilder().WithWorkerSelectorLabels(newNodeSet("foo")).Build(),
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: *resource.NewQuantity(1, resource.BinarySI),
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPersistentVolumeClaims(tt.args.nodeset, tt.args.pod); !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("GetPersistentVolumeClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGetPersistentVolumeClaims(b *testing.B) {
	controller := &slinkyv1beta1.Controller{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
	}

	nodesetWithoutClaims := &slinkyv1beta1.NodeSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "foo",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	}
	nodesetWithClaims := newNodeSet("foo")
	pod := NewNodeSetPod(fake.NewFakeClient(), newNodeSet("foo"), controller, 0, "")

	b.Run("Without claims", func(b *testing.B) {
		for b.Loop() {
			GetPersistentVolumeClaims(nodesetWithoutClaims, pod)
		}
	})

	b.Run("With claims", func(b *testing.B) {
		for b.Loop() {
			GetPersistentVolumeClaims(nodesetWithClaims, pod)
		}
	})

}

func TestGetPersistentVolumeClaimName(t *testing.T) {
	type args struct {
		nodeset       *slinkyv1beta1.NodeSet
		claim         *corev1.PersistentVolumeClaim
		paddedOrdinal string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Ordinal Zero",
			args: args{
				nodeset: newNodeSet("foo"),
				claim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "test",
					},
				},
				paddedOrdinal: "0",
			},
			want: "test-foo-0",
		},
		{
			name: "Non-Zero Ordinal",
			args: args{
				nodeset: newNodeSet("foo"),
				claim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: corev1.NamespaceDefault,
						Name:      "test",
					},
				},
				paddedOrdinal: "1",
			},
			want: "test-foo-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetPersistentVolumeClaimName(tt.args.nodeset, tt.args.claim, tt.args.paddedOrdinal); got != tt.want {
				t.Errorf("GetPersistentVolumeClaimName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkGetPersistentVolumeClaimName(b *testing.B) {
	nodeset := newNodeSet("foo")
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      "test",
		},
	}

	b.Run("Ordinal Zero", func(b *testing.B) {
		for b.Loop() {
			GetPersistentVolumeClaimName(nodeset, claim, "0")
		}
	})

	b.Run("Non-Zero Ordinal", func(b *testing.B) {
		for b.Loop() {
			GetPersistentVolumeClaimName(nodeset, claim, "1")
		}
	})
}
