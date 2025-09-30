// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package v1beta1

// Hub implements conversion.Hub interface.
//
// NOTE: `conversion.Hub` must be implemented on the `+kubebuilder:storageversion`.
func (src *LoginSet) Hub() {}
