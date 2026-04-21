// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("LoginSet Webhook", func() {
	Context("When Creating a LoginSet with Validating Webhook", func() {
		It("Should deny if controllerRef.name is empty", func(ctx SpecContext) {
			loginset := testutils.NewLoginset("test-loginset", nil, testutils.NewSssdConfRef("test"))

			_, err := loginSetWebhook.ValidateCreate(ctx, loginset)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny if sssdConfRef.name is empty", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			loginset := testutils.NewLoginset("test-loginset", controller, corev1.SecretKeySelector{})

			_, err := loginSetWebhook.ValidateCreate(ctx, loginset)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if all required fields are provided", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			loginset := testutils.NewLoginset("test-loginset", controller, testutils.NewSssdConfRef("test"))

			_, err := loginSetWebhook.ValidateCreate(ctx, loginset)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Updating a LoginSet with Validating Webhook", func() {
		It("Should reject changes to controllerRef", func(ctx SpecContext) {
			oldController := testutils.NewController("old-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldLoginSet := testutils.NewLoginset("test-loginset", oldController, testutils.NewSssdConfRef("test"))

			newController := testutils.NewController("new-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			newLoginSet := testutils.NewLoginset("test-loginset", newController, testutils.NewSssdConfRef("test"))

			_, err := loginSetWebhook.ValidateUpdate(ctx, oldLoginSet, newLoginSet)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if no immutable fields change", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldLoginSet := testutils.NewLoginset("test-loginset", controller, testutils.NewSssdConfRef("test"))
			newLoginSet := testutils.NewLoginset("test-loginset", controller, testutils.NewSssdConfRef("test"))
			newLoginSet.Spec.Replicas = ptr.To(int32(2))

			_, err := loginSetWebhook.ValidateUpdate(ctx, oldLoginSet, newLoginSet)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Deleting a LoginSet with Validating Webhook", func() {
		It("Should admit a Delete", func(ctx SpecContext) {
			loginset := testutils.NewLoginset("test-loginset", nil, corev1.SecretKeySelector{})

			_, err := loginSetWebhook.ValidateDelete(ctx, loginset)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When validating Spec.Strategy", func() {
		newWithStrategy := func(s appsv1.DeploymentStrategy) *slinkyv1beta1.LoginSet {
			controller := testutils.NewController("c", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			ls := testutils.NewLoginset("test-loginset", controller, testutils.NewSssdConfRef("test"))
			ls.Spec.Strategy = s
			return ls
		}

		It("Should admit empty strategy", func(ctx SpecContext) {
			_, err := loginSetWebhook.ValidateCreate(ctx, newWithStrategy(appsv1.DeploymentStrategy{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit RollingUpdate strategy with parameters", func(ctx SpecContext) {
			maxUnavailable := intstr.FromInt(0)
			maxSurge := intstr.FromInt(1)
			ls := newWithStrategy(appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			})
			_, err := loginSetWebhook.ValidateCreate(ctx, ls)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit Recreate strategy", func(ctx SpecContext) {
			_, err := loginSetWebhook.ValidateCreate(ctx, newWithStrategy(appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit OnDelete strategy", func(ctx SpecContext) {
			_, err := loginSetWebhook.ValidateCreate(ctx, newWithStrategy(appsv1.DeploymentStrategy{
				Type: slinkyv1beta1.OnDeleteLoginSetStrategyType,
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should reject unknown strategy.type", func(ctx SpecContext) {
			_, err := loginSetWebhook.ValidateCreate(ctx, newWithStrategy(appsv1.DeploymentStrategy{
				Type: "Bogus",
			}))
			Expect(err).To(HaveOccurred())
		})

		It("Should reject Recreate combined with rollingUpdate", func(ctx SpecContext) {
			maxUnavailable := intstr.FromInt(1)
			_, err := loginSetWebhook.ValidateCreate(ctx, newWithStrategy(appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
				},
			}))
			Expect(err).To(HaveOccurred())
		})

		It("Should reject OnDelete combined with rollingUpdate", func(ctx SpecContext) {
			maxUnavailable := intstr.FromInt(1)
			_, err := loginSetWebhook.ValidateCreate(ctx, newWithStrategy(appsv1.DeploymentStrategy{
				Type: slinkyv1beta1.OnDeleteLoginSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &maxUnavailable,
				},
			}))
			Expect(err).To(HaveOccurred())
		})
	})
})
