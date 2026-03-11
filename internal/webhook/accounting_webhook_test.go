// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Accounting Webhook", func() {
	Context("When updating an Accounting with Validating Webhook", func() {
		It("Should deny an Update if JwtRef has changed", func() {
			By("Returning an error")
			oldJwtKeyRef := testutils.NewJwtKeyRef("test-secret")
			oldAccounting := testutils.NewAccounting("test-accounting", corev1.SecretKeySelector{}, oldJwtKeyRef, corev1.SecretKeySelector{})

			newJwtKeyRef := testutils.NewJwtKeyRef("test-secret-2")
			newAccounting := testutils.NewAccounting("test-accounting", corev1.SecretKeySelector{}, newJwtKeyRef, corev1.SecretKeySelector{})

			_, err := accountingWebhook.ValidateUpdate(ctx, oldAccounting, newAccounting)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit an Update if JwtRef has not changed", func() {
			By("Not returning an error")
			newJwtKeyRef := testutils.NewJwtKeyRef("test-secret-2")
			newAccounting := testutils.NewAccounting("test-accounting", corev1.SecretKeySelector{}, newJwtKeyRef, corev1.SecretKeySelector{})

			_, err := accountingWebhook.ValidateUpdate(ctx, newAccounting, newAccounting)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When creating Accounting with Validating Webhook", func() {
		It("Should admit a Create for a CRD that passes Kube validation", func() {
			By("Not returning an error")
			newAccounting := testutils.NewAccounting("test-accounting", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, corev1.SecretKeySelector{})

			_, err := accountingWebhook.ValidateCreate(ctx, newAccounting)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When deleting Accounting with Validating Webhook", func() {
		It("Should admit a Delete for a CRD that passes Kube validation", func() {
			By("Not returning an error")
			newAccounting := testutils.NewAccounting("test-accounting", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, corev1.SecretKeySelector{})

			_, err := accountingWebhook.ValidateDelete(ctx, newAccounting)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
