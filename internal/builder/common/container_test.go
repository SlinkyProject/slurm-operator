// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildContainer(t *testing.T) {
	tests := []struct {
		name   string
		client client.Client
		opts   ContainerOpts
		want   corev1.Container
	}{
		{
			name:   "empty",
			client: fake.NewFakeClient(),
			opts:   ContainerOpts{},
			want:   corev1.Container{},
		},
		{
			name:   "merge",
			client: fake.NewFakeClient(),
			opts: ContainerOpts{
				Base: corev1.Container{
					Name:            "foo",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args:            []string{"-a", "-b"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
				},
				Merge: corev1.Container{
					Name:  "bar",
					Image: "nginx",
					Args:  []string{"-c"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				},
			},
			want: corev1.Container{
				Name:            "bar",
				Image:           "nginx",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Args:            []string{"-a", "-b", "-c"},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				},
			},
		},
		{
			name:   "livenessProbe exec replaces httpGet",
			client: fake.NewFakeClient(),
			opts: ContainerOpts{
				Base: corev1.Container{
					Name: "slurmctld",
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: SlurmLivez,
								Port: intstr.FromString("slurmctld"),
							},
						},
						FailureThreshold: 6,
						PeriodSeconds:    10,
					},
				},
				Merge: corev1.Container{
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"true"},
							},
						},
					},
				},
			},
			want: corev1.Container{
				Name: "slurmctld",
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						Exec: &corev1.ExecAction{
							Command: []string{"true"},
						},
					},
					FailureThreshold: 6,
					PeriodSeconds:    10,
				},
			},
		},
		{
			name:   "livenessProbe exec with custom thresholds",
			client: fake.NewFakeClient(),
			opts: ContainerOpts{
				Base: corev1.Container{
					Name: "slurmctld",
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: SlurmLivez,
								Port: intstr.FromString("slurmctld"),
							},
						},
						FailureThreshold: 6,
						PeriodSeconds:    10,
					},
				},
				Merge: corev1.Container{
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"true"},
							},
						},
						FailureThreshold: 3,
						PeriodSeconds:    5,
					},
				},
			},
			want: corev1.Container{
				Name: "slurmctld",
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						Exec: &corev1.ExecAction{
							Command: []string{"true"},
						},
					},
					FailureThreshold: 3,
					PeriodSeconds:    5,
				},
			},
		},
		{
			name:   "livenessProbe exec preserves other probes",
			client: fake.NewFakeClient(),
			opts: ContainerOpts{
				Base: corev1.Container{
					Name: "slurmctld",
					StartupProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: SlurmLivez,
								Port: intstr.FromString("slurmctld"),
							},
						},
						FailureThreshold: 6,
						PeriodSeconds:    10,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: SlurmReadyz,
								Port: intstr.FromString("slurmctld"),
							},
						},
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: SlurmLivez,
								Port: intstr.FromString("slurmctld"),
							},
						},
						FailureThreshold: 6,
						PeriodSeconds:    10,
					},
				},
				Merge: corev1.Container{
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"true"},
							},
						},
					},
				},
			},
			want: corev1.Container{
				Name: "slurmctld",
				StartupProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: SlurmLivez,
							Port: intstr.FromString("slurmctld"),
						},
					},
					FailureThreshold: 6,
					PeriodSeconds:    10,
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: SlurmReadyz,
							Port: intstr.FromString("slurmctld"),
						},
					},
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						Exec: &corev1.ExecAction{
							Command: []string{"true"},
						},
					},
					FailureThreshold: 6,
					PeriodSeconds:    10,
				},
			},
		},
		{
			name:   "livenessProbe httpGet override preserves handler type",
			client: fake.NewFakeClient(),
			opts: ContainerOpts{
				Base: corev1.Container{
					Name: "slurmctld",
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: SlurmLivez,
								Port: intstr.FromString("slurmctld"),
							},
						},
						FailureThreshold: 6,
						PeriodSeconds:    10,
					},
				},
				Merge: corev1.Container{
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.FromString("slurmctld"),
							},
						},
					},
				},
			},
			want: corev1.Container{
				Name: "slurmctld",
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromString("slurmctld"),
						},
					},
					FailureThreshold: 6,
					PeriodSeconds:    10,
				},
			},
		},
		{
			name:   "no merge probe leaves base untouched",
			client: fake.NewFakeClient(),
			opts: ContainerOpts{
				Base: corev1.Container{
					Name: "slurmctld",
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: SlurmLivez,
								Port: intstr.FromString("slurmctld"),
							},
						},
						FailureThreshold: 6,
						PeriodSeconds:    10,
					},
				},
				Merge: corev1.Container{},
			},
			want: corev1.Container{
				Name: "slurmctld",
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: SlurmLivez,
							Port: intstr.FromString("slurmctld"),
						},
					},
					FailureThreshold: 6,
					PeriodSeconds:    10,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.client)
			got := b.BuildContainer(tt.opts)
			if !apiequality.Semantic.DeepEqual(got, tt.want) {
				t.Errorf("Builder.BuildContainer() = %v, want %v", got, tt.want)
				return
			}
		})
	}
}
