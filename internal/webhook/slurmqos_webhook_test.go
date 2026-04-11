// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
)

func newSlurmQos(name string, controllerRefName string) *slinkyv1beta1.SlurmQos {
	qos := &slinkyv1beta1.SlurmQos{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: slinkyv1beta1.SlurmQosSpec{
			ControllerRef: slinkyv1beta1.ObjectReference{
				Name:      controllerRefName,
				Namespace: "default",
			},
		},
	}
	return qos
}

func newSlurmQosWithConfig(name, controllerRefName string, config map[string]interface{}) *slinkyv1beta1.SlurmQos {
	qos := newSlurmQos(name, controllerRefName)
	if config != nil {
		raw, _ := json.Marshal(config)
		qos.Spec.Config = &runtime.RawExtension{Raw: raw}
	}
	return qos
}

var _ = Describe("SlurmQos Webhook", func() {
	// Use a webhook instance without client for field-level validation tests
	noClientWebhook := &SlurmQosWebhook{}

	Context("When validating SlurmQos fields", func() {
		It("Should deny if controllerRef.name is empty", func(ctx SpecContext) {
			qos := newSlurmQos("normal", "")

			_, err := noClientWebhook.ValidateCreate(ctx, qos)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("controllerRef.name must not be empty"))
		})

		It("Should deny if name contains invalid characters", func(ctx SpecContext) {
			qos := newSlurmQos("invalid name!", "my-controller")

			_, err := noClientWebhook.ValidateCreate(ctx, qos)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must contain only alphanumeric"))
		})

		It("Should deny if name exceeds 255 characters", func(ctx SpecContext) {
			longName := ""
			for i := 0; i < 256; i++ {
				longName += "a"
			}
			qos := newSlurmQos(longName, "my-controller")

			_, err := noClientWebhook.ValidateCreate(ctx, qos)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exceeds 255 characters"))
		})

		It("Should deny if config is invalid JSON", func(ctx SpecContext) {
			qos := newSlurmQos("normal", "my-controller")
			qos.Spec.Config = &runtime.RawExtension{Raw: []byte("not valid json")}

			_, err := noClientWebhook.ValidateCreate(ctx, qos)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("config must be valid JSON"))
		})

		It("Should admit with valid fields and no client", func(ctx SpecContext) {
			config := map[string]interface{}{
				"priority": map[string]interface{}{"set": true, "number": 500},
			}
			qos := newSlurmQosWithConfig("normal", "my-controller", config)

			_, err := noClientWebhook.ValidateCreate(ctx, qos)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should warn when creating QOS named 'normal'", func(ctx SpecContext) {
			qos := newSlurmQos("normal", "my-controller")

			warns, err := noClientWebhook.ValidateCreate(ctx, qos)
			Expect(err).NotTo(HaveOccurred())
			Expect(warns).To(HaveLen(1))
			Expect(warns[0]).To(ContainSubstring("normal"))
		})

		It("Should allow names with hyphens and underscores", func(ctx SpecContext) {
			qos := newSlurmQos("cpu-normal_v2", "my-controller")

			_, err := noClientWebhook.ValidateCreate(ctx, qos)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Updating a SlurmQos with Validating Webhook", func() {
		It("Should reject changes to controllerRef", func(ctx SpecContext) {
			oldQos := newSlurmQos("normal", "old-controller")
			newQos := newSlurmQos("normal", "new-controller")

			_, err := noClientWebhook.ValidateUpdate(ctx, oldQos, newQos)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot change controllerRef"))
		})

		It("Should admit if no immutable fields change", func(ctx SpecContext) {
			oldQos := newSlurmQos("normal", "my-controller")
			newQos := newSlurmQos("normal", "my-controller")
			newQos.Spec.Description = "updated description"

			_, err := noClientWebhook.ValidateUpdate(ctx, oldQos, newQos)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When Deleting a SlurmQos with Validating Webhook", func() {
		It("Should admit a Delete", func(ctx SpecContext) {
			qos := newSlurmQos("normal", "my-controller")

			_, err := noClientWebhook.ValidateDelete(ctx, qos)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
