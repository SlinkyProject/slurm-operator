// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/labels"
)

func TestPodBindingWebhook_Default(t *testing.T) {
	workerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-0",
			Namespace: corev1.NamespaceDefault,
			Labels: map[string]string{
				labels.AppLabel: labels.WorkerApp,
			},
			Annotations: map[string]string{
				"kubectl.kubernetes.io/default-container": "slurmd",
			},
		},
	}

	nonWorkerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "other-pod",
			Namespace: corev1.NamespaceDefault,
			Labels: map[string]string{
				labels.AppLabel: "nginx",
			},
		},
	}

	nodeWithTopology := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Annotations: map[string]string{
				slinkyv1beta1.AnnotationNodeTopologySpec: "topo-switch:s2,topo-block:b2",
			},
		},
	}

	nodeWithoutTopology := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
		},
	}

	type args struct {
		ctx     context.Context
		binding *corev1.Binding
	}
	tests := []struct {
		name          string
		client        client.Client
		args          args
		wantErr       bool
		wantTopology  string
		checkTopology bool
	}{
		{
			name:   "Worker pod gets topology annotation from node",
			client: fake.NewFakeClient(workerPod.DeepCopy(), nodeWithTopology.DeepCopy()),
			args: args{
				ctx: context.TODO(),
				binding: &corev1.Binding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workerPod.Name,
						Namespace: workerPod.Namespace,
					},
					Target: corev1.ObjectReference{Name: nodeWithTopology.Name},
				},
			},
			wantErr:       false,
			wantTopology:  "topo-switch:s2,topo-block:b2",
			checkTopology: true,
		},
		{
			name:   "Worker pod gets empty topology when node has no annotation",
			client: fake.NewFakeClient(workerPod.DeepCopy(), nodeWithoutTopology.DeepCopy()),
			args: args{
				ctx: context.TODO(),
				binding: &corev1.Binding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workerPod.Name,
						Namespace: workerPod.Namespace,
					},
					Target: corev1.ObjectReference{Name: nodeWithoutTopology.Name},
				},
			},
			wantErr:       false,
			wantTopology:  "",
			checkTopology: true,
		},
		{
			name:   "Non-worker pod is skipped",
			client: fake.NewFakeClient(nonWorkerPod.DeepCopy()),
			args: args{
				ctx: context.TODO(),
				binding: &corev1.Binding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nonWorkerPod.Name,
						Namespace: nonWorkerPod.Namespace,
					},
					Target: corev1.ObjectReference{Name: "any-node"},
				},
			},
			wantErr: false,
		},
		{
			name:   "Pod with no labels is skipped",
			client: fake.NewFakeClient(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "bare-pod", Namespace: corev1.NamespaceDefault}}),
			args: args{
				ctx: context.TODO(),
				binding: &corev1.Binding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bare-pod",
						Namespace: corev1.NamespaceDefault,
					},
					Target: corev1.ObjectReference{Name: "any-node"},
				},
			},
			wantErr: false,
		},
		{
			name:   "Node not found is a no-op",
			client: fake.NewFakeClient(workerPod.DeepCopy()),
			args: args{
				ctx: context.TODO(),
				binding: &corev1.Binding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workerPod.Name,
						Namespace: workerPod.Namespace,
					},
					Target: corev1.ObjectReference{Name: "nonexistent-node"},
				},
			},
			wantErr: false,
		},
		{
			name:   "Pod not found returns error",
			client: fake.NewFakeClient(),
			args: args{
				ctx: context.TODO(),
				binding: &corev1.Binding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "missing-pod",
						Namespace: corev1.NamespaceDefault,
					},
					Target: corev1.ObjectReference{Name: "any-node"},
				},
			},
			wantErr: true,
		},
		{
			name: "Patch failure returns error",
			client: fake.NewClientBuilder().
				WithRuntimeObjects(workerPod.DeepCopy(), nodeWithTopology.DeepCopy()).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, client client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return http.ErrAbortHandler
					},
				}).
				Build(),
			args: args{
				ctx: context.TODO(),
				binding: &corev1.Binding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      workerPod.Name,
						Namespace: workerPod.Namespace,
					},
					Target: corev1.ObjectReference{Name: nodeWithTopology.Name},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PodBindingWebhook{Client: tt.client}
			if err := r.Default(tt.args.ctx, tt.args.binding); (err != nil) != tt.wantErr {
				t.Errorf("PodBindingWebhook.Default() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.checkTopology {
				gotPod := &corev1.Pod{}
				podKey := client.ObjectKeyFromObject(tt.args.binding)
				if err := tt.client.Get(tt.args.ctx, podKey, gotPod); err != nil {
					t.Fatalf("failed to get pod after Default: %v", err)
				}
				got := gotPod.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec]
				if got != tt.wantTopology {
					t.Errorf("pod topology annotation = %q, want %q", got, tt.wantTopology)
				}
			}
		})
	}
}
