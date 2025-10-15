// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package podinfo

import (
	"bytes"
	"encoding/json"

	"k8s.io/utils/ptr"
)

type PodInfo struct {
	Namespace string `json:"namespace"`
	PodName   string `json:"podName"`
	Node      string `json:"node"`
}

func (podInfo *PodInfo) Equal(cmp PodInfo) bool {
	a, _ := json.Marshal(podInfo)
	b, _ := json.Marshal(cmp)
	return bytes.Equal(a, b)
}

func (podInfo *PodInfo) ToString() string {
	b, _ := json.Marshal(podInfo)
	return string(b)
}

func ParseIntoPodInfo(str *string, out *PodInfo) error {
	data := ptr.Deref(str, "")
	return json.Unmarshal([]byte(data), &out)
}
