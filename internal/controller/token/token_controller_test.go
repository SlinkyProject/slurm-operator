// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("Token Controller", func() {
	Context("When reconciling a Token", func() {
		var name = testutils.GenerateResourceName(5)
		var token *slinkyv1beta1.Token
		var jwtKeySecret *corev1.Secret

		BeforeEach(func() {
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			token = testutils.NewToken(name, jwtKeySecret)
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, token.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, jwtKeySecret)
			_ = k8sClient.Delete(ctx, token)
		})

		It("Should successfully create create a token", func(ctx SpecContext) {
			By("Creating Token CR")
			createdToken := &slinkyv1beta1.Token{}
			tokenKey := client.ObjectKeyFromObject(token)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, tokenKey, createdToken)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))
	})

	Context("When deleting a Token", func() {
		var name = testutils.GenerateResourceName(5)
		var token *slinkyv1beta1.Token
		var jwtKeySecret *corev1.Secret

		BeforeEach(func() {
			jwtKeyRef := testutils.NewJwtKeyRef(name)
			jwtKeySecret = testutils.NewJwtKeySecret(jwtKeyRef)
			token = testutils.NewToken(name, jwtKeySecret)
			Expect(k8sClient.Create(ctx, jwtKeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, token.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, jwtKeySecret)
			_ = k8sClient.Delete(ctx, token)
		})

		It("Should successfully create create a token", func(ctx SpecContext) {
			By("Creating Token CR")
			tokenKey := client.ObjectKeyFromObject(token)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, tokenKey, token)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(testutils.Timeout))

		It("Should skip sync when the Token is being deleted", func(ctx SpecContext) {
			By("Creating Token CR")
			tokenKey := client.ObjectKeyFromObject(token)
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, tokenKey, token)).To(Succeed())
			}).Should(Succeed())

			By("Waiting for Token child to be created")
			secretKey := token.JwtKey()
			secret := &corev1.Secret{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Deleting Token with foregroud propagation")
			Expect(k8sClient.Delete(ctx, token,
				client.PropagationPolicy(metav1.DeletePropagationForeground),
			)).To(Succeed())

			By("Deleting Secret child while Token is terminating")
			Expect(k8sClient.Get(ctx, secretKey, secret)).To(Succeed())
			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, secretKey, secret)
				g.Expect(err).To(HaveOccurred())
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			}, testutils.Timeout, testutils.Interval).Should(Succeed())

			By("Verifying Secret child is NOT recreated")
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, secretKey, secret)
				g.Expect(err).To(HaveOccurred())
				g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			}, 5*testutils.Interval, testutils.Interval).Should(Succeed())

			By("Cleaning up: removing foregroundDeletion finalizer")
			Expect(k8sClient.Get(ctx, tokenKey, token)).To(Succeed())
			token.Finalizers = nil
			Expect(k8sClient.Update(ctx, token)).To(Succeed())
		})

	})
})
