// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

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

var _ = Describe("Slurm Controller", func() {
	Context("When creating Controller", func() {
		var name = testutils.GenerateResourceName(5)
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtKeySecret *corev1.Secret

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtKeyRef, nil)
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtKeySecret)
		})

		It("Should successfully create create a controller", func(ctx SpecContext) {
			By("Creating Controller CR")
			createdController := &slinkyv1beta1.Controller{}
			controllerKey := client.ObjectKeyFromObject(controller)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, controllerKey, createdController)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Expecting Controller CR service")
			serviceKey := controller.ServiceKey()
			service := &corev1.Service{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, serviceKey, service)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Expecting Controller CR statefulset")
			statefulsetKey := controller.Key()
			statefulset := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, statefulsetKey, statefulset)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))

		It("Should skip sync when Controller is being deleted", func(ctx SpecContext) {
			By("Waiting for Controller children to be created")
			statefulsetKey := controller.Key()
			statefulset := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, statefulsetKey, statefulset)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting Controller with foreground propagation")
			controllerKey := client.ObjectKeyFromObject(controller)
			Expect(k8sClient.Delete(ctx, controller,
				client.PropagationPolicy(metav1.DeletePropagationForeground),
			)).To(Succeed())

			By("Verifying Controller has deletionTimestamp set")
			Eventually(func(g Gomega) {
				controller := &slinkyv1beta1.Controller{}
				g.Expect(k8sClient.Get(ctx, controllerKey, controller)).To(Succeed())
				g.Expect(controller.DeletionTimestamp.IsZero()).To(BeFalse())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting StatefulSet child while Controller is terminating")
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
			controller := &slinkyv1beta1.Controller{}
			Expect(k8sClient.Get(ctx, controllerKey, controller)).To(Succeed())
			controller.Finalizers = nil
			Expect(k8sClient.Update(ctx, controller)).To(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
