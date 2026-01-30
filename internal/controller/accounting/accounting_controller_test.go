// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/utils/testutils"
)

var _ = Describe("Accounting controller", func() {
	Context("When creating Accounting", func() {
		var name = testutils.GenerateResourceName(5)
		var accounting *slinkyv1beta1.Accounting
		var slurmKeySecret *corev1.Secret
		var jwtHs256KeySecret *corev1.Secret
		var passwordSecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
			passwordRef := testutils.NewPasswordRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtHs256KeySecret = testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
			passwordSecret = testutils.NewPasswordSecret(passwordRef)
			accounting = testutils.NewAccounting(name, slurmKeyRef, jwtHs256KeyRef, passwordRef)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtHs256KeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, passwordSecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, accounting.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, accounting)
			_ = k8sClient.Delete(ctx, passwordSecret)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtHs256KeySecret)
		})

		It("Should successfully create create a accounting", func(ctx SpecContext) {
			By("Creating Accounting CR")
			createdAccounting := &slinkyv1beta1.Accounting{}
			accountingKey := client.ObjectKeyFromObject(accounting)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, accountingKey, createdAccounting)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Expecting Accounting CR Service")
			serviceKey := accounting.ServiceKey()
			service := &corev1.Service{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, serviceKey, service)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Expecting Accounting CR Statefulset")
			statefulsetKey := accounting.Key()
			statefulset := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, statefulsetKey, statefulset)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
