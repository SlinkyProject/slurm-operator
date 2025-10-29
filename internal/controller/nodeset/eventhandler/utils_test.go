// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package eventhandler

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func init() {
	utilruntime.Must(slinkyv1beta1.AddToScheme(clientgoscheme.Scheme))
}

func newQueue() workqueue.TypedRateLimitingInterface[reconcile.Request] {
	return workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
}

func newNodeSet(name, controllerName string, replicas int32) *slinkyv1beta1.NodeSet {
	return &slinkyv1beta1.NodeSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: corev1.NamespaceDefault,
			Name:      name,
		},
		Spec: slinkyv1beta1.NodeSetSpec{
			ControllerRef: slinkyv1beta1.ObjectReference{
				Namespace: corev1.NamespaceDefault,
				Name:      controllerName,
			},
			Replicas: ptr.To(replicas),
			Template: slinkyv1beta1.PodTemplate{
				PodMetadata: slinkyv1beta1.Metadata{
					Labels: map[string]string{
						"foo": "bar",
					},
				},
			},
			Slurmd: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "slurmd",
				},
			},
			ExtraConf: "Weight=10",
			LogFile: slinkyv1beta1.ContainerWrapper{
				Container: corev1.Container{
					Image: "alpine",
				},
			},
		},
	}
}
