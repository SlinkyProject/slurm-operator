// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-FileCopyrightText: Copyright 2016 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"reflect"
	"testing"

	slinkyv1alpha1 "github.com/SlinkyProject/slurm-operator/api/v1alpha1"
	"github.com/SlinkyProject/slurm-operator/internal/annotations"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
)

func Test_isPodFromNodeSet(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Names match",
			args: args{
				set: &slinkyv1alpha1.NodeSet{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
				pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "foo-"}},
			},
			want: true,
		},
		{
			name: "Names don't match",
			args: args{
				set: &slinkyv1alpha1.NodeSet{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
				pod: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bar-"}},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPodFromNodeSet(tt.args.set, tt.args.pod); got != tt.want {
				t.Errorf("isPodFromNodeSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_identityMatches(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Names match",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
				},
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-",
						Namespace: "default",
					},
				},
			},
			want: true,
		},
		{
			name: "Names don't match",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
				},
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-",
						Namespace: "system",
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := identityMatches(tt.args.set, tt.args.pod); got != tt.want {
				t.Errorf("identityMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_storageMatches(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Storage matches",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "nodeset"},
					Spec: slinkyv1alpha1.NodeSetSpec{
						VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "foo",
								},
							},
						},
					},
				},
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Hostname: "host",
						Volumes: []corev1.Volume{
							{
								Name: "foo",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "foo-nodeset-host",
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Storage doesn't match",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "nodeset"},
					Spec: slinkyv1alpha1.NodeSetSpec{
						VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "foo",
								},
							},
						},
					},
				},
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Hostname: "host",
						Volumes: []corev1.Volume{
							{
								Name: "foo",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "bar-nodeset-host",
									},
								},
							},
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := storageMatches(tt.args.set, tt.args.pod); got != tt.want {
				t.Errorf("storageMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPersistentVolumeClaimRetentionPolicy(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
	}
	tests := []struct {
		name string
		args args
		want slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy
	}{
		{
			name: "Return PersistentVolumeClaimRetentionPolicy",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						PersistentVolumeClaimRetentionPolicy: &slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
							WhenDeleted: slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType,
						},
					},
				},
			},
			want: slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPersistentVolumeClaimRetentionPolicy(tt.args.set); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPersistentVolumeClaimRetentionPolicy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_claimOwnerMatchesSetAndPod(t *testing.T) {
	type args struct {
		claim *corev1.PersistentVolumeClaim
		set   *slinkyv1alpha1.NodeSet
		pod   *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Claim ownership matches (unknown)",
			args: args{
				claim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("111")},
						},
					},
				},
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						PersistentVolumeClaimRetentionPolicy: &slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
							WhenDeleted: "",
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("111"),
					},
				},
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("222")},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Claim ownership doesn't match (delete)",
			args: args{
				claim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("111")},
						},
					},
				},
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						PersistentVolumeClaimRetentionPolicy: &slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
							WhenDeleted: slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType,
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("111"),
					},
				},
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("111")},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Claim ownership doesn't match (retain)",
			args: args{
				claim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("111")},
						},
					},
				},
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						PersistentVolumeClaimRetentionPolicy: &slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
							WhenDeleted: slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType,
						},
					},
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("222"),
					},
				},
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("111")},
						},
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := claimOwnerMatchesSetAndPod(klog.Logger{}, tt.args.claim, tt.args.set, tt.args.pod); got != tt.want {
				t.Errorf("claimOwnerMatchesSetAndPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateClaimOwnerRefForSetAndPod(t *testing.T) {
	type args struct {
		claim *corev1.PersistentVolumeClaim
		set   *slinkyv1alpha1.NodeSet
		pod   *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Delete PVC Retention Policy",
			args: args{
				claim: &corev1.PersistentVolumeClaim{},
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						PersistentVolumeClaimRetentionPolicy: &slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
							WhenDeleted: slinkyv1alpha1.DeletePersistentVolumeClaimRetentionPolicyType,
						},
					},
				},
				pod: &corev1.Pod{},
			},
			want: true,
		},
		{
			name: "Retain PVC Retention Policy",
			args: args{
				claim: &corev1.PersistentVolumeClaim{},
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						PersistentVolumeClaimRetentionPolicy: &slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
							WhenDeleted: slinkyv1alpha1.RetainPersistentVolumeClaimRetentionPolicyType,
						},
					},
				},
				pod: &corev1.Pod{},
			},
			want: true,
		},
		{
			name: "Unknown PVC Retention Policy",
			args: args{
				claim: &corev1.PersistentVolumeClaim{},
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						PersistentVolumeClaimRetentionPolicy: &slinkyv1alpha1.NodeSetPersistentVolumeClaimRetentionPolicy{
							WhenDeleted: "",
						},
					},
				},
				pod: &corev1.Pod{},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateClaimOwnerRefForSetAndPod(klog.Logger{}, tt.args.claim, tt.args.set, tt.args.pod); got != tt.want {
				t.Errorf("updateClaimOwnerRefForSetAndPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasOwnerRef(t *testing.T) {
	type args struct {
		target metav1.Object
		owner  metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Owner reference doesn't match",
			args: args{
				target: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("111"),
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("222")},
						},
					},
				},
				owner: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("111"),
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("222")},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "Owner reference matches",
			args: args{
				target: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("111"),
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("111")},
						},
					},
				},
				owner: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("111"),
						OwnerReferences: []metav1.OwnerReference{
							{UID: types.UID("111")},
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasOwnerRef(tt.args.target, tt.args.owner); got != tt.want {
				t.Errorf("hasOwnerRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasStaleOwnerRef(t *testing.T) {
	type args struct {
		target metav1.Object
		owner  metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Stale Owner Ref",
			args: args{
				target: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
						UID:  types.UID("111"),
						OwnerReferences: []metav1.OwnerReference{
							{Name: "bar", UID: types.UID("222")},
						},
					},
				},
				owner: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
						UID:  types.UID("111"),
					},
				},
			},
			want: true,
		},
		{
			name: "Not a Stale Owner Ref",
			args: args{
				target: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
						UID:  types.UID("111"),
						OwnerReferences: []metav1.OwnerReference{
							{Name: "bar", UID: types.UID("111")},
						},
					},
				},
				owner: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bar",
						UID:  types.UID("111"),
					},
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasStaleOwnerRef(tt.args.target, tt.args.owner); got != tt.want {
				t.Errorf("hasStaleOwnerRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setOwnerRef(t *testing.T) {
	type args struct {
		target    metav1.Object
		owner     metav1.Object
		ownerType *metav1.TypeMeta
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := setOwnerRef(tt.args.target, tt.args.owner, tt.args.ownerType); got != tt.want {
				t.Errorf("setOwnerRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_removeOwnerRef(t *testing.T) {
	type args struct {
		target metav1.Object
		owner  metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeOwnerRef(tt.args.target, tt.args.owner); got != tt.want {
				t.Errorf("removeOwnerRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPersistentVolumeClaimName(t *testing.T) {
	type args struct {
		set   *slinkyv1alpha1.NodeSet
		claim *corev1.PersistentVolumeClaim
		host  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test getPersistentVolumeClaimName",
			args: args{
				set:   &slinkyv1alpha1.NodeSet{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
				claim: &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "bar"}},
				host:  "baz",
			},
			want: "bar-foo-baz",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPersistentVolumeClaimName(tt.args.set, tt.args.claim, tt.args.host); got != tt.want {
				t.Errorf("getPersistentVolumeClaimName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPersistentVolumeClaims(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want map[string]corev1.PersistentVolumeClaim
	}{
		{
			name: "Test getPersistentVolumeClaimName",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "nodeset", Namespace: "default"},
					Spec: slinkyv1alpha1.NodeSetSpec{
						VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "foo",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
					},
				},
				pod: &corev1.Pod{Spec: corev1.PodSpec{Hostname: "host"}},
			},
			want: map[string]corev1.PersistentVolumeClaim{
				"foo": {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-nodeset-host",
						Namespace: "default",
						Labels:    map[string]string{"foo": "bar"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPersistentVolumeClaims(tt.args.set, tt.args.pod); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPersistentVolumeClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateStorage(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Test updateStorage",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "nodeset", Namespace: "default"},
					Spec: slinkyv1alpha1.NodeSetSpec{
						VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "foo",
								},
							},
						},
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"foo": "bar"},
						},
					},
				},
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Hostname: "host",
						Volumes:  []corev1.Volume{{Name: "buz"}},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateStorage(tt.args.set, tt.args.pod)
		})
	}
}

func Test_initIdentity(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initIdentity(tt.args.set, tt.args.pod)
		})
	}
}

