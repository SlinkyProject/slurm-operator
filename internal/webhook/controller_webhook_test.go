// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Controller Webhook", func() {
	Context("When creating Controller under Validating Webhook", func() {
		It("Should deny if a required field is empty", func() {
			// TODO(user): Add your logic here
		})

		It("Should admit if all required fields are provided", func() {
			// TODO(user): Add your logic here
		})
	})
})
