// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

//go:build configrollout

// Package configrollout holds a live-cluster verification test for the NodeSet
// config-change rollout feature. It is the Go port of hack/verify-config-rollout.sh.
//
// Unlike the test/e2e suite, this test does NOT provision a cluster: it runs
// against whatever cluster the ambient kubeconfig points at (current context),
// where an operator built from the feature branch must already be deployed. It
// is gated behind the "configrollout" build tag so it never runs as part of the
// normal unit-test or CI build.
//
// Run it explicitly, e.g. against a kind cluster:
//
//	go test -tags configrollout ./test/configrollout/ -v -timeout 5m
//
// Configuration (all optional, via environment variables):
//
//	CONFIG_ROLLOUT_NAMESPACE  NodeSet namespace            (default: slurm)
//	CONFIG_ROLLOUT_NODESET    NodeSet name                 (default: slurm-worker-slinky)
//	CONFIG_ROLLOUT_TARGET     "configmap/NAME"|"secret/NAME" to mutate
//	                          (default: first mounted ConfigMap, else first ref)
//	CONFIG_ROLLOUT_WAIT       max time to wait for the operator to react
//	                          (default: 120s)
//
// The test opts the NodeSet in (if needed), waits for the baseline checksum to
// be recorded in status.configHashes, mutates the target resource, then asserts
// that:
//  1. status.configHashes[<kind>/<name>] changes,
//  2. a new ControllerRevision is created,
//  3. a ConfigHashChanged event is emitted on the NodeSet.
package configrollout

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/indexes"
	"github.com/SlinkyProject/slurm-operator/internal/utils/confighash"
)

// configHashChangedReason mirrors nodeset.ConfigHashChangedReason. It is
// duplicated here to keep this live-cluster test free of a dependency on the
// controller package (which registers flags on import).
const configHashChangedReason = "ConfigHashChanged"

const probeKey = "rollout-probe"

