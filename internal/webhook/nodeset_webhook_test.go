// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

var _ = Describe("NodeSet Webhook", func() {
	Context("When creating NodeSet under Validating Webhook", func() {
		It("Should deny if a required field is empty", func() {
			// TODO(user): Add your logic here
		})

		It("Should admit if all required fields are provided", func() {
			// TODO(user): Add your logic here
		})
	})

	Context("When creating NodeSet under Mutating Webhook", func() {
		var webhook *NodeSetWebhook

		BeforeEach(func() {
			webhook = &NodeSetWebhook{}
		})

		It("Should default nil replicas to 0", func() {
			nodeset := &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nodeset",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					Replicas: nil, // nil replicas
				},
			}

			err := webhook.Default(context.Background(), nodeset)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeset.Spec.Replicas).NotTo(BeNil())
			Expect(*nodeset.Spec.Replicas).To(Equal(int32(0)))
		})

		It("Should not modify replicas when already set to 0", func() {
			nodeset := &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nodeset",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					Replicas: ptr.To[int32](0),
				},
			}

			err := webhook.Default(context.Background(), nodeset)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeset.Spec.Replicas).NotTo(BeNil())
			Expect(*nodeset.Spec.Replicas).To(Equal(int32(0)))
		})

		It("Should not modify replicas when already set to a positive value", func() {
			nodeset := &slinkyv1beta1.NodeSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nodeset",
					Namespace: "default",
				},
				Spec: slinkyv1beta1.NodeSetSpec{
					Replicas: ptr.To[int32](5),
				},
			}

			err := webhook.Default(context.Background(), nodeset)
			Expect(err).NotTo(HaveOccurred())
			Expect(nodeset.Spec.Replicas).NotTo(BeNil())
			Expect(*nodeset.Spec.Replicas).To(Equal(int32(5)))
		})
	})
})
