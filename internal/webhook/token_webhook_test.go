// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Token Webhook", func() {
	Context("When creating Token under Validating Webhook", func() {
		It("Should admit a Create for a CRD that passes Kube validation", func() {
			By("Not returning an error")
			newToken := testutils.NewToken("test", &corev1.Secret{})

			_, err := tokenWebhook.ValidateCreate(ctx, newToken)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When updating Token under Validating Webhook", func() {
		It("Should deny if a required field is empty", func() {
			oldToken := testutils.NewToken("token", &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
			})
			oldToken.Spec.JwtKeyRef = &v1beta1.JwtSecretKeySelector{
				SecretKeySelector: testutils.NewJwtKeyRef("test"),
			}

			newToken := testutils.NewToken("token", &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
			})
			newToken.Spec.JwtKeyRef = &v1beta1.JwtSecretKeySelector{
				SecretKeySelector: testutils.NewJwtKeyRef("test2"),
			}

			_, err := tokenWebhook.ValidateUpdate(ctx, oldToken, newToken)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if all required fields are provided", func() {
			oldToken := testutils.NewToken("token", &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
			})
			oldToken.Spec.JwtKeyRef = &v1beta1.JwtSecretKeySelector{
				SecretKeySelector: testutils.NewJwtKeyRef("test"),
			}

			newToken := testutils.NewToken("token", &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "token",
				},
			})
			newToken.Spec.JwtKeyRef = &v1beta1.JwtSecretKeySelector{
				SecretKeySelector: testutils.NewJwtKeyRef("test"),
			}

			_, err := tokenWebhook.ValidateUpdate(ctx, oldToken, newToken)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When deleting Token under Validating Webhook", func() {
		It("Should admit a Delete for a CRD that passes Kube validation", func() {
			By("Not returning an error")
			newToken := testutils.NewToken("test", &corev1.Secret{})

			_, err := tokenWebhook.ValidateDelete(ctx, newToken)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
