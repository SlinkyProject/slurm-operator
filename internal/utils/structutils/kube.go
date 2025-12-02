// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package structutils

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/SlinkyProject/slurm-operator/internal/utils/reflectutils"
)

// StrategicMergePatch merges two objects via kubernetes StrategicMergePatch
// after empty fields are pruned.
//
// Empty fields are considered not given and are pruned on the patch object.
// Doing so avoids empty values from overwriting the base object.
func StrategicMergePatch[T any](base, patch *T) *T {
	if base == nil && patch == nil {
		return nil
	}

	baseBytes, err := json.Marshal(base)
	if err != nil {
		panic(err)
	}
	patchBytes, err := cleanAndMarshal(patch)
	if err != nil {
		panic(err)
	}

	out := new(T)
	b, err := strategicpatch.StrategicMergePatch(baseBytes, patchBytes, out)
	if err != nil {
		panic(err)
	}

	if err := json.Unmarshal(b, out); err != nil {
		panic(err)
	}

	return out
}

// cleanAndMarshal will remarshal the object after removing empty fields.
func cleanAndMarshal(obj any) ([]byte, error) {
	tempJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	var m map[string]any
	if err := json.Unmarshal(tempJSON, &m); err != nil {
		return nil, err
	}

	removeEmpty(m)

	out, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// removeEmpty will recursively walk a map, deleting fields if its value is empty.
func removeEmpty(m map[string]any) {
	for k, v := range m {
		if v == nil || reflectutils.IsEmpty(v) {
			delete(m, k)
		} else if subMap, ok := v.(map[string]any); ok {
			removeEmpty(subMap)
			if len(subMap) == 0 {
				delete(m, k)
			}
		}
	}
}
