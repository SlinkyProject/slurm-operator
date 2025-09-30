// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("Token Controller", func() {
	Context("When reconciling a Token", func() {
		var name = testutils.GenerateResourceName(5)
		var token *slinkyv1beta1.Token
		var jwtHs256KeySecret *corev1.Secret

		BeforeEach(func() {
			jwtHs256KeyRef := testutils.NewJwtHs256KeyRef(name)
			jwtHs256KeySecret = testutils.NewJwtHs256KeySecret(jwtHs256KeyRef)
			token = testutils.NewToken(name, jwtHs256KeySecret)
			Expect(k8sClient.Create(ctx, jwtHs256KeySecret.DeepCopy())).To(Succeed())
			Expect(k8sClient.Create(ctx, token.DeepCopy())).To(Succeed())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(ctx, jwtHs256KeySecret)
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
})
