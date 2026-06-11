// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package confighash

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReferences(t *testing.T) {
	spec := corev1.PodSpec{
		Volumes: []corev1.Volume{
			{Name: "v1", VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: "vol-secret"}}},
			{Name: "v2", VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "vol-cm"}}}},
			{Name: "v3", VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{Sources: []corev1.VolumeProjection{
					{Secret: &corev1.SecretProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: "proj-secret"}}},
					{ConfigMap: &corev1.ConfigMapProjection{
						LocalObjectReference: corev1.LocalObjectReference{Name: "proj-cm"}}},
					{DownwardAPI: &corev1.DownwardAPIProjection{}},
				}}}},
			{Name: "v4", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
		},
		InitContainers: []corev1.Container{
			{Name: "init", EnvFrom: []corev1.EnvFromSource{
				{SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "envfrom-secret"}}},
			}},
		},
		Containers: []corev1.Container{
			{Name: "c1",
				EnvFrom: []corev1.EnvFromSource{
					{ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "envfrom-cm"}}},
				},
				Env: []corev1.EnvVar{
					{Name: "A", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "env-secret"}}}},
					{Name: "B", ValueFrom: &corev1.EnvVarSource{
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "env-cm"}}}},
					{Name: "C", Value: "plain"},
					// duplicate reference should be deduped
					{Name: "D", ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "vol-secret"}}}},
				}},
		},
	}

	wantSecrets := []string{"env-secret", "envfrom-secret", "proj-secret", "vol-secret"}
	wantConfigMaps := []string{"env-cm", "envfrom-cm", "proj-cm", "vol-cm"}

	gotSecrets, gotConfigMaps := References(spec)
	if diff := cmp.Diff(wantSecrets, gotSecrets); diff != "" {
		t.Errorf("secrets mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantConfigMaps, gotConfigMaps); diff != "" {
		t.Errorf("configMaps mismatch (-want +got):\n%s", diff)
	}
}

func TestReferencesEmpty(t *testing.T) {
	secrets, configMaps := References(corev1.PodSpec{})
	if len(secrets) != 0 || len(configMaps) != 0 {
		t.Errorf("expected no references, got secrets=%v configMaps=%v", secrets, configMaps)
	}
}

func TestAnnotationKey(t *testing.T) {
	const slinkyPrefix = "slinky.slurm.net/"

	// short name: built verbatim
	gotShort := AnnotationKey(SecretKindPrefix, "my-secret")
	wantShort := slinkyPrefix + "secret-my-secret-hash"
	if gotShort != wantShort {
		t.Errorf("short key = %q, want %q", gotShort, wantShort)
	}

	gotCM := AnnotationKey(ConfigMapKindPrefix, "my-cm")
	wantCM := slinkyPrefix + "configmap-my-cm-hash"
	if gotCM != wantCM {
		t.Errorf("configmap key = %q, want %q", gotCM, wantCM)
	}

	// long name: name segment (after the '/') must stay within 63 chars
	longName := ""
	for i := 0; i < 80; i++ {
		longName += "a"
	}
	gotLong := AnnotationKey(SecretKindPrefix, longName)
	segment := gotLong[len(slinkyPrefix):]
	if len(segment) > 63 {
		t.Errorf("long key segment length = %d, want <= 63 (%q)", len(segment), segment)
	}
	// distinct long names must produce distinct keys (disambiguator)
	other := longName[:79] + "b"
	if AnnotationKey(SecretKindPrefix, other) == gotLong {
		t.Errorf("expected distinct keys for distinct long names")
	}
}

