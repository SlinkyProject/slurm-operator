// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package workerbuilder

import (
	_ "embed"
	"reflect"
	"strings"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/common"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildWorkerPodTemplate(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		nodeset    *slinkyv1beta1.NodeSet
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				nodeset: &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm-foo",
					},
					Spec: slinkyv1beta1.NodeSetSpec{
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
						ExtraConf: strings.Join([]string{
							"features=bar",
							"weight=5",
						}, " "),
						Template: slinkyv1beta1.PodTemplate{
							PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
								PodSpec: corev1.PodSpec{
									Hostname: "foo-",
								},
							},
						},
					},
					Status: slinkyv1beta1.NodeSetStatus{
						Selector: k8slabels.SelectorFromSet(k8slabels.Set(labels.NewBuilder().WithWorkerSelectorLabels(&slinkyv1beta1.NodeSet{ObjectMeta: metav1.ObjectMeta{Name: "slurm"}}).Build())).String(),
					},
				},
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got := b.BuildWorkerPodTemplate(tt.args.nodeset, tt.args.controller)
			selector, err := k8slabels.ConvertSelectorToLabelsMap(tt.args.nodeset.Status.Selector)
			if err != nil {
				t.Errorf("ConvertSelectorToLabelsMap() = %v", err)
			}
			switch {
			case !set.KeySet(got.Labels).HasAll(set.KeySet(selector).UnsortedList()...):
				t.Errorf("Labels = %v , Selector = %v", got.Labels, selector)

			case got.Spec.Containers[0].Name != labels.WorkerApp:
				t.Errorf("Containers[0].Name = %v , want = %v",
					got.Spec.Containers[0].Name, labels.WorkerApp)

			case got.Spec.Containers[0].Ports[0].Name != labels.WorkerApp:
				t.Errorf("Containers[0].Ports[0].Name = %v , want = %v",
					got.Spec.Containers[0].Ports[0].Name, labels.WorkerApp)

			case got.Spec.Containers[0].Ports[0].ContainerPort != common.SlurmdPort:
				t.Errorf("Containers[0].Ports[0].ContainerPort = %v , want = %v",
					got.Spec.Containers[0].Ports[0].Name, common.SlurmdPort)

			case got.Spec.Subdomain == "":
				t.Errorf("Subdomain = %v , want = non-empty", got.Spec.Subdomain)

			case got.Spec.DNSConfig == nil:
				t.Errorf("DNSConfig = %v , want = non-nil", got.Spec.DNSConfig)

			case len(got.Spec.DNSConfig.Searches) == 0:
				t.Errorf("len(DNSConfig.Searches) = %v , want = > 0", len(got.Spec.DNSConfig.Searches))
			}
		})
	}
}

func BenchmarkBuilder_BuildWorkerPodTemplate(b *testing.B) {
	type fields struct {
		client client.Client
	}
	type args struct {
		nodeset    *slinkyv1beta1.NodeSet
		controller *slinkyv1beta1.Controller
	}
	benchmarks := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewFakeClient(),
			},
			args: args{
				nodeset: &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm-foo",
					},
					Spec: slinkyv1beta1.NodeSetSpec{
						ControllerRef: corev1.LocalObjectReference{
							Name: "slurm",
						},
						ExtraConf: strings.Join([]string{
							"features=bar",
							"weight=5",
						}, " "),
						Template: slinkyv1beta1.PodTemplate{
							PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
								PodSpec: corev1.PodSpec{
									Hostname: "foo-",
								},
							},
						},
					},
					Status: slinkyv1beta1.NodeSetStatus{
						Selector: k8slabels.SelectorFromSet(k8slabels.Set(labels.NewBuilder().WithWorkerSelectorLabels(&slinkyv1beta1.NodeSet{ObjectMeta: metav1.ObjectMeta{Name: "slurm"}}).Build())).String(),
					},
				},
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				},
			},
		},
	}
	for _, bb := range benchmarks {
		b.Run(bb.name, func(b *testing.B) {
			client := New(bb.fields.client)

			for b.Loop() {
				client.BuildWorkerPodTemplate(bb.args.nodeset, bb.args.controller)
			}
		})
	}
}

