// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginset

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("LoginSet Controller", func() {
	Context("When reconciling a LoginSet", func() {
		var name = testutils.GenerateResourceName(5)
		var loginset *slinkyv1beta1.LoginSet
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtHs256KeySecret *corev1.Secret
		var sssdConfSecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtHs256KeySecret = testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
			sssdconfRef := testutils.NewSssdConfRef(name)
			sssdConfSecret = testutils.NewSssdConfSecret(sssdconfRef)
			loginset = testutils.NewLoginset(name, controller, sssdconfRef)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtHs256KeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, sssdConfSecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, loginset.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtHs256KeySecret)
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, sssdConfSecret)
			_ = k8sClient.Delete(ctx, loginset)
		})

		It("Should successfully create create an loginset", func(ctx SpecContext) {
			By("Creating LoginSet CR")
			createdLoginset := &slinkyv1beta1.Controller{}
			loginsetKey := client.ObjectKeyFromObject(controller)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, loginsetKey, createdLoginset)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Creating LoginSet CR Service")
			serviceKey := loginset.ServiceKey()
			service := &corev1.Service{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, serviceKey, service)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Creating LoginSet CR Deployment")
			deploymentKey := loginset.Key()
			deployment := &appsv1.Deployment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
