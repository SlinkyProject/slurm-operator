// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"github.com/SlinkyProject/slurm-operator/internal/utils/testutils"
)

var _ = Describe("NodeSet Webhook", func() {
	Context("When Creating a NodeSet with Validating Webhook", func() {
		It("Should admit if template hostname is a valid generateName-style prefix", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.Template.PodSpecWrapper.Hostname = "foo-"

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deny if template hostname contains a newline", func(ctx SpecContext) {
			controller := testutils.NewController("valid-controller", corev1.SecretKeySelector{}, corev1.SecretKeySelector{}, nil)
			nodeset := testutils.NewNodeset("test-nodeset", controller, 1)
			nodeset.Spec.Template.PodSpecWrapper.Hostname = "evil\nSchedulerParameters=malicious"

			_, err := nodeSetWebhook.ValidateCreate(ctx, nodeset)
			Expect(err).To(HaveOccurred())
		})
	})
})
