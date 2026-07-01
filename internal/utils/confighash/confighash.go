// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package confighash

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/set"
	"sigs.k8s.io/controller-runtime/pkg/client"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/utils/crypto"
)

const (
	// SecretKindPrefix and ConfigMapKindPrefix are the kind segments used in
	// per-resource checksum annotation keys.
	SecretKindPrefix    = "secret-"
	ConfigMapKindPrefix = "configmap-"

	hashSuffix = "-hash"

	// maxNameSegment is the maximum length of an annotation key name segment
	// (the part after the '/').
	maxNameSegment = 63

	// disambiguatorLen is the number of hex chars from the name checksum used
	// to keep truncated keys unique.
	disambiguatorLen = 8
)

// AnnotationKey builds a stable, K8s-valid annotation key for a tracked
// resource of the given kind. When the natural name segment would exceed the
// 63-char limit, the name is truncated and a short checksum of the full name is
// appended so distinct resources keep distinct keys.
func AnnotationKey(kindPrefix, name string) string {
	segment := kindPrefix + name + hashSuffix
	if len(segment) <= maxNameSegment {
		return slinkyv1beta1.SlinkyPrefix + segment
	}

	disambiguator := crypto.CheckSum([]byte(name))[:disambiguatorLen]
	// overhead = kindPrefix + '-' + disambiguator + hashSuffix
	overhead := len(kindPrefix) + 1 + disambiguatorLen + len(hashSuffix)
	keep := maxNameSegment - overhead
	if keep < 0 {
		keep = 0
	}
	truncated := name
	if len(truncated) > keep {
		truncated = truncated[:keep]
	}
	return slinkyv1beta1.SlinkyPrefix + kindPrefix + truncated + "-" + disambiguator + hashSuffix
}

// References returns the sorted, de-duplicated names of Secrets and ConfigMaps
// referenced by spec via volumes (including projected sources), envFrom, and
// env[].valueFrom. Empty names are ignored.
func References(spec corev1.PodSpec) (secrets, configMaps []string) {
	secretSet := set.New[string]()
	configMapSet := set.New[string]()

	for _, v := range spec.Volumes {
		switch {
		case v.Secret != nil:
			secretSet.Insert(v.Secret.SecretName)
		case v.ConfigMap != nil:
			configMapSet.Insert(v.ConfigMap.Name)
		case v.Projected != nil:
			for _, src := range v.Projected.Sources {
				if src.Secret != nil {
					secretSet.Insert(src.Secret.Name)
				}
				if src.ConfigMap != nil {
					configMapSet.Insert(src.ConfigMap.Name)
				}
			}
		}
	}

	containers := make([]corev1.Container, 0, len(spec.Containers)+len(spec.InitContainers))
	containers = append(containers, spec.InitContainers...)
	containers = append(containers, spec.Containers...)
	for _, c := range containers {
		for _, ef := range c.EnvFrom {
			if ef.SecretRef != nil {
				secretSet.Insert(ef.SecretRef.Name)
			}
			if ef.ConfigMapRef != nil {
				configMapSet.Insert(ef.ConfigMapRef.Name)
			}
		}
		for _, e := range c.Env {
			if e.ValueFrom == nil {
				continue
			}
			if e.ValueFrom.SecretKeyRef != nil {
				secretSet.Insert(e.ValueFrom.SecretKeyRef.Name)
			}
			if e.ValueFrom.ConfigMapKeyRef != nil {
				configMapSet.Insert(e.ValueFrom.ConfigMapKeyRef.Name)
			}
		}
	}

	secretSet.Delete("")
	configMapSet.Delete("")

	secrets = secretSet.UnsortedList()
	configMaps = configMapSet.UnsortedList()
	sort.Strings(secrets)
	sort.Strings(configMaps)
	return secrets, configMaps
}

// MissingSentinel is the deterministic hash value used when a referenced
// resource does not exist. It lets the rollout self-heal once the resource
// appears while keeping the revision stable in the meantime.
const MissingSentinel = "notfound"

// contentChecksum returns a deterministic hex sha256 over the map's keys AND
// values. Keys are sorted and both keys and values are length-prefixed so that
// no key rename or cross-key byte rebalancing can collide to the same digest.
func contentChecksum(items map[string][]byte) string {
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := sha256.New()
	var lenBuf [8]byte
	for _, k := range keys {
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(k)))
		_, _ = h.Write(lenBuf[:])
		_, _ = h.Write([]byte(k))

		v := items[k]
		binary.BigEndian.PutUint64(lenBuf[:], uint64(len(v)))
		_, _ = h.Write(lenBuf[:])
		_, _ = h.Write(v)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// SecretHash returns a deterministic checksum of the named Secret's Data, or
// MissingSentinel if the Secret does not exist. Transient read errors are
// returned to the caller.
func SecretHash(ctx context.Context, reader client.Reader, namespace, name string) (string, error) {
	obj := &corev1.Secret{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := reader.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return MissingSentinel, nil
		}
		return "", err
	}
	return contentChecksum(obj.Data), nil
}

// ConfigMapHash returns a deterministic checksum of the named ConfigMap's Data
// and BinaryData, or MissingSentinel if the ConfigMap does not exist. Transient
// read errors are returned to the caller.
func ConfigMapHash(ctx context.Context, reader client.Reader, namespace, name string) (string, error) {
	obj := &corev1.ConfigMap{}
	key := types.NamespacedName{Namespace: namespace, Name: name}
	if err := reader.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return MissingSentinel, nil
		}
		return "", err
	}
	combined := make(map[string][]byte, len(obj.Data)+len(obj.BinaryData))
	for k, v := range obj.Data {
		combined["d/"+k] = []byte(v)
	}
	for k, v := range obj.BinaryData {
		combined["b/"+k] = v
	}
	return contentChecksum(combined), nil
}
