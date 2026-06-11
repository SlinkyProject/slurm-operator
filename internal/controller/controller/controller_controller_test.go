// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

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

		It("Should recreate the Service when HA replicas toggles headlessness", func(ctx SpecContext) {
			By("Waiting for the singleton ClusterIP Service")
			serviceKey := controller.ServiceKey()
			service := &corev1.Service{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, serviceKey, service)).To(Succeed())
				g.Expect(service.Spec.ClusterIP).ToNot(Equal(corev1.ClusterIPNone))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
			originalUID := service.UID

			By("Enabling HA")
			controllerKey := client.ObjectKeyFromObject(controller)
			Eventually(func(g Gomega) {
				current := &slinkyv1beta1.Controller{}
				g.Expect(k8sClient.Get(ctx, controllerKey, current)).To(Succeed())
				current.Spec.Replicas = ptr.To[int32](2)
				current.Spec.Persistence.ExistingClaim = "statesave"
				g.Expect(k8sClient.Update(ctx, current)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Expecting the Service to be recreated headless")
			Eventually(func(g Gomega) {
				recreated := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, serviceKey, recreated)).To(Succeed())
				g.Expect(recreated.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
				g.Expect(recreated.UID).ToNot(Equal(originalUID))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
			headless := &corev1.Service{}
			Expect(k8sClient.Get(ctx, serviceKey, headless)).To(Succeed())
			headlessUID := headless.UID

			By("Disabling HA")
			Eventually(func(g Gomega) {
				current := &slinkyv1beta1.Controller{}
				g.Expect(k8sClient.Get(ctx, controllerKey, current)).To(Succeed())
				current.Spec.Replicas = ptr.To[int32](1)
				g.Expect(k8sClient.Update(ctx, current)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Expecting the Service to be recreated with a ClusterIP")
			Eventually(func(g Gomega) {
				restored := &corev1.Service{}
				g.Expect(k8sClient.Get(ctx, serviceKey, restored)).To(Succeed())
				g.Expect(restored.Spec.ClusterIP).ToNot(Equal(corev1.ClusterIPNone))
				g.Expect(restored.UID).ToNot(Equal(headlessUID))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout*2))

		It("Should reject service overrides combined with replicas > 1", func(ctx SpecContext) {
			By("Updating the Controller with replicas=2 and a nodePort override")
			controllerKey := client.ObjectKeyFromObject(controller)
			Eventually(func(g Gomega) {
				current := &slinkyv1beta1.Controller{}
				g.Expect(k8sClient.Get(ctx, controllerKey, current)).To(Succeed())
				current.Spec.Replicas = ptr.To[int32](2)
				current.Spec.Persistence.ExistingClaim = "statesave"
				current.Spec.Service.NodePort = 32500
				err := k8sClient.Update(ctx, current)
				g.Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected Invalid, got: %v", err)
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})

	Context("When a Service the operator does not own occupies the controller Service name", func() {
		var name = testutils.GenerateResourceName(5)
		var controller *slinkyv1beta1.Controller
		var slurmKeySecret *corev1.Secret
		var jwtKeySecret *corev1.Secret
		var unowned *corev1.Service

		BeforeEach(func() {
			slurmKeyRef := testutils.NewSlurmKeyRef(name)
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			slurmKeySecret = testutils.NewSlurmKeySecret(slurmKeyRef)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			controller = testutils.NewController(name, slurmKeyRef, jwtKeyRef, nil)
			// Headless where the controller (replicas=1) wants ClusterIP, and
			// no owner reference: a mismatch the operator must not delete.
			unowned = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controller.ServiceKey().Name,
					Namespace: controller.ServiceKey().Namespace,
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: corev1.ClusterIPNone,
					Ports: []corev1.ServicePort{
						{Name: "slurmctld", Port: 6817},
					},
				},
			}
			Expect(k8sClient.Create(ctx, slurmKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, unowned.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, controller.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, controller)
			_ = k8sClient.Delete(ctx, unowned)
			_ = k8sClient.Delete(ctx, slurmKeySecret)
			_ = k8sClient.Delete(ctx, jwtKeySecret)
		})

		It("Should not delete a Service it does not own", func(ctx SpecContext) {
			By("Waiting for the Controller children to be created")
			statefulsetKey := controller.Key()
			statefulset := &appsv1.StatefulSet{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, statefulsetKey, statefulset)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Verifying the unowned Service survives reconciliation")
			existing := &corev1.Service{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(unowned), existing)).To(Succeed())
			Expect(existing.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
			Expect(existing.OwnerReferences).To(BeEmpty())
		}, SpecTimeout(testutils.Timeout))
	})
})
