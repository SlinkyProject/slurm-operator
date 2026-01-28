// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controllerbuilder

import (
	"math/rand/v2"
	"strings"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuilder_BuildControllerConfig(t *testing.T) {
	type fields struct {
		client client.Client
	}
	type args struct {
		controller *slinkyv1beta1.Controller
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantErr     bool
		wantScripts []string
	}{
		{
			name: "default",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name: "prolog",
						},
						Data: map[string]string{
							"00-exit.sh": strings.Join([]string{
								"#!/usr/bin/sh",
								"exit 0",
							}, "\n"),
						},
					}).
					WithObjects(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name: "epilog",
						},
						Data: map[string]string{
							"00-exit.sh": strings.Join([]string{
								"#!/usr/bin/sh",
								"exit 0",
							}, "\n"),
						},
					}).
					Build(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						ExtraConf: strings.Join([]string{
							"MinJobAge=2",
						}, "\n"),
						PrologScriptRefs: []slinkyv1beta1.ObjectReference{
							{Name: "prolog"},
						},
						EpilogScriptRefs: []slinkyv1beta1.ObjectReference{
							{Name: "epilog"},
						},
					},
				},
			},
		},
		{
			name: "with accounting, nodesets, config",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&slinkyv1beta1.Accounting{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					}).
					WithObjects(&slinkyv1beta1.Controller{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm",
						},
					}).
					WithObjects(&slinkyv1beta1.NodeSet{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm-foo",
						},
						Spec: slinkyv1beta1.NodeSetSpec{
							ControllerRef: slinkyv1beta1.ObjectReference{
								Name: "slurm",
							},
							ExtraConf: strings.Join([]string{
								"features=bar",
							}, " "),
							Partition: slinkyv1beta1.NodeSetPartition{
								Enabled: true,
							},
							Template: slinkyv1beta1.PodTemplate{
								PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
									PodSpec: corev1.PodSpec{
										Hostname: "foo-",
									},
								},
							},
						},
					}).
					WithObjects(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name: "slurm-config",
						},
						Data: map[string]string{
							CgroupConfFile: `# Override cgroup.conf
							CgroupPlugin=autodetect
							IgnoreSystemd=yes
							ConstrainCores=yes
							ConstrainRAMSpace=yes
							ConstrainDevices=yes
							ConstrainSwapSpace=yes`,
							"foo.conf": "Foo=bar",
						},
					}).
					Build(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
					Spec: slinkyv1beta1.ControllerSpec{
						AccountingRef: slinkyv1beta1.ObjectReference{
							Name: "slurm",
						},
						ConfigFileRefs: []slinkyv1beta1.ObjectReference{
							{Name: "slurm-config"},
						},
					},
				},
			},
		},
		{
			name: "multiple prolog configmaps",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "prolog-1"},
						Data: map[string]string{
							"00-first.sh": strings.Join([]string{
								"#!/usr/bin/sh",
								"exit 0",
							}, "\n"),
						},
					}).
					WithObjects(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "prolog-2"},
						Data: map[string]string{
							"90-second.sh": strings.Join([]string{
								"#!/usr/bin/sh",
								"exit 0",
							}, "\n"),
						},
					}).
					Build(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{Name: "slurm"},
					Spec: slinkyv1beta1.ControllerSpec{
						PrologScriptRefs: []slinkyv1beta1.ObjectReference{
							{Name: "prolog-1"},
							{Name: "prolog-2"},
						},
					},
				},
			},
			wantScripts: []string{"00-first.sh", "90-second.sh"},
		},
		{
			name: "multiple epilog configmaps",
			fields: fields{
				client: fake.NewClientBuilder().
					WithObjects(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "epilog-1"},
						Data: map[string]string{
							"00-cleanup.sh": strings.Join([]string{
								"#!/usr/bin/sh",
								"exit 0",
							}, "\n"),
						},
					}).
					WithObjects(&corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{Name: "epilog-2"},
						Data: map[string]string{
							"90-finalize.sh": strings.Join([]string{
								"#!/usr/bin/sh",
								"exit 0",
							}, "\n"),
						},
					}).
					Build(),
			},
			args: args{
				controller: &slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{Name: "slurm"},
					Spec: slinkyv1beta1.ControllerSpec{
						EpilogScriptRefs: []slinkyv1beta1.ObjectReference{
							{Name: "epilog-1"},
							{Name: "epilog-2"},
						},
					},
				},
			},
			wantScripts: []string{"00-cleanup.sh", "90-finalize.sh"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.fields.client)
			got, err := b.BuildControllerConfig(tt.args.controller)
			if (err != nil) != tt.wantErr {
				t.Errorf("Builder.BuildControllerConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			switch {
			case err != nil:
				return

			case got.Data[SlurmConfFile] == "" && got.BinaryData[SlurmConfFile] == nil:
				t.Errorf("got.Data[%s] = %v", SlurmConfFile, got.Data[SlurmConfFile])
			}

			// Verify expected scripts are present in slurm.conf
			for _, script := range tt.wantScripts {
				if !strings.Contains(got.Data[SlurmConfFile], script) {
					t.Errorf("Expected %s in slurm.conf", script)
				}
			}
		})
	}
}