func TestSecretAndConfigMapHash(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"},
		Data:       map[string][]byte{"k": []byte("v1")},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Data:       map[string]string{"k": "v1"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, cm).Build()
	ctx := context.Background()

	sHash, err := SecretHash(ctx, c, "ns", "s")
	if err != nil {
		t.Fatal(err)
	}
	cHash, err := ConfigMapHash(ctx, c, "ns", "c")
	if err != nil {
		t.Fatal(err)
	}
	if sHash == "" || cHash == "" {
		t.Fatalf("expected non-empty hashes, got secret=%q configmap=%q", sHash, cHash)
	}

	// Changing data changes the hash.
	secret.Data["k"] = []byte("v2")
	if err := c.Update(ctx, secret); err != nil {
		t.Fatal(err)
	}
	sHash2, err := SecretHash(ctx, c, "ns", "s")
	if err != nil {
		t.Fatal(err)
	}
	if sHash2 == sHash {
		t.Errorf("expected secret hash to change after data update")
	}

	// Missing resource yields the sentinel, not an error.
	missing, err := SecretHash(ctx, c, "ns", "does-not-exist")
	if err != nil {
		t.Fatalf("expected nil error for missing secret, got %v", err)
	}
	if missing != MissingSentinel {
		t.Errorf("missing secret hash = %q, want %q", missing, MissingSentinel)
	}
}

func TestConfigMapHashMergesDataAndBinaryData(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	// "split" shares the key "k" across Data and BinaryData; the "d/"/"b/"
	// prefixes must keep these distinct.
	split := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "split", Namespace: "ns"},
		Data:       map[string]string{"k": "x"},
		BinaryData: map[string][]byte{"k": []byte("y")},
	}
	// "collapsed" stores the same bytes under a single BinaryData key; without
	// the prefix scheme its hash could collide with "split".
	collapsed := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "collapsed", Namespace: "ns"},
		BinaryData: map[string][]byte{"k": []byte("xy")},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(split, collapsed).Build()
	ctx := context.Background()

	splitHash, err := ConfigMapHash(ctx, c, "ns", "split")
	if err != nil {
		t.Fatal(err)
	}
	if splitHash == "" || splitHash == MissingSentinel {
		t.Fatalf("expected non-empty, non-sentinel hash, got %q", splitHash)
	}

	collapsedHash, err := ConfigMapHash(ctx, c, "ns", "collapsed")
	if err != nil {
		t.Fatal(err)
	}
	if splitHash == collapsedHash {
		t.Errorf("expected distinct hashes for merged Data/BinaryData vs collapsed BinaryData, both = %q", splitHash)
	}

	// Changing BinaryData changes the hash (ConfigMap change-detection).
	split.BinaryData["k"] = []byte("z")
	if err := c.Update(ctx, split); err != nil {
		t.Fatal(err)
	}
	splitHash2, err := ConfigMapHash(ctx, c, "ns", "split")
	if err != nil {
		t.Fatal(err)
	}
	if splitHash2 == splitHash {
		t.Errorf("expected configmap hash to change after BinaryData update")
	}
}

func TestConfigMapHashDetectsKeyRename(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cm1 := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Data:       map[string]string{"app.conf": "x"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cm1).Build()
	ctx := context.Background()

	h1, err := ConfigMapHash(ctx, c, "ns", "c")
	if err != nil {
		t.Fatal(err)
	}

	// Rename the key, keep the value identical.
	cm1.Data = map[string]string{"app.yaml": "x"}
	if err := c.Update(ctx, cm1); err != nil {
		t.Fatal(err)
	}
	h2, err := ConfigMapHash(ctx, c, "ns", "c")
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Errorf("expected hash to change when a key is renamed (h1=%s h2=%s)", h1, h2)
	}
}

func TestSecretHashDetectsCrossKeyRebalance(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"},
		Data:       map[string][]byte{"a": []byte("1"), "b": []byte("23")},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(s).Build()
	ctx := context.Background()

	h1, err := SecretHash(ctx, c, "ns", "s")
	if err != nil {
		t.Fatal(err)
	}

	// Same concatenated value bytes ("123"), different key boundaries.
	s.Data = map[string][]byte{"a": []byte("12"), "b": []byte("3")}
	if err := c.Update(ctx, s); err != nil {
		t.Fatal(err)
	}
	h2, err := SecretHash(ctx, c, "ns", "s")
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h2 {
		t.Errorf("expected hash to change when bytes move across keys (h1=%s h2=%s)", h1, h2)
	}
}
