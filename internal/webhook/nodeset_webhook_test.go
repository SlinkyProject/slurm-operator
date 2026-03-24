// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("NodeSet Webhook", func() {
	Context("When Creating a NodeSet with Validating Webhook", func() {
		It("Should deny if controllerRef.name is empty", func(ctx SpecContext) {
			nodeset := testutils.NewNodeset("test-nodeset", nil, 1)

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny if maxUnavailable is 0", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(intstr.FromInt32(0))

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny if maxUnavailable is 0%", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(intstr.FromString("0%"))

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny if SSH is enabled without sssdConfRef", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.Ssh.Enabled = true

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if all required fields are provided", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Updating a NodeSet with Validating Webhook", func() {
		It("Should reject changes to controllerRef", func(ctx SpecContext) {
			oldController := testutils.NewController("old-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldNodeSet := testutils.NewNodeset("test-nodeset", oldController, 1)

			newController := testutils.NewController("new-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			newNodeSet := testutils.NewNodeset("test-nodeset", newController, 1)

			_, err := nodeSetWebhook.ValidateUpdate(ctx, oldNodeSet, newNodeSet)
			Expect(err).To(HaveOccurred())
		})

		It("Should reject changes to scalingMode", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldNodeSet := testutils.NewNodeset("test-nodeset", controller, 1)
			oldNodeSet.Spec.ScalingMode = slinkyv1beta1.ScalingModeStatefulset

			newNodeSet := testutils.NewNodeset("test-nodeset", controller, 1)
			newNodeSet.Spec.ScalingMode = slinkyv1beta1.ScalingModeDaemonset

			_, err := nodeSetWebhook.ValidateUpdate(ctx, oldNodeSet, newNodeSet)
			Expect(err).To(HaveOccurred())
		})

		It("Should reject changes to volumeClaimTemplates", func(ctx SpecContext) {
			controller := testutils.NewController("some-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldNodeSet := testutils.NewNodeset("test-nodeset", controller, 1)

			newNodeSet := testutils.NewNodeset("test-nodeset", controller, 1)
			newNodeSet.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
				{ObjectMeta: metav1.ObjectMeta{Name: "data"}},
			}

			_, err := nodeSetWebhook.ValidateUpdate(ctx, oldNodeSet, newNodeSet)
			Expect(err).To(HaveOccurred())
		})

		It("Should admit if no immutable fields change", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			oldNodeSet := testutils.NewNodeset("test-nodeset", controller, 1)
			newNodeSet := testutils.NewNodeset("test-nodeset", controller, 2)

			_, err := nodeSetWebhook.ValidateUpdate(ctx, oldNodeSet, newNodeSet)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Deleting a NodeSet with Validating Webhook", func() {
		It("Should admit a Delete", func(ctx SpecContext) {
			nodeset := testutils.NewNodeset("test-nodeset", nil, 1)

			_, err := nodeSetWebhook.ValidateDelete(ctx, nodeset)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
