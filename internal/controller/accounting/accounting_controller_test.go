// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("Accounting controller", func() {
	Context("When creating Accounting", func() {
		var name = testutils.GenerateResourceName(5)
		var accounting *slinkyv1beta1.Accounting
		var slurmKeySecret *corev1.Secret
		var jwtKeySecret *corev1.Secret
		var passwordSecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			passwordRef := testutils.NewPasswordRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			passwordSecret = testutils.NewPasswordSecret(passwordRef)
			accounting = testutils.NewAccounting(name, slurmKeyRef, jwtKeyRef, passwordRef)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, passwordSecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, accounting.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, accounting)
			_ = k8sClient.Delete(ctx, passwordSecret)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtKeySecret)
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

		It("Should skip sync when Accounting is being deleted", func(ctx SpecContext) {
			By("Waiting for Accounting children to be created")
			statefulsetKey := accounting.Key()
			statefulset := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, statefulsetKey, statefulset)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting Accounting with foreground propagation")
			accountingKey := client.ObjectKeyFromObject(accounting)
			Expect(k8sClient.Delete(ctx, accounting,
				client.PropagationPolicy(metav1.DeletePropagationForeground),
			)).To(Succeed())

			By("Verifying Accounting has deletionTimestamp set")
			Eventually(func(g Gomega) {
				accounting := &slinkyv1beta1.Accounting{}
				g.Expect(k8sClient.Get(ctx, accountingKey, accounting)).To(Succeed())
				g.Expect(accounting.DeletionTimestamp.IsZero()).To(BeFalse())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting StatefulSet child while Accounting is terminating")
			Expect(k8sClient.Get(ctx, statefulsetKey, statefulset)).To(Succeed())
			Expect(k8sClient.Delete(ctx, statefulset)).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, statefulsetKey, statefulset)
				g.Expect(err).To(HaveOccurred())
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Verifying StatefulSet child is NOT recreated")
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, statefulsetKey, statefulset)
				g.Expect(err).To(HaveOccurred())
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			}, 5*testutils.Interval, testutils.Interval).Should(Succeed())

			By("Cleaning up: removing foregroundDeletion finalizer")
			accounting := &slinkyv1beta1.Accounting{}
			Expect(k8sClient.Get(ctx, accountingKey, accounting)).To(Succeed())
			accounting.Finalizers = nil
			Expect(k8sClient.Update(ctx, accounting)).To(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