func TestWorkerBuilder_getResourceLimits(t *testing.T) {
	client := fake.NewFakeClient()

	// Create values to test resource requirements with
	cpu1, err := resource.ParseQuantity("1")
	if err != nil {
		t.Fatalf("Failed to call resource.ParseQuantity")
	}

	mem1g, err := resource.ParseQuantity("1Gi")
	if err != nil {
		t.Fatalf("Failed to call resource.ParseQuantity")
	}

	cpu4, err := resource.ParseQuantity("4")
	if err != nil {
		t.Fatalf("Failed to call resource.ParseQuantity")
	}

	mem2g, err := resource.ParseQuantity("2Gi")
	if err != nil {
		t.Fatalf("Failed to call resource.ParseQuantity")
	}

	type limits struct {
		cpu    int64
		memory int64
	}
	tests := []struct {
		name    string
		nodeset *slinkyv1beta1.NodeSetSpec
		want    limits
	}{
		{
			name:    "default - no limits",
			nodeset: &slinkyv1beta1.NodeSetSpec{},
			want: limits{
				cpu:    0,
				memory: 0,
			},
		},
		{
			name: "pod limits - cpu",
			nodeset: &slinkyv1beta1.NodeSetSpec{
				Template: slinkyv1beta1.PodTemplate{
					PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
						PodSpec: corev1.PodSpec{
							Resources: &corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu": cpu1,
								},
							},
						},
					},
				},
			},
			want: limits{
				cpu:    1,
				memory: 0,
			},
		},
		{
			name: "pod limits - memory",
			nodeset: &slinkyv1beta1.NodeSetSpec{
				Template: slinkyv1beta1.PodTemplate{
					PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
						PodSpec: corev1.PodSpec{
							Resources: &corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"memory": mem1g,
								},
							},
						},
					},
				},
			},
			want: limits{
				cpu:    0,
				memory: 1024,
			},
		},
		{
			name: "pod limits - cpu & memory",
			nodeset: &slinkyv1beta1.NodeSetSpec{
				Template: slinkyv1beta1.PodTemplate{
					PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
						PodSpec: corev1.PodSpec{
							Resources: &corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    cpu1,
									"memory": mem1g,
								},
							},
						},
					},
				},
			},
			want: limits{
				cpu:    1,
				memory: 1024,
			},
		},
		{
			name: "container limits - cpu",
			nodeset: &slinkyv1beta1.NodeSetSpec{
				Slurmd: slinkyv1beta1.ContainerWrapper{
					Container: corev1.Container{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu": cpu4,
							},
						},
					},
				},
			},
			want: limits{
				cpu: 4,
			},
		},
		{
			name: "container limits - memory",
			nodeset: &slinkyv1beta1.NodeSetSpec{
				Slurmd: slinkyv1beta1.ContainerWrapper{
					Container: corev1.Container{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"memory": mem2g,
							},
						},
					},
				},
			},
			want: limits{
				memory: 2048,
			},
		},
		{
			name: "container limits - cpu & memory",
			nodeset: &slinkyv1beta1.NodeSetSpec{
				Slurmd: slinkyv1beta1.ContainerWrapper{
					Container: corev1.Container{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    cpu4,
								"memory": mem2g,
							},
						},
					},
				},
			},
			want: limits{
				cpu:    4,
				memory: 2048,
			},
		},
		{
			name: "pod & container limits - cpu & memory",
			nodeset: &slinkyv1beta1.NodeSetSpec{
				Template: slinkyv1beta1.PodTemplate{
					PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
						PodSpec: corev1.PodSpec{
							Resources: &corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"cpu":    cpu4,
									"memory": mem2g,
								},
							},
						},
					},
				},
				Slurmd: slinkyv1beta1.ContainerWrapper{
					Container: corev1.Container{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    cpu1,
								"memory": mem1g,
							},
						},
					},
				},
			},
			want: limits{
				cpu:    1,
				memory: 1024,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(client)
			cpu, memory := b.getResourceLimits(tt.nodeset)
			if cpu != tt.want.cpu {
				t.Errorf("getResourceLimits() = %v, want %v", cpu, tt.want.cpu)
			}
			if memory != tt.want.memory {
				t.Errorf("getResourceLimits() = %v, want %v", memory, tt.want.memory)
			}
		})
	}
}

