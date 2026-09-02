// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package structutils

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/util/strategicpatch"
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

// removeEmpty will recursively walk a map, deleting fields that carry no
// information: nulls, empty strings, empty objects and empty lists.
//
// Booleans and numbers are deliberately kept, even when they are the zero
// value of their type. In Kubernetes API types, `false` and `0` are explicit
// choices rather than "unset": `allowPrivilegeEscalation: false` is required
// by the restricted Pod Security Standard, `privileged: false` is the only
// way to override a `true` coming from the base spec, and `runAsUser: 0`
// asks for root on purpose. Such fields are pointers with `omitempty` in the
// Go structs, so they only reach this map when the user actually set them —
// dropping them here silently discarded the user's configuration.
func removeEmpty(m map[string]any) {
	for k, v := range m {
		switch value := v.(type) {
		case nil:
			delete(m, k)
		case string:
			if value == "" {
				delete(m, k)
			}
		case []any:
			if len(value) == 0 {
				delete(m, k)
			}
		case map[string]any:
			removeEmpty(value)
			if len(value) == 0 {
				delete(m, k)
			}
		}
	}
}
