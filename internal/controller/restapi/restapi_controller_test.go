// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	testutils "github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("RestApi Controller", func() {
	Context("When reconciling a RestApi", func() {
		var name = testutils.GenerateResourceName(5)
		var restapi *slinkyv1beta1.RestApi
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtHs256KeySecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtHs256KeySecret = testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
			restapi = testutils.NewRestapi(name, controller)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtHs256KeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, restapi.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, restapi)
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtHs256KeySecret)
		})

		It("Should successfully create create a restapi", func(ctx SpecContext) {
			By("Creating RestApi CR")
			createdRestapi := &slinkyv1beta1.RestApi{}
			restapiKey := client.ObjectKeyFromObject(restapi)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, restapiKey, createdRestapi)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Expecting RestApi CR Service")
			serviceKey := restapi.ServiceKey()
			service := &corev1.Service{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, serviceKey, service)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Expecting RestApi CR Deployment")
			deploymentKey := restapi.Key()
			deployment := &appsv1.Deployment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
