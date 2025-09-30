// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("Slurm Controller", func() {
	Context("When creating Controller", func() {
		var name = testutils.GenerateResourceName(5)
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtHs256KeySecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtHs256KeySecret = testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtHs256KeyRef, nil)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtHs256KeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtHs256KeySecret)
		})

		It("Should successfully create create a controller", func(ctx SpecContext) {
			By("Creating Controller CR")
			createdController := &slinkyv1beta1.Controller{}
			controllerKey := client.ObjectKeyFromObject(controller)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, controllerKey, createdController)).To(Succeed())
			}).Should(Succeed())

			By("Expecting Controller CR service")
			serviceKey := controller.ServiceKey()
			service := &corev1.Service{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, serviceKey, service)).To(Succeed())
			}).Should(Succeed())

			By("Expecting Controller CR statefulset")
			statefulsetKey := controller.Key()
			statefulset := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, statefulsetKey, statefulset)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