func TestParseExtraConf(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    map[string][]string
		wantErr bool
	}{
		{name: "empty", input: "", want: map[string][]string{}},
		{name: "single", input: "Weight=10", want: map[string][]string{"Weight": {"10"}}},
		{name: "multiple keys", input: "Feature=a Weight=10", want: map[string][]string{"Feature": {"a"}, "Weight": {"10"}}},
		{name: "csv value splits", input: "Features=a,b,c", want: map[string][]string{"Features": {"a", "b", "c"}}},
		{name: "case normalised", input: "feature=a", want: map[string][]string{"Feature": {"a"}}},
		{name: "repeated and surrounding whitespace tolerated", input: "  Weight=5  Feature=a ", want: map[string][]string{"Weight": {"5"}, "Feature": {"a"}}},
		{name: "missing equals", input: "Weight10", wantErr: true},
		{name: "missing equals among valid", input: "Weight=5 nope Feature=a", wantErr: true},
		{name: "Feature and Features stay distinct keys", input: "Feature=a Features=b", want: map[string][]string{"Feature": {"a"}, "Features": {"b"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseExtraConf(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseExtraConf() err = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseExtraConf() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBaselineFeatures(t *testing.T) {
	cases := []struct {
		name      string
		nodesetFn func() *slinkyv1beta1.NodeSet
		want      []string
	}{
		{
			name: "name only",
			nodesetFn: func() *slinkyv1beta1.NodeSet {
				return &slinkyv1beta1.NodeSet{ObjectMeta: metav1.ObjectMeta{Name: "gpu"}}
			},
			want: []string{"gpu"},
		},
		{
			name: "single Feature=",
			nodesetFn: func() *slinkyv1beta1.NodeSet {
				return &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
					Spec:       slinkyv1beta1.NodeSetSpec{ExtraConf: "Feature=a100"},
				}
			},
			want: []string{"a100", "gpu"},
		},
		{
			name: "Features= CSV",
			nodesetFn: func() *slinkyv1beta1.NodeSet {
				return &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
					Spec:       slinkyv1beta1.NodeSetSpec{ExtraConf: "Features=a100,nvlink"},
				}
			},
			want: []string{"a100", "gpu", "nvlink"},
		},
		{
			name: "multiple Feature= entries merged and deduped",
			nodesetFn: func() *slinkyv1beta1.NodeSet {
				return &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
					Spec:       slinkyv1beta1.NodeSetSpec{ExtraConf: "Feature=a100 Feature=nvlink Features=a100"},
				}
			},
			want: []string{"a100", "gpu", "nvlink"},
		},
		{
			name: "hostname override replaces nodeset name",
			nodesetFn: func() *slinkyv1beta1.NodeSet {
				return &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
					Spec: slinkyv1beta1.NodeSetSpec{
						Template: slinkyv1beta1.PodTemplate{
							PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
								PodSpec: corev1.PodSpec{Hostname: "gpu-x"},
							},
						},
					},
				}
			},
			want: []string{"gpu-x"},
		},
		{
			name: "malformed ExtraConf falls back to bare baseline",
			nodesetFn: func() *slinkyv1beta1.NodeSet {
				return &slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
					Spec:       slinkyv1beta1.NodeSetSpec{ExtraConf: "bad"},
				}
			},
			want: []string{"gpu"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := baselineFeatures(tc.nodesetFn())
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("baselineFeatures() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSlurmdConfArgs(t *testing.T) {
	cases := []struct {
		name    string
		nodeset *slinkyv1beta1.NodeSet
		want    []string
	}{
		{
			name:    "name only",
			nodeset: &slinkyv1beta1.NodeSet{ObjectMeta: metav1.ObjectMeta{Name: "gpu"}},
			want:    []string{"--conf", "'Features=gpu'"},
		},
		{
			name: "feature and non-feature keys",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
				Spec:       slinkyv1beta1.NodeSetSpec{ExtraConf: "Weight=10 feature=a"},
			},
			want: []string{"--conf", "'Features=a,gpu Weight=10'"},
		},
		{
			name: "Features CSV sorted and merged with name",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
				Spec:       slinkyv1beta1.NodeSetSpec{ExtraConf: "Features=z,a"},
			},
			want: []string{"--conf", "'Features=a,gpu,z'"},
		},
		{
			name: "hostname override is the feature name",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
				Spec: slinkyv1beta1.NodeSetSpec{
					Template: slinkyv1beta1.PodTemplate{
						PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
							PodSpec: corev1.PodSpec{Hostname: "foo-"},
						},
					},
				},
			},
			want: []string{"--conf", "'Features=foo'"},
		},
		{
			name: "malformed ExtraConf degrades to baseline instead of panicking",
			nodeset: &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{Name: "gpu"},
				Spec:       slinkyv1beta1.NodeSetSpec{ExtraConf: "Weight10 Feature=a"},
			},
			want: []string{"--conf", "'Features=gpu'"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := slurmdConfArgs(tc.nodeset)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("slurmdConfArgs() = %v, want %v", got, tc.want)
			}
		})
	}
}
