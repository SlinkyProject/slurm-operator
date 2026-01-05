// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmclient

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("SlurmClient Controller", func() {
	Context("When reconciling Controller", func() {
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
			controllerKey := client.ObjectKeyFromObject(controller)

			By("Expecting RestApi Deployment")
			restapiDeploymentKey := restapi.Key()
			createdDeployment := &appsv1.Deployment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, restapiDeploymentKey, createdDeployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Simulating RestApi Deployment Ready Status")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, restapiDeploymentKey, createdDeployment)).To(Succeed())
				createdDeployment.Status.Replicas = 1
				createdDeployment.Status.ReadyReplicas = 1
				g.Expect(k8sClient.Status().Update(ctx, createdDeployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Creating Slurm Client")
			Eventually(func(g Gomega) {
				slurmClient := clientMap.Get(controllerKey)
				g.Expect(slurmClient).ShouldNot(BeNil())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting Controller")
			Expect(k8sClient.Delete(ctx, controller.DeepCopy())).To(Succeed())

			By("Removing Slurm Client")
			Eventually(func(g Gomega) {
				slurmClient := clientMap.Get(controllerKey)
				g.Expect(slurmClient).Should(BeNil())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