func Test_updateIdentity(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateIdentity(tt.args.set, tt.args.pod)
		})
	}
}

func Test_setPodRevision(t *testing.T) {
	type args struct {
		pod      *corev1.Pod
		revision string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setPodRevision(tt.args.pod, tt.args.revision)
		})
	}
}

func Test_getPodRevision(t *testing.T) {
	type args struct {
		pod *corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test getPodRevision (foo)",
			args: args{
				pod: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							slinkyv1alpha1.NodeSetRevisionLabel: "foo",
						},
					},
				},
			},
			want: "foo",
		},
		{
			name: "Test getPodRevision ('')",
			args: args{
				pod: &corev1.Pod{},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getPodRevision(tt.args.pod); got != tt.want {
				t.Errorf("getPodRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getNodeSetRevisionLabel(t *testing.T) {
	type args struct {
		revision *appsv1.ControllerRevision
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getNodeSetRevisionLabel(tt.args.revision); got != tt.want {
				t.Errorf("getNodeSetRevisionLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateNodeSetPodAntiAffinity(t *testing.T) {
	type args struct {
		affinity *corev1.Affinity
	}
	tests := []struct {
		name string
		args args
		want *corev1.Affinity
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateNodeSetPodAntiAffinity(tt.args.affinity); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("updateNodeSetPodAntiAffinity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_newNodeSetPod(t *testing.T) {
	type args struct {
		set      *slinkyv1alpha1.NodeSet
		nodeName string
		hash     string
	}
	tests := []struct {
		name string
		args args
		want *corev1.Pod
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newNodeSetPod(tt.args.set, tt.args.nodeName, tt.args.hash); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newNodeSetPod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPatch(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPatch(tt.args.set)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatch(t *testing.T) {
	type args struct {
		set     *slinkyv1alpha1.NodeSet
		history *appsv1.ControllerRevision
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Match(tt.args.set, tt.args.history)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApplyRevision(t *testing.T) {
	type args struct {
		set      *slinkyv1alpha1.NodeSet
		revision *appsv1.ControllerRevision
	}
	tests := []struct {
		name    string
		args    args
		want    *slinkyv1alpha1.NodeSet
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyRevision(tt.args.set, tt.args.revision)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyRevision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ApplyRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_nodeInSameCondition(t *testing.T) {
	type args struct {
		old []corev1.NodeCondition
		cur []corev1.NodeCondition
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeInSameCondition(tt.args.old, tt.args.cur); got != tt.want {
				t.Errorf("nodeInSameCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_failedPodsBackoffKey(t *testing.T) {
	type args struct {
		set      *slinkyv1alpha1.NodeSet
		nodeName string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test failedPodsBackoffKey",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("1234"),
					},
					Status: slinkyv1alpha1.NodeSetStatus{
						ObservedGeneration: int64(111),
					},
				},
				nodeName: "foo",
			},
			want: "1234/111/foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := failedPodsBackoffKey(tt.args.set, tt.args.nodeName); got != tt.want {
				t.Errorf("failedPodsBackoffKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_predicates(t *testing.T) {
	type args struct {
		pod    *corev1.Pod
		node   *corev1.Node
		taints []corev1.Taint
	}
	tests := []struct {
		name                 string
		args                 args
		wantFitsNodeName     bool
		wantFitsNodeAffinity bool
		wantFitsTaints       bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFitsNodeName, gotFitsNodeAffinity, gotFitsTaints := predicates(tt.args.pod, tt.args.node, tt.args.taints)
			if gotFitsNodeName != tt.wantFitsNodeName {
				t.Errorf("predicates() gotFitsNodeName = %v, want %v", gotFitsNodeName, tt.wantFitsNodeName)
			}
			if gotFitsNodeAffinity != tt.wantFitsNodeAffinity {
				t.Errorf("predicates() gotFitsNodeAffinity = %v, want %v", gotFitsNodeAffinity, tt.wantFitsNodeAffinity)
			}
			if gotFitsTaints != tt.wantFitsTaints {
				t.Errorf("predicates() gotFitsTaints = %v, want %v", gotFitsTaints, tt.wantFitsTaints)
			}
		})
	}
}

func Test_nodeShouldRunNodeSetPod(t *testing.T) {
	type args struct {
		node *corev1.Node
		set  *slinkyv1alpha1.NodeSet
	}
	tests := []struct {
		name  string
		args  args
		want  bool
		want1 bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := nodeShouldRunNodeSetPod(tt.args.node, tt.args.set)
			if got != tt.want {
				t.Errorf("nodeShouldRunNodeSetPod() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("nodeShouldRunNodeSetPod() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_isNodeSetPodAvailable(t *testing.T) {
	type args struct {
		pod             *corev1.Pod
		minReadySeconds int32
		now             metav1.Time
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNodeSetPodAvailable(tt.args.pod, tt.args.minReadySeconds, tt.args.now); got != tt.want {
				t.Errorf("isNodeSetPodAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNodeSetCreationProgressively(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNodeSetCreationProgressively(tt.args.set); got != tt.want {
				t.Errorf("isNodeSetCreationProgressively() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNodeSetPodCordon(t *testing.T) {
	type args struct {
		pod metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNodeSetPodCordon(tt.args.pod); got != tt.want {
				t.Errorf("isNodeSetPodCordon() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNodeSetPodDelete(t *testing.T) {
	type args struct {
		obj metav1.Object
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNodeSetPodDelete(tt.args.obj); got != tt.want {
				t.Errorf("isNodeSetPodDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getNodesNeedingPods(t *testing.T) {
	type args struct {
		newPodsNum       int
		desire           int
		partition        int
		progressive      bool
		nodesNeedingPods []*corev1.Node
	}
	tests := []struct {
		name string
		args args
		want []*corev1.Node
	}{
		{
			name: "Test getNodesNeedingPods (not progressive)",
			args: args{
				newPodsNum:  0,
				desire:      0,
				partition:   0,
				progressive: false,
				nodesNeedingPods: []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "2"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "1"},
						},
					},
				},
			},
			want: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{annotations.NodeWeight: "1"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{annotations.NodeWeight: "2"},
					},
				},
			},
		},
		{
			name: "Test getNodesNeedingPods (want none)",
			args: args{
				newPodsNum:       0,
				desire:           0,
				partition:        0,
				progressive:      false,
				nodesNeedingPods: []*corev1.Node{},
			},
			want: []*corev1.Node{},
		},
		{
			name: "Test getNodesNeedingPods (want one)",
			args: args{
				newPodsNum:  0,
				desire:      1,
				partition:   0,
				progressive: true,
				nodesNeedingPods: []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "2"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "1"},
						},
					},
				},
			},
			want: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{annotations.NodeWeight: "1"},
					},
				},
			},
		},
		{
			name: "Test getNodesNeedingPods (want none)",
			args: args{
				newPodsNum:  0,
				desire:      0,
				partition:   0,
				progressive: true,
				nodesNeedingPods: []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "2"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "1"},
						},
					},
				},
			},
			want: []*corev1.Node{},
		},
		{
			name: "Test getNodesNeedingPods (maxCreate == len(nodesNeedingPods))",
			args: args{
				newPodsNum:  0,
				desire:      10,
				partition:   0,
				progressive: true,
				nodesNeedingPods: []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "2"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "1"},
						},
					},
				},
			},
			want: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{annotations.NodeWeight: "1"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{annotations.NodeWeight: "2"},
					},
				},
			},
		},
		{
			name: "Test getNodesNeedingPods (want none)",
			args: args{
				newPodsNum:  0,
				desire:      0,
				partition:   0,
				progressive: true,
				nodesNeedingPods: []*corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "2"},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{annotations.NodeWeight: "1"},
						},
					},
				},
			},
			want: []*corev1.Node{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getNodesNeedingPods(tt.args.newPodsNum, tt.args.desire, tt.args.partition, tt.args.progressive, tt.args.nodesNeedingPods); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNodesNeedingPods() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isNodeSetPaused(t *testing.T) {
	type args struct {
		set *slinkyv1alpha1.NodeSet
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNodeSetPaused(tt.args.set); got != tt.want {
				t.Errorf("isNodeSetPaused() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_unavailableCount(t *testing.T) {
	type args struct {
		set              *slinkyv1alpha1.NodeSet
		numberToSchedule int
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "Mismatch UpdateStrategy",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						UpdateStrategy: slinkyv1alpha1.NodeSetUpdateStrategy{
							Type: slinkyv1alpha1.OnDeleteNodeSetStrategyType,
						},
					},
				},
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "Mismatch UpdateStrategy (RollingUpdate)",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						UpdateStrategy: slinkyv1alpha1.NodeSetUpdateStrategy{
							Type: slinkyv1alpha1.RollingUpdateNodeSetStrategyType,
						},
					},
				},
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "Mismatch UpdateStrategy (MaxUnavailable == nil)",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						UpdateStrategy: slinkyv1alpha1.NodeSetUpdateStrategy{
							Type:          slinkyv1alpha1.RollingUpdateNodeSetStrategyType,
							RollingUpdate: &slinkyv1alpha1.RollingUpdateNodeSetStrategy{},
						},
					},
				},
				numberToSchedule: 1,
			},
			want:    0,
			wantErr: false,
		},
		{
			name: "Mismatch UpdateStrategy (error)",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						UpdateStrategy: slinkyv1alpha1.NodeSetUpdateStrategy{
							Type: slinkyv1alpha1.RollingUpdateNodeSetStrategyType,
							RollingUpdate: &slinkyv1alpha1.RollingUpdateNodeSetStrategy{
								MaxUnavailable: &intstr.IntOrString{
									Type: intstr.Type(1),
								},
							},
						},
					},
				},
			},
			want:    0,
			wantErr: true,
		},
		{
			name: "Mismatch UpdateStrategy (MaxUnavailable == 1)",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					Spec: slinkyv1alpha1.NodeSetSpec{
						UpdateStrategy: slinkyv1alpha1.NodeSetUpdateStrategy{
							Type: slinkyv1alpha1.RollingUpdateNodeSetStrategyType,
							RollingUpdate: &slinkyv1alpha1.RollingUpdateNodeSetStrategy{
								MaxUnavailable: &intstr.IntOrString{
									Type:   intstr.Type(intstr.Int),
									IntVal: 1,
								},
							},
						},
					},
				},
			},
			want:    1,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unavailableCount(tt.args.set, tt.args.numberToSchedule)
			if (err != nil) != tt.wantErr {
				t.Errorf("unavailableCount() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("unavailableCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getUnscheduledPodsWithoutNode(t *testing.T) {
	type args struct {
		runningNodesList  []*corev1.Node
		nodeToNodeSetPods map[*corev1.Node][]*corev1.Pod
	}
	tests := []struct {
		name string
		args args
		want []*corev1.Pod
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getUnscheduledPodsWithoutNode(tt.args.runningNodesList, tt.args.nodeToNodeSetPods); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getUnscheduledPodsWithoutNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_findUpdatedPodsOnNode(t *testing.T) {
	type args struct {
		set        *slinkyv1alpha1.NodeSet
		podsOnNode []*corev1.Pod
		hash       string
	}
	tests := []struct {
		name       string
		args       args
		wantNewPod *corev1.Pod
		wantOldPod *corev1.Pod
		wantOk     bool
	}{
		{
			name: "test",
			args: args{
				set: &slinkyv1alpha1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							appsv1.DeprecatedTemplateGeneration: "1234",
						},
					},
				},
				podsOnNode: []*corev1.Pod{{Status: corev1.PodStatus{}}},
				hash:       "1234",
			},
			wantNewPod: nil,
			wantOldPod: &corev1.Pod{Status: corev1.PodStatus{}},
			wantOk:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNewPod, gotOldPod, gotOk := findUpdatedPodsOnNode(tt.args.set, tt.args.podsOnNode, tt.args.hash)
			if !reflect.DeepEqual(gotNewPod, tt.wantNewPod) {
				t.Errorf("findUpdatedPodsOnNode() gotNewPod = %v, want %v", gotNewPod, tt.wantNewPod)
			}
			if !reflect.DeepEqual(gotOldPod, tt.wantOldPod) {
				t.Errorf("findUpdatedPodsOnNode() gotOldPod = %v, want %v", gotOldPod, tt.wantOldPod)
			}
			if gotOk != tt.wantOk {
				t.Errorf("findUpdatedPodsOnNode() gotOk = %v, want %v", gotOk, tt.wantOk)
			}
		})
	}
}

func Test_getSafeNodeName(t *testing.T) {
	type args struct {
		nodeName          string
		correctedNodeName string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Non-FQDN nodeName",
			args: args{
				nodeName:          "node1",
				correctedNodeName: "node1",
			},
		},
		{
			name: "FQDN nodeName escaped",
			args: args{
				nodeName:          "node1.us-west-2.compute.internal",
				correctedNodeName: "node1-us-west-2-compute-internal",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSafeNodeName(tt.args.nodeName); got != tt.args.correctedNodeName {
				t.Errorf("safeNodeName() = %v, wanted %v", got, tt.args.correctedNodeName)
			}
		})
	}
}
