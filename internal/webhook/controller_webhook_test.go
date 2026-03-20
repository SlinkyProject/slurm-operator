// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("Controller Webhook", func() {
	Context("When Creating a Controller with Validating Webhook", func() {
		It("Should deny if ClusterName exceeds 40 characters", func(ctx SpecContext) {
			controller := testutils.NewController("thisclusternameisverylongandthereforewillcausecontrollerfailure", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)

			_, err := controllerWebhook.ValidateCreate(ctx, controller)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if all required fields are provided and ClusterName is compliant", func(ctx SpecContext) {
			controller := testutils.NewController("clustername", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)

			_, err := controllerWebhook.ValidateCreate(ctx, controller)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Updating a Controller with Validating Webhook", func() {
		It("Should reject changes to ClusterName", func(ctx SpecContext) {
			oldController := testutils.NewController("cluster2", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			newController := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)

			_, err := controllerWebhook.ValidateUpdate(ctx, oldController, newController)
			Expect(err).To(HaveOccurred())
		})

		It("Should reject changes to SlurmKeyRef", func(ctx SpecContext) {
			oldSlurmKey := testutils.NewSlurmKeyRef("test")
			oldController := testutils.NewController("cluster", oldSlurmKey, corev1.SecretKeySelector{}, nil)

			newSlurmKey := testutils.NewSlurmKeyRef("test2")
			newController := testutils.NewController("cluster", newSlurmKey, corev1.SecretKeySelector{}, nil)

			_, err := controllerWebhook.ValidateUpdate(ctx, oldController, newController)
			Expect(err).To(HaveOccurred())
		})

		It("Should reject changes to JwtKeyRef", func(ctx SpecContext) {
			oldJwtKey := testutils.NewJwtKeyRef("test")
			oldController := testutils.NewController("cluster", corev1.SecretKeySelector{}, oldJwtKey, nil)

			newJwtKey := testutils.NewJwtKeyRef("test2")
			newController := testutils.NewController("cluster", corev1.SecretKeySelector{}, newJwtKey, nil)

			_, err := controllerWebhook.ValidateUpdate(ctx, oldController, newController)
			Expect(err).To(HaveOccurred())
		})

		It("Should reject changes to controller.persistence.enabled", func(ctx SpecContext) {
			oldController := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)

			newController := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			newController.Spec.Persistence.Enabled = ptr.To(true)

			_, err := controllerWebhook.ValidateUpdate(ctx, oldController, newController)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if changes pass validation", func(ctx SpecContext) {
			oldController := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			newController := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			newController.Spec.ExtraConf = "SchedulerParameters=allow_zero_lic"

			_, err := controllerWebhook.ValidateUpdate(ctx, oldController, newController)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Deleting a Controller with Validating Webhook", func() {
		It("Should admit a Delete for a CRD that passes Kube validation", func(ctx SpecContext) {
			By("Not returning an error")
			newController := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)

			_, err := controllerWebhook.ValidateDelete(ctx, newController)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
