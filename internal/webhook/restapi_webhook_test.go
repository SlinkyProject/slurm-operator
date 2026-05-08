// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

var _ = Describe("Restapi Webhook", func() {
	Context("When updating Restapi under Validating Webhook", func() {
		It("Should admit an Update for a CRD that passes Kube validation", func() {
			By("Not returning an error")
			controller := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldRestapi := testutils.NewRestapi("test", controller)

			newRestapi := testutils.NewRestapi("test", controller)
			newRestapi.Spec.Replicas = ptr.To(int32(2))

			_, err := restapiWebhook.ValidateUpdate(ctx, oldRestapi, newRestapi)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit when replicas is one and PDB is enabled even if minAvailable and maxUnavailable are both set", func() {
			controller := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldRestapi := testutils.NewRestapi("test", controller)
			oldRestapi.Spec.Replicas = ptr.To(int32(1))
			newRestapi := oldRestapi.DeepCopy()
			newRestapi.Spec.PodDisruptionBudget = &slinkyv1beta1.RestApiPodDisruptionBudget{
				PodDisruptionBudgetSpec: policyv1.PodDisruptionBudgetSpec{
					MinAvailable:   ptr.To(intstr.FromInt(2)),
					MaxUnavailable: ptr.To(intstr.FromInt(1)),
				},
			}
			_, err := restapiWebhook.ValidateUpdate(ctx, oldRestapi, newRestapi)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject when replicas is two and minAvailable and maxUnavailable are both set", func() {
			controller := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldRestapi := testutils.NewRestapi("test", controller)
			oldRestapi.Spec.Replicas = ptr.To(int32(1))
			newRestapi := oldRestapi.DeepCopy()
			newRestapi.Spec.Replicas = ptr.To(int32(2))
			newRestapi.Spec.PodDisruptionBudget = &slinkyv1beta1.RestApiPodDisruptionBudget{
				PodDisruptionBudgetSpec: policyv1.PodDisruptionBudgetSpec{
					MinAvailable:   ptr.To(intstr.FromInt(1)),
					MaxUnavailable: ptr.To(intstr.FromInt(1)),
				},
			}
			_, err := restapiWebhook.ValidateUpdate(ctx, oldRestapi, newRestapi)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("minAvailable and maxUnavailable cannot both be set"))
		})
	})

	Context("When creating RestAPI with Validating Webhook", func() {
		It("Should admit a Create for a CRD that passes Kube validation", func() {
			By("Not returning an error")
			controller := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			restapi := testutils.NewRestapi("test", controller)

			_, err := restapiWebhook.ValidateCreate(ctx, restapi)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When deleting RestAPI with Validating Webhook", func() {
		It("Should admit a Delete for a CRD that passes Kube validation", func() {
			By("Not returning an error")
			controller := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			restapi := testutils.NewRestapi("test", controller)

			_, err := restapiWebhook.ValidateDelete(ctx, restapi)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
