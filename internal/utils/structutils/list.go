// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package structutils

import (
	"sort"
	"strings"
)

// Convert a list to a referenced list.
func ReferenceList[T any](items []T) []*T {
	list := make([]*T, 0, len(items))
	for _, item := range items {
		list = append(list, &item)
	}
	return list
}

// Convert a referenced list to a list.
func DereferenceList[T any](items []*T) []T {
	list := make([]T, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		list = append(list, *item)
	}
	return list
}

// MergeList concatenates all lists.
func MergeList[T any](items ...[]T) []T {
	cnt := 0
	for _, item := range items {
		cnt += len(item)
	}
	list := make([]T, 0, cnt)
	for _, item := range items {
		list = append(list, item...)
	}
	return list
}

// SortedDedup trims whitespace from each entry, drops empty entries and
// duplicates, and returns the result sorted, normalizing a string set into a
// deterministic, comparable list.
func SortedDedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