func Test_isCgroupEnabled(t *testing.T) {
	type args struct {
		cgroupConf string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "enabled",
			args: args{
				cgroupConf: "CgroupPlugin=autodetect",
			},
			want: true,
		},
		{
			name: "enabled, lowercase+multiline+comment",
			args: args{
				cgroupConf: `# Multiline file
cgroupplugin=autodetect # this is a comment
ignoresystemd=yes`,
			},
			want: true,
		},
		{
			name: "disabled",
			args: args{
				cgroupConf: "CgroupPlugin=disabled",
			},
			want: false,
		},
		{
			name: "disabled, lowercase+multiline+comment",
			args: args{
				cgroupConf: `# Multiline file
cgroupplugin=disabled # this is a comment
ignoresystemd=yes`,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCgroupEnabled(tt.args.cgroupConf); got != tt.want {
				t.Errorf("isCgroupEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuilder_BuildControllerConfigExternal(t *testing.T) {
	tests := []struct {
		name       string
		c          client.Client
		controller *slinkyv1beta1.Controller
		want       *corev1.ConfigMap
		wantErr    bool
	}{
		{
			name: "default",
			c:    fake.NewFakeClient(),
			controller: &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slurm",
				},
			},
			want: &corev1.ConfigMap{
				Data: map[string]string{
					SlurmConfFile: "#\n### GENERAL ###\nClusterName=_slurm\nSlurmUser=slurm\nSlurmctldHost=slurm-controller-0\nSlurmctldPort=6817\n#\n### PLUGINS & PARAMETERS ###\nAuthType=auth/slurm\nCredType=cred/slurm\nAuthAltTypes=auth/jwt\n#\n### ACCOUNTING ###\nAccountingStorageType=accounting_storage/none\n",
				},
			},
			wantErr: false,
		},
		{
			name: "With config",
			c:    fake.NewFakeClient(),
			controller: &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slurm",
				},
				Spec: slinkyv1beta1.ControllerSpec{
					ExtraConf: strings.Join([]string{
						"MinJobAge=2",
					}, "\n"),
				},
			},
			want: &corev1.ConfigMap{
				Data: map[string]string{
					SlurmConfFile: "#\n### GENERAL ###\nClusterName=_slurm\nSlurmUser=slurm\nSlurmctldHost=slurm-controller-0\nSlurmctldPort=6817\n#\n### PLUGINS & PARAMETERS ###\nAuthType=auth/slurm\nCredType=cred/slurm\nAuthAltTypes=auth/jwt\n#\n### ACCOUNTING ###\nAccountingStorageType=accounting_storage/none\n",
				},
			},
			wantErr: false,
		},
		{
			name: "With accounting, nodesets, config",
			c: fake.NewClientBuilder().
				WithObjects(&slinkyv1beta1.Accounting{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				}).
				WithObjects(&slinkyv1beta1.Controller{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm",
					},
				}).
				WithObjects(&slinkyv1beta1.NodeSet{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm-foo",
					},
					Spec: slinkyv1beta1.NodeSetSpec{
						ControllerRef: slinkyv1beta1.ObjectReference{
							Name: "slurm",
						},
						ExtraConf: strings.Join([]string{
							"features=bar",
						}, " "),
						Partition: slinkyv1beta1.NodeSetPartition{
							Enabled: true,
						},
						Template: slinkyv1beta1.PodTemplate{
							PodSpecWrapper: slinkyv1beta1.PodSpecWrapper{
								PodSpec: corev1.PodSpec{
									Hostname: "foo-",
								},
							},
						},
					},
				}).
				WithObjects(&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name: "slurm-config",
					},
					Data: map[string]string{
						CgroupConfFile: `# Override cgroup.conf
							CgroupPlugin=autodetect
							IgnoreSystemd=yes
							ConstrainCores=yes
							ConstrainRAMSpace=yes
							ConstrainDevices=yes
							ConstrainSwapSpace=yes`,
						"foo.conf": "Foo=bar",
					},
				}).
				Build(),
			controller: &slinkyv1beta1.Controller{
				ObjectMeta: metav1.ObjectMeta{
					Name: "slurm",
				},
				Spec: slinkyv1beta1.ControllerSpec{
					AccountingRef: slinkyv1beta1.ObjectReference{
						Name: "slurm",
					},
					ConfigFileRefs: []slinkyv1beta1.ObjectReference{
						{Name: "slurm-config"},
					},
				},
			},
			want: &corev1.ConfigMap{
				Data: map[string]string{
					SlurmConfFile: "#\n### GENERAL ###\nClusterName=_slurm\nSlurmUser=slurm\nSlurmctldHost=slurm-controller-0\nSlurmctldPort=6817\n#\n### PLUGINS & PARAMETERS ###\nAuthType=auth/slurm\nCredType=cred/slurm\nAuthAltTypes=auth/jwt\n#\n### ACCOUNTING ###\nAccountingStorageType=accounting_storage/slurmdbd\nAccountingStorageHost=slurm-accounting\nAccountingStoragePort=6819\n",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(tt.c)
			got, gotErr := b.BuildControllerConfigExternal(tt.controller)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("BuildControllerConfigExternal() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("BuildControllerConfigExternal() succeeded unexpectedly")
			}
			if got.Data[SlurmConfFile] == "" && got.BinaryData[SlurmConfFile] == nil {
				t.Errorf("got.Data[%s] = %v", SlurmConfFile, got.Data[SlurmConfFile])
			}
			if got.Data[SlurmConfFile] != tt.want.Data[SlurmConfFile] {
				t.Errorf("got.Data[%s] = %v\nwant.Data[%s] = %v", SlurmConfFile, got.Data[SlurmConfFile], SlurmConfFile, tt.want.Data[SlurmConfFile])

			}
		})
	}
}