func TestConfigRollout(t *testing.T) {
	namespace := getenv("CONFIG_ROLLOUT_NAMESPACE", "slurm")
	nodesetName := getenv("CONFIG_ROLLOUT_NODESET", "slurm-worker-slinky")
	targetSpec := os.Getenv("CONFIG_ROLLOUT_TARGET")
	wait := getDuration(t, "CONFIG_ROLLOUT_WAIT", 120*time.Second)

	cfg, err := config.GetConfig()
	if err != nil {
		t.Skipf("no kubeconfig/cluster available, skipping live verification: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("register client-go scheme: %v", err)
	}
	if err := slinkyv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("register slinky scheme: %v", err)
	}

	c, err := crclient.New(cfg, crclient.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	ctx := context.Background()
	key := types.NamespacedName{Namespace: namespace, Name: nodesetName}

	t.Logf("== NodeSet config-rollout verification: %s/%s ==", namespace, nodesetName)

	nodeset := &slinkyv1beta1.NodeSet{}
	if err := c.Get(ctx, key, nodeset); err != nil {
		if apierrors.IsNotFound(err) {
			t.Skipf("NodeSet %s not found; skipping (is the Slurm cluster deployed?)", key)
		}
		t.Fatalf("get NodeSet %s: %v", key, err)
	}

	ensureOptedIn(ctx, t, c, key)

	// Re-read after the possible opt-in patch so we discover refs from the
	// current template.
	if err := c.Get(ctx, key, nodeset); err != nil {
		t.Fatalf("re-get NodeSet %s: %v", key, err)
	}

	kind, name := pickTarget(t, nodeset, targetSpec)
	statusKey := statusKeyFor(kind, name)
	t.Logf("target to mutate: %s/%s (status key %q)", kind, name, statusKey)

	// The reconciler records a checksum on first observation WITHOUT emitting an
	// event, so wait for the baseline to be recorded before mutating; otherwise
	// the change after opt-in would be the first observation and emit no event.
	baseHash := waitForStatusHash(ctx, t, c, key, statusKey, wait)
	baseRevs := countRevisions(ctx, t, c, namespace, nodesetName)
	t.Logf("baseline: status hash %q, ControllerRevisions=%d", short(baseHash), baseRevs)

	mutate(ctx, t, c, namespace, kind, name)

	// Poll for the operator's reaction.
	deadline := time.Now().Add(wait)
	var (
		hashChanged, revCreated, eventSeen bool
		newHash                            = baseHash
		newRevs                            = baseRevs
	)
	for time.Now().Before(deadline) {
		newHash = statusHash(ctx, t, c, key, statusKey)
		if newHash != "" && newHash != baseHash {
			hashChanged = true
		}
		newRevs = countRevisions(ctx, t, c, namespace, nodesetName)
		if newRevs > baseRevs {
			revCreated = true
		}
		if configHashEventSeen(ctx, t, c, namespace, nodesetName) {
			eventSeen = true
		}
		if hashChanged && revCreated && eventSeen {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if hashChanged {
		t.Logf("PASS: status config hash changed -> %s", short(newHash))
	} else {
		t.Errorf("status config hash did NOT change (still %s)", short(baseHash))
	}
	if revCreated {
		t.Logf("PASS: new ControllerRevision created (%d -> %d)", baseRevs, newRevs)
	} else {
		t.Errorf("no new ControllerRevision created (%d -> %d)", baseRevs, newRevs)
	}
	if eventSeen {
		t.Logf("PASS: %s event emitted on %s", configHashChangedReason, nodesetName)
	} else {
		t.Errorf("no %s event emitted on %s", configHashChangedReason, nodesetName)
	}
}

// ensureOptedIn sets the reload-on-change annotation to "true" when missing.
func ensureOptedIn(ctx context.Context, t *testing.T, c crclient.Client, key types.NamespacedName) {
	t.Helper()
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		ns := &slinkyv1beta1.NodeSet{}
		if err := c.Get(ctx, key, ns); err != nil {
			return err
		}
		if ns.Annotations[slinkyv1beta1.AnnotationReloadOnChange] == "true" {
			return nil
		}
		if ns.Annotations == nil {
			ns.Annotations = map[string]string{}
		}
		ns.Annotations[slinkyv1beta1.AnnotationReloadOnChange] = "true"
		t.Logf("opting NodeSet in: setting %s=true", slinkyv1beta1.AnnotationReloadOnChange)
		return c.Update(ctx, ns)
	})
	if err != nil {
		t.Fatalf("ensure opt-in: %v", err)
	}
}

// pickTarget resolves the resource to mutate. When targetSpec is empty it
// prefers the first mounted ConfigMap, falling back to the first Secret.
func pickTarget(t *testing.T, nodeset *slinkyv1beta1.NodeSet, targetSpec string) (kind, name string) {
	t.Helper()
	secrets, configMaps := confighash.References(nodeset.Spec.Template.PodSpecWrapper.PodSpec)
	if len(secrets) == 0 && len(configMaps) == 0 {
		t.Fatalf("no mounted Secrets/ConfigMaps discovered in the pod template")
	}

	if targetSpec != "" {
		k, n, ok := strings.Cut(targetSpec, "/")
		if !ok || n == "" || (k != "configmap" && k != "secret") {
			t.Fatalf("invalid CONFIG_ROLLOUT_TARGET %q; want configmap/NAME or secret/NAME", targetSpec)
		}
		if k == "configmap" && !contains(configMaps, n) {
			t.Fatalf("target configmap/%s is not mounted by the NodeSet (mounted: %v)", n, configMaps)
		}
		if k == "secret" && !contains(secrets, n) {
			t.Fatalf("target secret/%s is not mounted by the NodeSet (mounted: %v)", n, secrets)
		}
		return k, n
	}

	if len(configMaps) > 0 {
		return "configmap", configMaps[0]
	}
	return "secret", secrets[0]
}

// statusKeyFor returns the status.configHashes key for a target, matching the
// reconciler's "<kind>/<name>" format.
func statusKeyFor(kind, name string) string {
	if kind == "configmap" {
		return indexes.ConfigMapRefPrefix + name
	}
	return indexes.SecretRefPrefix + name
}

// mutate adds/updates a harmless probe key on the target resource.
func mutate(ctx context.Context, t *testing.T, c crclient.Client, namespace, kind, name string) {
	t.Helper()
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	t.Logf("mutating %s/%s (setting %s=%s)", kind, name, probeKey, stamp)

	key := types.NamespacedName{Namespace: namespace, Name: name}
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if kind == "configmap" {
			obj := &corev1.ConfigMap{}
			if err := c.Get(ctx, key, obj); err != nil {
				return err
			}
			if obj.Data == nil {
				obj.Data = map[string]string{}
			}
			obj.Data[probeKey] = stamp
			return c.Update(ctx, obj)
		}
		obj := &corev1.Secret{}
		if err := c.Get(ctx, key, obj); err != nil {
			return err
		}
		if obj.Data == nil {
			obj.Data = map[string][]byte{}
		}
		obj.Data[probeKey] = []byte(stamp)
		return c.Update(ctx, obj)
	})
	if err != nil {
		t.Fatalf("mutate %s/%s: %v", kind, name, err)
	}
}

