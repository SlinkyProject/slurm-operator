// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		It("Should warn if external IPs are set", func(ctx SpecContext) {
			controller := testutils.NewController("clustername", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			controller.Spec.Service.ServiceSpecWrapper.ExternalIPs = []string{"169.254.169.254"}

			warnings, err := controllerWebhook.ValidateCreate(ctx, controller)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(ContainElement("ExternalIPs may not be set for controller service"))
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

		It("Should reject disable of controller.ha.enabled", func(ctx SpecContext) {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: metav1.NamespaceDefault,
					Name:      "statesave",
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("4Gi"),
						},
					},
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
				},
			}
			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())

			oldController := testutils.NewController("cluster", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldController.Spec.HighAvailability.Enabled = false
			oldController.Spec.Persistence.Enabled = new(true)
			oldController.Spec.Persistence.ExistingClaim = pvc.Name

			newController := oldController.DeepCopy()
			newController.Spec.HighAvailability.Enabled = true

			_, err := controllerWebhook.ValidateUpdate(ctx, oldController, newController)
			Expect(err).To(HaveOccurred())

			_ = k8sClient.Delete(ctx, pvc)
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