func Test_buildNodeSetConf(t *testing.T) {
	tests := []struct {
		name        string
		nodesetList *slinkyv1beta1.NodeSetList
		want        string
	}{
		{
			name: "empty",
			nodesetList: &slinkyv1beta1.NodeSetList{
				Items: []slinkyv1beta1.NodeSet{},
			},
			want: "",
		},
		{
			name: "non-empty",
			nodesetList: &slinkyv1beta1.NodeSetList{
				Items: []slinkyv1beta1.NodeSet{
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: metav1.NamespaceDefault,
							Name:      "nodeset-0",
						},
						Spec: slinkyv1beta1.NodeSetSpec{
							Partition: slinkyv1beta1.NodeSetPartition{
								Enabled: false,
								Config:  "MaxTime=UNLIMITED OverSubscribe=EXCLUSIVE",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: metav1.NamespaceDefault,
							Name:      "nodeset-1",
						},
						Spec: slinkyv1beta1.NodeSetSpec{
							Partition: slinkyv1beta1.NodeSetPartition{
								Enabled: true,
								Config:  "MaxTime=UNLIMITED OverSubscribe=EXCLUSIVE",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Namespace: metav1.NamespaceDefault,
							Name:      "nodeset-2",
						},
						Spec: slinkyv1beta1.NodeSetSpec{
							Partition: slinkyv1beta1.NodeSetPartition{
								Enabled: true,
								Config:  "MaxTime=UNLIMITED PreemptMode=REQUEUE",
							},
						},
					},
				},
			},
			want: `#
### COMPUTE & PARTITION ###
NodeSet=nodeset-0 Feature=nodeset-0
NodeSet=nodeset-1 Feature=nodeset-1
PartitionName=nodeset-1 Nodes=nodeset-1 MaxTime=UNLIMITED OverSubscribe=EXCLUSIVE
NodeSet=nodeset-2 Feature=nodeset-2
PartitionName=nodeset-2 Nodes=nodeset-2 MaxTime=UNLIMITED PreemptMode=REQUEUE`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := len(tt.nodesetList.Items)
			for range 5 {
				idx := rand.Perm(size)
				randomized := make([]slinkyv1beta1.NodeSet, size)
				for j := range size {
					randomized[j] = tt.nodesetList.Items[idx[j]]
				}
				got := buildNodeSetConf(tt.nodesetList)
				if got != tt.want {
					t.Errorf("buildNodeSetConf() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
