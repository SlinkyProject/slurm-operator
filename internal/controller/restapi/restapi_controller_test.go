// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package restapi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/builder/restapibuilder"
	testutils "github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("RestApi Controller", func() {
	Context("When reconciling a RestApi", func() {
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

			By("Expecting no PodDisruptionBudget when replicas is one")
			pdbKey := types.NamespacedName{
				Namespace: restapi.Namespace,
				Name:      restapibuilder.RestapiPodDisruptionBudgetName(restapi),
			}
			pdb := &policyv1.PodDisruptionBudget{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, pdbKey, pdb)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))

		It("Should create PodDisruptionBudget when replicas is greater than one", func(ctx SpecContext) {
			By("Waiting for Deployment")
			deploymentKey := restapi.Key()
			deployment := &appsv1.Deployment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Patching RestApi to two replicas")
			restapiKey := client.ObjectKeyFromObject(restapi)
			Expect(k8sClient.Get(ctx, restapiKey, restapi)).To(Succeed())
			restapi.Spec.Replicas = ptr.To(int32(2))
			Expect(k8sClient.Update(ctx, restapi)).To(Succeed())

			By("Expecting PodDisruptionBudget")
			pdbKey := types.NamespacedName{
				Namespace: restapi.Namespace,
				Name:      restapibuilder.RestapiPodDisruptionBudgetName(restapi),
			}
			pdb := &policyv1.PodDisruptionBudget{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, pdbKey, pdb)).To(Succeed())
				g.Expect(pdb.Spec.MinAvailable).NotTo(BeNil())
				g.Expect(pdb.Spec.MinAvailable.IntValue()).To(Equal(1))
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))

		It("Should skip sync when RestApi is being deleted", func(ctx SpecContext) {
			By("Waiting for RestApi children to be created")
			deploymentKey := restapi.Key()
			deployment := &appsv1.Deployment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting RestApi with foreground propagation")
			restapiKey := client.ObjectKeyFromObject(restapi)
			Expect(k8sClient.Delete(ctx, restapi,
				client.PropagationPolicy(metav1.DeletePropagationForeground),
			)).To(Succeed())

			By("Verifying RestApi has deletionTimestamp set")
			Eventually(func(g Gomega) {
				restapi := &slinkyv1beta1.RestApi{}
				g.Expect(k8sClient.Get(ctx, restapiKey, restapi)).To(Succeed())
				g.Expect(restapi.DeletionTimestamp.IsZero()).To(BeFalse())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting Deployment child while RestApi is terminating")
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, deploymentKey, deployment)
				g.Expect(err).To(HaveOccurred())
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Verifying Deployment child is NOT recreated")
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, deploymentKey, deployment)
				g.Expect(err).To(HaveOccurred())
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			}, 5*testutils.Interval, testutils.Interval).Should(Succeed())

			By("Cleaning up: removing foregroundDeletion finalizer")
			restapi := &slinkyv1beta1.RestApi{}
			Expect(k8sClient.Get(ctx, restapiKey, restapi)).To(Succeed())
			restapi.Finalizers = nil
			Expect(k8sClient.Update(ctx, restapi)).To(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})

	Context("When PodDisruptionBudget is disabled with multiple replicas", func() {
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
			restapi.Spec.Replicas = ptr.To(int32(2))
			restapi.Spec.PodDisruptionBudget = &slinkyv1beta1.RestApiPodDisruptionBudget{
				Enabled: ptr.To(false),
			}
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

		It("Should not create PodDisruptionBudget", func(ctx SpecContext) {
			deploymentKey := restapi.Key()
			deployment := &appsv1.Deployment{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			pdbKey := types.NamespacedName{
				Namespace: restapi.Namespace,
				Name:      restapibuilder.RestapiPodDisruptionBudgetName(restapi),
			}
			pdb := &policyv1.PodDisruptionBudget{}
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, pdbKey, pdb)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})
})
