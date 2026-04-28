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
		var jwtKeySecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtKeyRef, nil)
			restapi = testutils.NewRestapi(name, controller)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, restapi.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, restapi)
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtKeySecret)
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

		It("Should remove the Slurm Client when its RestApi is deleted but Controller remains", func(ctx SpecContext) {
			controllerKey := client.ObjectKeyFromObject(controller)

			By("Simulating RestApi Deployment Ready Status")
			restapiDeploymentKey := restapi.Key()
			createdDeployment := &appsv1.Deployment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, restapiDeploymentKey, createdDeployment)).To(Succeed())
				createdDeployment.Status.Replicas = 1
				createdDeployment.Status.ReadyReplicas = 1
				g.Expect(k8sClient.Status().Update(ctx, createdDeployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Creating Slurm Client")
			Eventually(func(g Gomega) {
				g.Expect(clientMap.Get(controllerKey)).ShouldNot(BeNil())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting RestApi but leaving Controller in place")
			Expect(k8sClient.Delete(ctx, restapi.DeepCopy())).To(Succeed())

			By("Touching Controller to trigger a reconcile")
			Eventually(func(g Gomega) {
				fresh := &slinkyv1beta1.Controller{}
				g.Expect(k8sClient.Get(ctx, controllerKey, fresh)).To(Succeed())
				if fresh.Annotations == nil {
					fresh.Annotations = map[string]string{}
				}
				fresh.Annotations["test.slinky.slurm.net/touch"] = "1"
				g.Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Removing Slurm Client")
			Eventually(func(g Gomega) {
				g.Expect(clientMap.Get(controllerKey)).Should(BeNil())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
