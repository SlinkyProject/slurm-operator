// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/confighash"
)

var _ = Describe("NodeSet config-change rollout", func() {
	It("creates a new ControllerRevision when a mounted ConfigMap changes", func() {
		ns := corev1.NamespaceDefault

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "rollout-cm", Namespace: ns},
			Data:       map[string]string{"app.conf": "version=1"},
		}
		Expect(k8sClient.Create(ctx, cm)).To(Succeed())

		nodeset := &slinkyv1beta1.NodeSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "rollout-ns",
				Namespace:   ns,
				Annotations: map[string]string{slinkyv1beta1.AnnotationReloadOnChange: "true"},
			},
			Spec: slinkyv1beta1.NodeSetSpec{
				ControllerRef: slinkyv1beta1.ObjectReference{Name: "rollout-controller", Namespace: ns},
			},
		}
		nodeset.Spec.Template.PodSpecWrapper.PodSpec.Containers = []corev1.Container{
			{Name: "worker", Image: "busybox"},
		}
		nodeset.Spec.Template.PodSpecWrapper.PodSpec.Volumes = []corev1.Volume{
			{Name: "cfg", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "rollout-cm"}}}},
		}
		Expect(k8sClient.Create(ctx, nodeset)).To(Succeed())

		key := confighash.AnnotationKey(confighash.ConfigMapKindPrefix, "rollout-cm")

		// The latest ControllerRevision should carry a checksum annotation in its template.
		var firstRevisionCount int
		Eventually(func(g Gomega) {
			revs := listConfigRolloutRevisions(g, ns, "rollout-ns")
			g.Expect(len(revs)).To(BeNumerically(">=", 1))
			firstRevisionCount = len(revs)
			latest := revs[len(revs)-1]
			g.Expect(string(latest.Data.Raw)).To(ContainSubstring(key))
		}).Should(Succeed())

		// Mutate the ConfigMap; expect a new revision to appear.
		Eventually(func(g Gomega) {
			fresh := &corev1.ConfigMap{}
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cm), fresh)).To(Succeed())
			fresh.Data["app.conf"] = "version=2"
			g.Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
		}).Should(Succeed())

		Eventually(func(g Gomega) {
			revs := listConfigRolloutRevisions(g, ns, "rollout-ns")
			g.Expect(len(revs)).To(BeNumerically(">", firstRevisionCount))
		}).Should(Succeed())
	})
})

func listConfigRolloutRevisions(g Gomega, namespace, nodesetName string) []appsv1.ControllerRevision {
	list := &appsv1.ControllerRevisionList{}
	g.Expect(k8sClient.List(ctx, list, client.InNamespace(namespace))).To(Succeed())
	out := []appsv1.ControllerRevision{}
	for _, r := range list.Items {
		for _, owner := range r.OwnerReferences {
			if owner.Kind == "NodeSet" && owner.Name == nodesetName {
				out = append(out, r)
			}
		}
	}
	return out
}