// waitForStatusHash blocks until status.configHashes[statusKey] is populated
// (non-empty), returning the observed value.
func waitForStatusHash(ctx context.Context, t *testing.T, c crclient.Client, key types.NamespacedName, statusKey string, wait time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(wait)
	for {
		if h := statusHash(ctx, t, c, key, statusKey); h != "" {
			return h
		}
		if !time.Now().Before(deadline) {
			t.Fatalf("status.configHashes[%q] was not recorded within %s; is the operator running the feature build?", statusKey, wait)
		}
		time.Sleep(3 * time.Second)
	}
}

// statusHash returns the current status.configHashes[statusKey] value ("" if absent).
func statusHash(ctx context.Context, t *testing.T, c crclient.Client, key types.NamespacedName, statusKey string) string {
	t.Helper()
	ns := &slinkyv1beta1.NodeSet{}
	if err := c.Get(ctx, key, ns); err != nil {
		t.Fatalf("get NodeSet %s: %v", key, err)
	}
	return ns.Status.ConfigHashes[statusKey]
}

// countRevisions counts ControllerRevisions owned by the NodeSet.
func countRevisions(ctx context.Context, t *testing.T, c crclient.Client, namespace, nodesetName string) int {
	t.Helper()
	list := &appsv1.ControllerRevisionList{}
	if err := c.List(ctx, list, crclient.InNamespace(namespace)); err != nil {
		t.Fatalf("list ControllerRevisions: %v", err)
	}
	count := 0
	for i := range list.Items {
		for _, ref := range list.Items[i].OwnerReferences {
			if ref.Name == nodesetName && ref.Kind == "NodeSet" {
				count++
				break
			}
		}
	}
	return count
}

// configHashEventSeen reports whether a ConfigHashChanged event regarding the
// NodeSet exists.
func configHashEventSeen(ctx context.Context, t *testing.T, c crclient.Client, namespace, nodesetName string) bool {
	t.Helper()
	list := &eventsv1.EventList{}
	if err := c.List(ctx, list, crclient.InNamespace(namespace)); err != nil {
		t.Fatalf("list events: %v", err)
	}
	for i := range list.Items {
		e := &list.Items[i]
		if e.Reason == configHashChangedReason && e.Regarding.Name == nodesetName {
			return true
		}
	}
	return false
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func short(s string) string {
	if s == "" {
		return "<none>"
	}
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(t *testing.T, key string, def time.Duration) time.Duration {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		t.Fatalf("invalid %s=%q: %v", key, v, err)
	}
	return d
}
