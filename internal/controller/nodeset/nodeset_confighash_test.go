// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package nodeset

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/indexes"
	"github.com/SlinkyProject/slurm-operator/internal/utils/confighash"
)

// newConfigHashNodeSet returns an opted-in NodeSet that mounts the named Secret.
func newConfigHashNodeSet(name, secretName string) *slinkyv1beta1.NodeSet {
	ns := newNodeSet(name, "controller0", 1)
	ns.Annotations = map[string]string{slinkyv1beta1.AnnotationReloadOnChange: "true"}
	ns.Spec.Template.PodSpecWrapper.PodSpec.Volumes = []corev1.Volume{
		{Name: "v", VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: secretName}}},
	}
	return ns
}

func TestApplyConfigHashes(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-secret", Namespace: corev1.NamespaceDefault},
		Data:       map[string][]byte{"k": []byte("v1")},
	}

	nodeset := newConfigHashNodeSet("ns0", "cfg-secret")

	c := indexes.NewFakeClientBuilderWithIndexes(secret).Build()
	r := newNodeSetController(c, clientmap.NewClientMap())

	if err := r.applyConfigHashes(ctx, nodeset); err != nil {
		t.Fatalf("applyConfigHashes returned error: %v", err)
	}

	annKey := confighash.AnnotationKey(confighash.SecretKindPrefix, "cfg-secret")
	got := nodeset.Spec.Template.Metadata.Annotations[annKey]
	if got == "" || got == confighash.MissingSentinel {
		t.Fatalf("expected a real checksum at %q, got %q", annKey, got)
	}

	// configHashesFromTemplate mirrors the stamped annotations, keyed by <kind>/<name>.
	statusKey := indexes.SecretRefPrefix + "cfg-secret"
	hashes := configHashesFromTemplate(nodeset)
	if hashes[statusKey] != got {
		t.Errorf("configHashesFromTemplate[%q] = %q, want %q", statusKey, hashes[statusKey], got)
	}

	// Not opted-in: no annotation added and no tracked hashes.
	plain := newNodeSet("ns1", "controller0", 1)
	plain.Spec.Template.PodSpecWrapper.PodSpec.Volumes = nodeset.Spec.Template.PodSpecWrapper.PodSpec.Volumes
	if err := r.applyConfigHashes(ctx, plain); err != nil {
		t.Fatalf("applyConfigHashes (not opted in) returned error: %v", err)
	}
	if len(plain.Spec.Template.Metadata.Annotations) != 0 {
		t.Errorf("expected no annotations for non-opted-in NodeSet, got %v",
			plain.Spec.Template.Metadata.Annotations)
	}
	if configHashesFromTemplate(plain) != nil {
		t.Errorf("expected nil config hashes for non-opted-in NodeSet")
	}
}

func TestApplyConfigHashesEmitsEventOnStatusChange(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-secret", Namespace: corev1.NamespaceDefault},
		Data:       map[string][]byte{"k": []byte("v2")},
	}
	nodeset := newConfigHashNodeSet("ns0", "cfg-secret")
	statusKey := indexes.SecretRefPrefix + "cfg-secret"

	// status records a different (stale) hash for the same key.
	nodeset.Status.ConfigHashes = map[string]string{statusKey: "stale-hash"}

	c := indexes.NewFakeClientBuilderWithIndexes(secret).Build()
	r := newNodeSetController(c, clientmap.NewClientMap())
	rec := events.NewFakeRecorder(10)
	r.eventRecorder = rec

	if err := r.applyConfigHashes(ctx, nodeset); err != nil {
		t.Fatalf("applyConfigHashes returned error: %v", err)
	}

	select {
	case ev := <-rec.Events:
		if !strings.Contains(ev, ConfigHashChangedReason) || !strings.Contains(ev, "cfg-secret") {
			t.Errorf("unexpected event: %q", ev)
		}
	default:
		t.Fatalf("expected a %s event, got none", ConfigHashChangedReason)
	}
}

func TestApplyConfigHashesNoEventWhenStatusMatches(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-secret", Namespace: corev1.NamespaceDefault},
		Data:       map[string][]byte{"k": []byte("v1")},
	}
	nodeset := newConfigHashNodeSet("ns0", "cfg-secret")

	c := indexes.NewFakeClientBuilderWithIndexes(secret).Build()
	r := newNodeSetController(c, clientmap.NewClientMap())

	// Learn the current hash, then record it in status so the next apply matches.
	if err := r.applyConfigHashes(ctx, nodeset); err != nil {
		t.Fatalf("applyConfigHashes (learn) returned error: %v", err)
	}
	nodeset.Status.ConfigHashes = configHashesFromTemplate(nodeset)

	rec := events.NewFakeRecorder(10)
	r.eventRecorder = rec
	if err := r.applyConfigHashes(ctx, nodeset); err != nil {
		t.Fatalf("applyConfigHashes returned error: %v", err)
	}

	select {
	case ev := <-rec.Events:
		t.Fatalf("expected no event when status matches, got %q", ev)
	default:
	}
}

func TestApplyConfigHashesNoEventOnFirstObservation(t *testing.T) {
	ctx := context.Background()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg-secret", Namespace: corev1.NamespaceDefault},
		Data:       map[string][]byte{"k": []byte("v1")},
	}
	nodeset := newConfigHashNodeSet("ns0", "cfg-secret")

	c := indexes.NewFakeClientBuilderWithIndexes(secret).Build()
	r := newNodeSetController(c, clientmap.NewClientMap())
	rec := events.NewFakeRecorder(10)
	r.eventRecorder = rec

	// status.ConfigHashes is nil: first time we observe the hash, no event.
	if err := r.applyConfigHashes(ctx, nodeset); err != nil {
		t.Fatalf("applyConfigHashes returned error: %v", err)
	}

	select {
	case ev := <-rec.Events:
		t.Fatalf("expected no event on first observation, got %q", ev)
	default:
	}
}
