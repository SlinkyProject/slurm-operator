// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package loginset

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("LoginSet Controller", func() {
	Context("When reconciling a LoginSet", func() {
		var name string
		var loginset *slinkyv1beta1.LoginSet
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtKeySecret *corev1.Secret
		var sssdConfSecret *corev1.Secret

		BeforeEach(func() {
			name = testutils.GenerateResourceName(5)
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtKeyRef, nil)
			sssdconfRef := testutils.NewSssdConfRef(name)
			sssdConfSecret = testutils.NewSssdConfSecret(sssdconfRef)
			loginset = testutils.NewLoginset(name, controller, sssdconfRef)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, sssdConfSecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, loginset.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtKeySecret)
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, sssdConfSecret)
			_ = k8sClient.Delete(ctx, loginset)
		})

		It("Should skip sync when LoginSet is being deleted", func(ctx SpecContext) {
			By("Waiting for LoginSet children to be created")
			deploymentKey := loginset.Key()
			deployment := &appsv1.Deployment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting LoginSet with foreground propagation")
			loginsetKey := client.ObjectKeyFromObject(loginset)
			Expect(k8sClient.Delete(ctx, loginset,
				client.PropagationPolicy(metav1.DeletePropagationForeground),
			)).To(Succeed())

			By("Verifying LoginSet has deletionTimestamp set")
			Eventually(func(g Gomega) {
				ls := &slinkyv1beta1.LoginSet{}
				g.Expect(k8sClient.Get(ctx, loginsetKey, ls)).To(Succeed())
				g.Expect(ls.DeletionTimestamp.IsZero()).To(BeFalse())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Waiting for the controller's cache to observe the deletionTimestamp")
			Eventually(func(g Gomega) {
				ls := &slinkyv1beta1.LoginSet{}
				g.Expect(k8sManagerClient.Get(ctx, loginsetKey, ls)).To(Succeed())
				g.Expect(ls.DeletionTimestamp.IsZero()).To(BeFalse())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting Deployment child while LoginSet is terminating")
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())

			By("Verifying Deployment child is NOT recreated")
			const requiredConsecutiveAbsences = 5
			consecutiveAbsences := 0
			Eventually(func(g Gomega) {
				if err := k8sClient.Get(ctx, deploymentKey, deployment); err == nil {
					Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
					consecutiveAbsences = 0
					g.Expect(consecutiveAbsences).To(BeNumerically(">=", requiredConsecutiveAbsences), "Deployment was recreated")
					return
				}
				consecutiveAbsences++
				g.Expect(consecutiveAbsences).To(BeNumerically(">=", requiredConsecutiveAbsences))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Cleaning up: removing foregroundDeletion finalizer")
			ls := &slinkyv1beta1.LoginSet{}
			Expect(k8sClient.Get(ctx, loginsetKey, ls)).To(Succeed())
			ls.Finalizers = nil
			Expect(k8sClient.Update(ctx, ls)).To(Succeed())
		}, SpecTimeout(testutils.Timeout))

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
