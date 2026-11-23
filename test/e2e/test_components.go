// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/pkg/types"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	clienttoken "github.com/SlinkyProject/slurm-client/pkg/client/token"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/token/slurmjwt"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	"github.com/SlinkyProject/slurm-operator/test"
)

// Dependency Component Health Checks

func checkMariaDBHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) context.Context {
	t.Helper()

	// Get MariaDB CR

	mariadb := &mariadbv1alpha1.MariaDB{}

	mariadbKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "mariadb",
	}

	err := crClient.Get(ctx, mariadbKey, mariadb)
	require.NoError(t, err, "failed to Get() mariadb using controller-runtime client")

	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "mariadb",
		},
	}
	err = wait.For(
		conditions.New(config.Client().Resources()).ResourceScaled(statefulSet, func(object k8s.Object) int32 {
			return object.(*appsv1.StatefulSet).Status.ReadyReplicas
		}, 1),
		wait.WithContext(ctx),
		wait.WithTimeout(10*time.Minute),
		wait.WithInterval(5*time.Second),
		wait.WithImmediate(),
	)
	require.NoError(
		t,
		err,
		"timed out waiting for StatefulSet %s/%s to reach one ready replica; observed status: %s",
		statefulSet.Namespace,
		statefulSet.Name,
		test.StatusJSON(statefulSet.Status),
	)

	return ctx
}

// Slinky Component Health Checks

// Controller tests

func checkControllerHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) {
	t.Helper()

	// Get Controller CR
	controller := &slinkyv1beta1.Controller{}

	controllerKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, controllerKey, controller)
	require.NoError(t, err, "failed to Get() controller using controller-runtime client")

	controllerUID := controller.UID

	// Get Controller StatefulSet using controller CR
	statefulSetKey := controller.Key()
	statefulSet := &appsv1.StatefulSet{}
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.NoError(
			collect,
			crClient.Get(ctx, statefulSetKey, statefulSet),
			"failed to Get() statefulset using controller-runtime client",
		)
	}, 30*time.Second, time.Second, "timed out waiting for controller StatefulSet %s", statefulSetKey)

	// Confirm ownership of controller statefulset
	for _, owner := range statefulSet.OwnerReferences {
		require.Equal(t, controllerUID, owner.UID, "dubious ownership of statefulset: %v", statefulSet)
	}

	// Wait for controller statefulset to become ready
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulSet, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, *statefulSet.Spec.Replicas))
	if err != nil {
		_ = crClient.Get(ctx, controllerKey, controller)
		t.Fatalf(
			"timed out waiting for controller StatefulSet %s/%s to reach %d ready replicas: %v; controller status: %s; StatefulSet status: %s",
			statefulSet.Namespace,
			statefulSet.Name,
			*statefulSet.Spec.Replicas,
			err,
			test.StatusJSON(controller.Status),
			test.StatusJSON(statefulSet.Status),
		)
	}
}

func testSlurmController(namespace string) types.Feature {
	return features.New("Assess the functionality of the Slurm controller").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("slurmctld is responsive", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scontrol", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("slurm controller can resolve nodeset by hostname", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			checkHostnameResolution(ctx, t, namespace, "slurm-controller-0", "slinky-0")

			return ctx
		}).
		Assess("job launch & execution succeeds (srun)", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "srun", "--immediate=10", "-K", "-Q", "--time=0:15", "hostname"}
			wants := "slinky-0"

			cleanup_command := "kubectl"
			cleanup_args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("controller retains jobs when its pod is recreated", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err)
			podKey := crclient.ObjectKey{Namespace: namespace, Name: "slurm-controller-0"}
			before := &corev1.Pod{}
			require.NoError(t, crClient.Get(ctx, podKey, before))

			output, err := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podKey.Name, "-c", "slurmctld", "--", //nolint:gosec // Test-only code
				"sbatch", "--parsable", "--hold", "--job-name=e2e-persistence", "--wrap=true").CombinedOutput()
			require.NoError(t, err, "failed to submit held job: %s", output)
			jobID := strings.TrimSpace(string(output))
			_, err = strconv.ParseUint(jobID, 10, 64)
			require.NoError(t, err, "invalid job ID: %q", jobID)

			require.NoError(t, crClient.Delete(ctx, before))
			require.Eventually(t, func() bool {
				after := &corev1.Pod{}
				return crClient.Get(ctx, podKey, after) == nil && after.UID != before.UID &&
					after.DeletionTimestamp == nil && podReady(after)
			}, 3*time.Minute, 2*time.Second, "controller pod was not recreated")

			output, err = exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podKey.Name, "-c", "slurmctld", "--", //nolint:gosec // Test-only code
				"scontrol", "show", "job", jobID, "--oneliner").CombinedOutput()
			require.NoError(t, err, "controller lost job %s after pod recreation: %s", jobID, output)
			require.Contains(t, strings.Fields(string(output)), "JobName=e2e-persistence")
			require.Contains(t, strings.Fields(string(output)), "JobState=PENDING")
			output, err = exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podKey.Name, "-c", "slurmctld", "--", //nolint:gosec // Test-only code
				"scancel", jobID).CombinedOutput()
			require.NoError(t, err, "failed to cancel held job %s: %s", jobID, output)
			return ctx
		}).
		Assess("slurmctld is responsive after restart", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scontrol", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("slurm controller can resolve nodeset by hostname after restart", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			checkHostnameResolution(ctx, t, namespace, "slurm-controller-0", "slinky-0")

			return ctx
		}).
		Assess("job launch & execution succeeds (srun) after restart", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "srun", "--immediate=10", "-K", "-Q", "--time=0:15", "hostname"}
			wants := "slinky-0"

			cleanup_command := "kubectl"
			cleanup_args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).Feature()
}

func testSlurmHA(namespace string) types.Feature {
	const primary = "slurm-controller-0"
	const backup = "slurm-controller-1"
	var primaryStopped bool
	var jobID, originalJobID string
	jobName := fmt.Sprintf("ha-failover-%d", time.Now().UnixNano())

	// Restrict each client to the daemon in the selected pod. A responding
	// standby, or a client falling back to another host, does not prove takeover.
	run := func(ctx context.Context, config *envconf.Config, pod string, command ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		args := []string{"--kubeconfig", config.KubeconfigFile(), "--context", config.KubeContext(),
			"exec", "-n", namespace, pod, "-c", "slurmctld", "--", "sh", "-ec", `
conf=$(mktemp)
trap 'rm -f "$conf"' EXIT
sed '/^SlurmctldHost=/d; /^SlurmctldAddr=/d' /etc/slurm/slurm.conf > "$conf"
printf '\nSlurmctldHost=%s(127.0.0.1)\n' "$(hostname -s)" >> "$conf"
export SLURM_CONF="$conf"
timeout 15s "$@"
`, "ha-check"}
		return exec.CommandContext(ctx, "kubectl", append(args, command...)...).CombinedOutput() //nolint:gosec // Test-only code
	}
	waitActive := func(ctx context.Context, t *testing.T, config *envconf.Config, pod string) {
		t.Helper()
		var output []byte
		var commandErr error
		err := wait.For(func(ctx context.Context) (bool, error) {
			output, commandErr = run(ctx, config, pod, "scontrol", "show", "config")
			return commandErr == nil, nil
		}, wait.WithContext(ctx), wait.WithTimeout(4*time.Minute), wait.WithInterval(3*time.Second), wait.WithImmediate())
		if err != nil {
			t.Fatalf("controller %s/%s did not become active: %v; last command error: %v; output: %s", namespace, pod, err, commandErr, output)
		}
	}

	return features.New("Assess Slurm controller HA failover").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err)
			controllerKey := crclient.ObjectKey{Namespace: namespace, Name: "slurm"}
			controller := &slinkyv1beta1.Controller{}
			require.NoError(t, crClient.Get(ctx, controllerKey, controller))
			require.True(t, controller.Spec.HighAvailability.Enabled, "HA must be enabled")
			originalProbe := controller.Spec.Slurmctld.LivenessProbe.DeepCopy()

			t.Cleanup(func() {
				cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
				defer cancel()
				if primaryStopped {
					output, err := run(cleanupCtx, config, primary, "sh", "-ec", `kill -CONT $(pgrep -x slurmctld)`)
					if err != nil {
						t.Errorf("failed to resume HA primary: %v; output: %s", err, output)
					}
				}
				if jobID != "" {
					for _, name := range []string{jobName, jobName + "-primary", jobName + "-before"} {
						output, err := run(cleanupCtx, config, primary, "scancel", "--name="+name)
						if err != nil {
							t.Errorf("failed to clean up HA job %s: %v; output: %s", name, err, output)
						}
					}
				}
				current := &slinkyv1beta1.Controller{}
				if err := crClient.Get(cleanupCtx, controllerKey, current); err != nil {
					t.Errorf("failed to get HA Controller for probe restoration: %v", err)
					return
				}
				base := current.DeepCopy()
				current.Spec.Slurmctld.LivenessProbe = originalProbe
				if err := crClient.Patch(cleanupCtx, current, crclient.MergeFrom(base)); err != nil {
					t.Errorf("failed to restore HA liveness probe: %v", err)
				}
			})

			// Default liveness restarts a hung daemon before Slurm's 120s
			// takeover timeout. Keep it stopped long enough to exercise HA.
			pod := &corev1.Pod{}
			require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: primary}, pod))
			base := controller.DeepCopy()
			for _, container := range pod.Spec.Containers {
				if container.Name == "slurmctld" {
					controller.Spec.Slurmctld.LivenessProbe = container.LivenessProbe.DeepCopy()
				}
			}
			require.NotNil(t, controller.Spec.Slurmctld.LivenessProbe)
			controller.Spec.Slurmctld.LivenessProbe.FailureThreshold = 60
			require.NoError(t, crClient.Patch(ctx, controller, crclient.MergeFrom(base)))
			err = wait.For(func(ctx context.Context) (bool, error) {
				for _, name := range []string{primary, backup} {
					current := &corev1.Pod{}
					if err := crClient.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: name}, current); err != nil || !podReady(current) || current.DeletionTimestamp != nil {
						return false, nil //nolint:nilerr
					}
					for _, container := range current.Spec.Containers {
						if container.Name == "slurmctld" && (container.LivenessProbe == nil || container.LivenessProbe.FailureThreshold != 60) {
							return false, nil
						}
					}
				}
				return true, nil
			}, wait.WithContext(ctx), wait.WithTimeout(3*time.Minute), wait.WithInterval(2*time.Second), wait.WithImmediate())
			require.NoError(t, err, "HA controller pods did not adopt the temporary liveness probe")
			waitActive(ctx, t, config, primary)
			output, err := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, "slurm-worker-slinky-0", "-c", "slurmd", "--", //nolint:gosec // Test-only code
				"sbatch", "--parsable", "--hold", "--job-name="+jobName+"-before", "--wrap=true").CombinedOutput()
			require.NoError(t, err, "worker could not submit a job before failover: %s", output)
			originalJobID = strings.TrimSpace(string(output))
			_, err = strconv.ParseUint(originalJobID, 10, 64)
			require.NoError(t, err, "invalid job ID before failover: %q", originalJobID)
			// Slurm checkpoints asynchronously; suspend only after this job is saved.
			require.Eventually(t, func() bool {
				_, err := run(ctx, config, primary, "sh", "-ec", `grep -aFq -- "$1" /var/spool/slurmctld/*/job_state`, "checkpoint", jobName+"-before")
				return err == nil
			}, 30*time.Second, time.Second, "primary did not checkpoint the job before failover")
			return ctx
		}).
		Assess("slurmctld is responsive", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scontrol", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("both controller replicas are ready", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			controllerKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-controller",
			}

			checkStatefulSetReplicas(crClient, ctx, t, controllerKey)

			return ctx
		}).
		Assess("backup eventually takes over when the primary stops responding", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			primaryStopped = true
			output, err := run(ctx, config, primary, "sh", "-ec", `kill -STOP $(pgrep -x slurmctld)`)
			require.NoError(t, err, "failed to suspend the primary: %s", output)
			started := time.Now()
			waitActive(ctx, t, config, backup)
			t.Logf("backup took over after %s", time.Since(started).Round(time.Millisecond))
			return ctx
		}).
		Assess("normal clients can submit and execute jobs after failover", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err)
			require.NoError(t, discoveryv1.AddToScheme(crClient.Scheme()))
			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				slices := &discoveryv1.EndpointSliceList{}
				if !assert.NoError(collect, crClient.List(ctx, slices, crclient.InNamespace(namespace),
					crclient.MatchingLabels{discoveryv1.LabelServiceName: "slurm-controller"})) {
					return
				}
				ready := 0
				for _, slice := range slices.Items {
					for _, endpoint := range slice.Endpoints {
						if !ptr.Deref(endpoint.Conditions.Ready, false) {
							continue
						}
						ready++
						if assert.NotNil(collect, endpoint.TargetRef) {
							assert.Equal(collect, backup, endpoint.TargetRef.Name, "Service must route to the active backup")
						}
					}
				}
				assert.Positive(collect, ready, "controller Service has no ready endpoints")
			}, time.Minute, 2*time.Second, "controller Service did not follow HA takeover")

			workerArgs := []string{"exec", "-n", namespace, "slurm-worker-slinky-0", "-c", "slurmd", "--"}
			output, err := exec.CommandContext(ctx, "kubectl", append(workerArgs, "scontrol", "show", "job", originalJobID, "--oneliner")...).CombinedOutput() //nolint:gosec // Test-only code
			require.NoError(t, err, "job %s submitted before failover was lost: %s", originalJobID, output)
			require.Contains(t, strings.Fields(string(output)), "JobName="+jobName+"-before")
			output, err = exec.CommandContext(ctx, "kubectl", append(workerArgs, "sbatch", "--parsable", "--hold", "--job-name="+jobName, "--wrap=true")...).CombinedOutput() //nolint:gosec // Test-only code
			require.NoError(t, err, "worker could not submit a job after failover: %s", output)
			jobID = strings.TrimSpace(string(output))
			_, err = strconv.ParseUint(jobID, 10, 64)
			require.NoError(t, err, "sbatch returned an invalid job ID: %q", jobID)
			test.WaitForCommand(ctx, t, "kubectl", append(workerArgs, "srun", "--immediate=10", "--time=0:01", "hostname"),
				"slinky-0", "", nil, time.Minute, 2*time.Second)

			controller := &slinkyv1beta1.Controller{}
			require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: "slurm"}, controller))
			keyRef := controller.AuthJwtRef()
			secret := &corev1.Secret{}
			require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: keyRef.Name}, secret))
			restClient := newSlurmRESTClient(t, config, namespace, secret.Data[keyRef.Key])
			require.EventuallyWithT(t, func(collect *assert.CollectT) {
				pings := &slurmapi.V0044ControllerPingObjectList{}
				if !assert.NoError(collect, restClient.List(ctx, pings, &slurmclient.ListOptions{SkipCache: true})) {
					return
				}
				responding := false
				for _, ping := range pings.Items {
					if ptr.Deref(ping.Hostname, "") == backup && ping.Responding {
						responding = true
					}
				}
				assert.True(collect, responding, "REST API could not reach the active backup: %+v", pings.Items)
			}, time.Minute, 2*time.Second, "REST API did not follow HA takeover")
			return ctx
		}).
		Assess("primary restarts and retains jobs submitted during failover", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			// Finish the backup's checkpoint before the primary loads it.
			output, err := run(ctx, config, backup, "scontrol", "shutdown", "controller")
			require.NoError(t, err, "failed to shut down the backup: %s", output)
			err = wait.For(func(ctx context.Context) (bool, error) {
				output, err = run(ctx, config, backup, "sh", "-ec", "! pgrep -x slurmctld")
				return err == nil, nil
			}, wait.WithContext(ctx), wait.WithTimeout(2*time.Minute), wait.WithInterval(3*time.Second), wait.WithImmediate())
			require.NoError(t, err, "backup did not finish shutting down: %s", output)

			// Discard the suspended primary's stale memory; Supervisor restarts it.
			output, err = run(ctx, config, primary, "sh", "-ec", `kill -KILL $(pgrep -x slurmctld)`)
			require.NoError(t, err, "failed to restart the primary: %s", output)
			primaryStopped = false
			waitActive(ctx, t, config, primary)
			output, err = run(ctx, config, primary, "scontrol", "show", "job", jobID, "--oneliner")
			require.NoError(t, err, "primary lost job %s accepted by the backup: %s", jobID, output)
			require.Contains(t, strings.Fields(string(output)), "JobName="+jobName, "primary must retain the backup's submitted job")

			output, err = run(ctx, config, primary, "sbatch", "--parsable", "--hold", "--job-name="+jobName+"-primary", "--wrap=true")
			require.NoError(t, err, "primary rejected job submission after handover: %s", output)
			output, err = exec.CommandContext(ctx, "kubectl", "--kubeconfig", config.KubeconfigFile(), "--context", config.KubeContext(), //nolint:gosec // Test-only code
				"delete", "pod", "-n", namespace, backup, "--wait=false").CombinedOutput()
			require.NoError(t, err, "failed to restart the backup pod: %s", output)
			return ctx
		}).Feature()
}

func checkHostnameResolution(ctx context.Context, t *testing.T, namespace, sourcePod, nodeName string) {
	t.Helper()

	const attempts = 16
	var (
		lastAddress string
		lastErr     error
		lastOutput  []byte
	)

	for attempt := range attempts {
		nodeInfo, err := test.GetSlurmNodeInfo(ctx, namespace, nodeName)
		if err != nil {
			lastErr = err
		} else {
			lastAddress = nodeInfo["NodeAddr"]
			if lastAddress == "" {
				lastErr = nil
				lastOutput = nil
			} else {
				args := []string{
					"exec", "-n", namespace, sourcePod, "--",
					"getent", "hosts", lastAddress,
				}
				cmd := exec.CommandContext(ctx, "kubectl", args...)
				lastOutput, lastErr = cmd.CombinedOutput()
				fields := strings.Fields(string(lastOutput))
				if lastErr == nil && len(fields) > 1 && strings.HasPrefix(fields[len(fields)-1], nodeName) {
					return
				}
			}
		}

		if attempt < attempts-1 {
			select {
			case <-ctx.Done():
				t.Fatalf(
					"context ended while resolving Slurm node %q from pod %q: address=%q, last command error=%v, last combined output=%q: %v",
					nodeName,
					sourcePod,
					lastAddress,
					lastErr,
					strings.TrimSpace(string(lastOutput)),
					ctx.Err(),
				)
			case <-time.After(5 * time.Second):
			}
		}
	}

	t.Fatalf(
		"failed to resolve Slurm node %q from pod %q after %d attempts: address=%q, last command error=%v, last combined output=%q",
		nodeName,
		sourcePod,
		attempts,
		lastAddress,
		lastErr,
		strings.TrimSpace(string(lastOutput)),
	)
}

// RestAPI tests

func checkRestAPIHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) {
	t.Helper()

	// Get RestAPI CR
	restapi := &slinkyv1beta1.RestApi{}

	restapiKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, restapiKey, restapi)
	require.NoError(t, err, "failed to Get() restapi using controller-runtime client")

	restapiUID := restapi.UID

	// Get RestAPI Deployment using RestAPI CR
	deploymentKey := restapi.Key()
	deployment := &appsv1.Deployment{}
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.NoError(
			collect,
			crClient.Get(ctx, deploymentKey, deployment),
			"failed to Get() deployment using controller-runtime client",
		)
	}, 30*time.Second, time.Second, "timed out waiting for REST API Deployment %s", deploymentKey)

	// Confirm ownership of RestAPI deployment
	for _, owner := range deployment.OwnerReferences {
		require.Equal(t, restapiUID, owner.UID, "dubious ownership of deployment: %v", deployment)
	}

	// Check whether RestAPI deployment is healthy
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, *deployment.Spec.Replicas))
	if err != nil {
		_ = crClient.Get(ctx, restapiKey, restapi)
		t.Fatalf(
			"timed out waiting for REST API Deployment %s/%s to reach %d ready replicas: %v; REST API status: %s; Deployment status: %s",
			deployment.Namespace,
			deployment.Name,
			*deployment.Spec.Replicas,
			err,
			test.StatusJSON(restapi.Status),
			test.StatusJSON(deployment.Status),
		)
	}
}

func testSlurmRestAPI(namespace string, withAccounting bool) types.Feature {
	return features.New("Assess the functionality of the Slurm RestAPI").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("slurmrestd container args match expectations", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"get", "deployment", "-n", namespace, "slurm-restapi", "-o", `jsonpath="{.spec.template.spec.containers[0].args}"`}
			var wants string
			if withAccounting {
				wants = `"["0.0.0.0:6820"]"`
			} else {
				wants = `"["-s","openapi/slurmctld","0.0.0.0:6820"]"`
			}

			cmd := exec.Command(command, args...)
			output, err := cmd.Output()

			if strings.TrimSpace(string(output)) != wants {
				require.NoError(t, err, "failed running %v %v", command, args)
				require.Equal(t, wants, strings.TrimSpace(string(output)))
			}

			return ctx
		}).Feature()
}

// NodeSet tests

const (
	nodeSetReadyAttempts = 25

	// topologySyncDisabledObservation spans the NodeSet controller's 30-second periodic reconcile.
	topologySyncDisabledObservation = 35 * time.Second
	topologyReadTimeout             = 5 * time.Second
)

func checkNodeSetReplicas(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, nodesetKey crclient.ObjectKey) {
	t.Helper()

	nodeset := &slinkyv1beta1.NodeSet{}
	started := time.Now()

	for retry := range nodeSetReadyAttempts {

		err := crClient.Get(ctx, nodesetKey, nodeset)
		require.NoError(t, err, "failed to Get() NodeSet using controller-runtime client")

		if *nodeset.Spec.Replicas == nodeset.Status.AvailableReplicas {
			break
		}

		if retry == nodeSetReadyAttempts-1 {
			t.Fatalf(
				"timed out after %s waiting for NodeSet %s/%s replicas to become available: spec.replicas=%d; observed status=%s",
				time.Since(started).Round(time.Millisecond),
				nodeset.Namespace,
				nodeset.Name,
				*nodeset.Spec.Replicas,
				test.StatusJSON(nodeset.Status),
			)
		}

		time.Sleep(5 * time.Second)
	}
}

func checkStatefulSetReplicas(
	crClient crclient.Client,
	ctx context.Context,
	t *testing.T,
	statefulSetKey crclient.ObjectKey,
) {
	t.Helper()

	statefulSet := &appsv1.StatefulSet{}
	started := time.Now()

	for retry := range nodeSetReadyAttempts {
		if err := crClient.Get(ctx, statefulSetKey, statefulSet); err != nil {
			t.Fatalf("failed to Get() StatefulSet %s: %v", statefulSetKey, err)
		}

		desired := int32(1)
		if statefulSet.Spec.Replicas != nil {
			desired = *statefulSet.Spec.Replicas
		}

		if statefulSet.Status.ObservedGeneration >= statefulSet.Generation &&
			statefulSet.Status.AvailableReplicas == desired {
			return
		}

		if retry == nodeSetReadyAttempts-1 {
			t.Fatalf(
				"timed out after %s waiting for StatefulSet %s/%s replicas to become available: spec.replicas=%d; observed status=%s",
				time.Since(started).Round(time.Millisecond),
				statefulSet.Namespace,
				statefulSet.Name,
				desired,
				test.StatusJSON(statefulSet.Status),
			)
		}

		time.Sleep(5 * time.Second)
	}
}

func testSlurmNodeSet(namespace string) types.Feature {
	return features.New("Assess the functionality of the Slurm NodeSet").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Nodeset can contact controller", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-worker-slinky-0", "--", "scontrol", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 4*time.Minute, 5*time.Second)

			return ctx
		}).
		Assess("NodeSet is idle", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-worker-slinky-0", "--", "sinfo", "-N", "-n", "slinky-0", "--Format=StateLong", "-h"}
			wants := "idle"

			cleanup_command := "kubectl"
			cleanup_args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"}

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("slurmd spool survives a container restart", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			podName := "slurm-worker-slinky-0"
			marker := "/var/spool/slurmd/e2e-restart-marker"
			restartCount := func() int32 {
				pod := &corev1.Pod{}
				require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: podName}, pod))
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.Name == "slurmd" {
						return cs.RestartCount
					}
				}
				return 0
			}

			out, err := exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "-c", "slurmd", "--", "touch", marker).CombinedOutput()
			require.NoError(t, err, "failed to write spool marker: %s", out)
			before := restartCount()

			// Stopping PID 1 restarts the container but keeps the Pod and its emptyDir volumes.
			_ = exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "-c", "slurmd", "--", "kill", "1").Run() //nolint:errcheck
			require.Eventually(t, func() bool { return restartCount() > before }, 2*time.Minute, 5*time.Second, "slurmd container did not restart")

			test.WaitForCommand(ctx, t, "kubectl", []string{"exec", "-n", namespace, podName, "-c", "slurmd", "--", "test", "-f", marker}, "", "", nil, 2*time.Minute, 5*time.Second)
			test.WaitForCommand(ctx, t, "kubectl", []string{"exec", "-n", namespace, podName, "-c", "slurmd", "--", "sinfo", "-N", "-n", "slinky-0", "--Format=StateLong", "-h"}, "idle", "", nil, 2*time.Minute, 5*time.Second)

			_ = exec.CommandContext(ctx, "kubectl", "exec", "-n", namespace, podName, "-c", "slurmd", "--", "rm", "-f", marker).Run() //nolint:errcheck

			return ctx
		}).
		Assess("NodeSet scale-up functions", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky",
			}
			mutateNodeSet(ctx, t, crClient, nodesetKey, func(nodeset *slinkyv1beta1.NodeSet) {
				nodeset.Spec.Replicas = ptr.To[int32](2)
			})

			checkNodeSetReplicas(crClient, ctx, t, config, nodesetKey)

			return ctx
		}).
		Assess("NodeSets can resolve each other's hostnames", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			checkHostnameResolution(ctx, t, namespace, "slurm-worker-slinky-0", "slinky-1")

			return ctx
		}).
		Assess("NodeSet scale-down functions", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "Failed to get new controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky",
			}
			mutateNodeSet(ctx, t, crClient, nodesetKey, func(nodeset *slinkyv1beta1.NodeSet) {
				nodeset.Spec.Replicas = ptr.To[int32](1)
			})

			checkNodeSetReplicas(crClient, ctx, t, config, nodesetKey)

			return ctx
		}).Feature()
}

func testSlurmNodeSetTopologySync(namespace string) types.Feature {
	return features.New("Assess NodeSet topology annotation synchronization").
		Assess("syncTopology controls topology annotation synchronization", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")

			podKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky-0",
			}
			workerPod := &corev1.Pod{}
			require.NoError(t, crClient.Get(ctx, podKey, workerPod), "failed to get NodeSet Pod")
			require.NotEmpty(t, workerPod.Spec.NodeName, "NodeSet Pod %s/%s has empty spec.nodeName", workerPod.Namespace, workerPod.Name)

			slurmNodeName := workerPod.Labels[slinkyv1beta1.LabelNodeSetPodHostname]
			require.NotEmpty(t, slurmNodeName, "NodeSet Pod %s/%s has no Slurm node name label", workerPod.Namespace, workerPod.Name)

			node := &corev1.Node{}
			nodeKey := crclient.ObjectKey{Name: workerPod.Spec.NodeName}
			require.NoError(t, crClient.Get(ctx, nodeKey, node), "failed to get Kubernetes Node %s", nodeKey.Name)

			var originalSyncTopology *bool
			if nodeset.Spec.SyncTopology != nil {
				originalSyncTopology = ptr.To(*nodeset.Spec.SyncTopology)
			}
			var originalNodeTopology *string
			if topology, ok := node.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec]; ok {
				originalNodeTopology = ptr.To(topology)
			}

			targetTopology := "topo-e2e:b0"
			if ptr.Deref(originalNodeTopology, "") == targetTopology {
				targetTopology = "topo-e2e:b1"
			}

			restored := false
			restore := func(restoreCtx context.Context) error {
				if err := updateNodeTopologyAnnotation(restoreCtx, crClient, nodeKey, originalNodeTopology); err != nil {
					return fmt.Errorf("restore Kubernetes Node topology annotation: %w", err)
				}
				if err := updateNodeSetSyncTopology(restoreCtx, crClient, nodesetKey, originalSyncTopology); err != nil {
					return fmt.Errorf("restore NodeSet topology synchronization: %w", err)
				}
				return nil
			}
			t.Cleanup(func() {
				if restored {
					return
				}
				cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if err := restore(cleanupCtx); err != nil {
					t.Errorf("failed to clean up topology synchronization test: %v", err)
				}
			})

			waitForNodeSetTopology(ctx, t, crClient, podKey, namespace, slurmNodeName, ptr.Deref(originalNodeTopology, ""))

			require.NoError(
				t,
				updateNodeSetSyncTopology(ctx, crClient, nodesetKey, ptr.To(false)),
				"failed to disable NodeSet topology synchronization",
			)
			require.Eventually(t, func() bool {
				current := &slinkyv1beta1.NodeSet{}
				if err := crClient.Get(ctx, nodesetKey, current); err != nil {
					return false
				}
				return current.Spec.SyncTopology != nil &&
					!*current.Spec.SyncTopology &&
					current.Status.ObservedGeneration == current.Generation
			}, time.Minute, time.Second, "timed out waiting for NodeSet to observe syncTopology=false")

			originalPodTopology, originalSlurmTopology, err := readNodeSetTopologies(ctx, crClient, podKey, namespace, slurmNodeName)
			require.NoError(t, err, "failed to read topology before changing the Kubernetes Node annotation")

			require.NoError(
				t,
				updateNodeTopologyAnnotation(ctx, crClient, nodeKey, ptr.To(targetTopology)),
				"failed to update Kubernetes Node topology annotation",
			)
			checkNodeSetTopologyUnchanged(
				ctx,
				t,
				crClient,
				podKey,
				namespace,
				slurmNodeName,
				originalPodTopology,
				originalSlurmTopology,
			)

			require.NoError(
				t,
				updateNodeSetSyncTopology(ctx, crClient, nodesetKey, ptr.To(true)),
				"failed to enable NodeSet topology synchronization",
			)
			waitForNodeSetTopology(ctx, t, crClient, podKey, namespace, slurmNodeName, targetTopology)

			require.NoError(t, restore(ctx), "failed to restore topology synchronization test resources")
			restored = true

			return ctx
		}).Feature()
}

func nodeSetPods(ctx context.Context, crClient crclient.Client, nodeset *slinkyv1beta1.NodeSet) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := crClient.List(ctx, podList, crclient.InNamespace(nodeset.Namespace)); err != nil {
		return nil, err
	}

	pods := make([]corev1.Pod, 0, len(podList.Items))
	for i := range podList.Items {
		pod := &podList.Items[i]
		if metav1.IsControlledBy(pod, nodeset) {
			pods = append(pods, *pod)
		}
	}
	return pods, nil
}

// waitForDaemonSetReplicas waits for a DaemonSet-mode NodeSet and all of its
// pods to converge. Pass a negative expected value to accept the controller's
// non-zero desired count, which is useful during initial installation.
func waitForDaemonSetReplicas(
	crClient crclient.Client,
	ctx context.Context,
	t *testing.T,
	nodesetKey crclient.ObjectKey,
	expected int32,
) (*slinkyv1beta1.NodeSet, []corev1.Pod) {
	t.Helper()

	var (
		nodeset slinkyv1beta1.NodeSet
		pods    []corev1.Pod
	)

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		current := &slinkyv1beta1.NodeSet{}
		if !assert.NoError(
			collect,
			crClient.Get(ctx, nodesetKey, current),
			"failed to get NodeSet",
		) {
			return
		}

		currentPods, err := nodeSetPods(ctx, crClient, current)
		if !assert.NoError(collect, err, "failed to list NodeSet pods") {
			return
		}

		nodeset = *current
		pods = currentPods

		desired := expected
		if desired < 0 {
			desired = current.Status.Desired
			assert.NotZero(collect, desired, "desired replica count is not populated")
		}

		assert.Equal(collect, current.Generation, current.Status.ObservedGeneration)
		assert.Equal(collect, desired, current.Status.Desired)
		assert.Equal(collect, desired, current.Status.Replicas)
		assert.Equal(collect, desired, current.Status.UpdatedReplicas)
		assert.Equal(collect, desired, current.Status.ReadyReplicas)
		assert.Equal(collect, desired, current.Status.AvailableReplicas)
		assert.Len(collect, currentPods, int(desired))

		for i := range currentPods {
			assert.True(collect, podReady(&currentPods[i]), "pod %s is not ready", currentPods[i].Name)
		}
	}, 2*time.Minute, 2*time.Second,
		"DaemonSet-mode NodeSet %s/%s did not converge",
		nodesetKey.Namespace,
		nodesetKey.Name,
	)

	return &nodeset, pods
}

func testSlurmDaemonSet(namespace string) types.Feature {
	nodesetKey := crclient.ObjectKey{Namespace: namespace, Name: "slurm-worker-slinky"}
	var (
		initialDesired       int32
		originalNodeSelector map[string]string
		selectedNode         string
		workerHostname       string
		workerPod            string
	)

	return features.New("Assess DaemonSet scaling of the Slurm NodeSet").
		Assess("DaemonSet creates one ready Slurm worker per eligible Kubernetes node", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			nodeset, pods := waitForDaemonSetReplicas(crClient, ctx, t, nodesetKey, -1)
			require.Equal(t, slinkyv1beta1.ScalingModeDaemonset, nodeset.Spec.ScaleStrategy.Mode)
			require.Greater(t, nodeset.Status.Desired, int32(1), "DaemonSet scaling e2e test requires at least two eligible Kubernetes nodes")
			if nodeset.Spec.Replicas != nil {
				require.NotEqual(t, *nodeset.Spec.Replicas, nodeset.Status.Desired, "DaemonSet desired count must be node-driven, not spec.replicas")
			}

			initialDesired = nodeset.Status.Desired
			originalNodeSelector = make(map[string]string, len(nodeset.Spec.Template.PodSpecWrapper.NodeSelector))
			for key, value := range nodeset.Spec.Template.PodSpecWrapper.NodeSelector {
				originalNodeSelector[key] = value
			}

			seenNodes := make(map[string]struct{}, len(pods))
			for i := range pods {
				pod := &pods[i]
				require.NotEmpty(t, pod.Spec.NodeName, "DaemonSet pod %s was not assigned to a Kubernetes node", pod.Name)
				require.NotEmpty(t, pod.Spec.Hostname, "DaemonSet pod %s has no Slurm hostname", pod.Name)
				require.NotContains(t, seenNodes, pod.Spec.NodeName, "multiple DaemonSet pods were assigned to Kubernetes node %s", pod.Spec.NodeName)
				seenNodes[pod.Spec.NodeName] = struct{}{}
				require.Equal(t, string(slinkyv1beta1.ScalingModeDaemonset), pod.Labels[slinkyv1beta1.LabelNodeSetScalingMode])
				require.Empty(t, pod.Labels[slinkyv1beta1.LabelNodeSetPodIndex], "DaemonSet pod must not have a StatefulSet ordinal")
				require.Equal(t, pod.Spec.Hostname, pod.Labels[slinkyv1beta1.LabelNodeSetPodHostname])
			}

			selectedNode = pods[0].Spec.NodeName
			workerHostname = pods[0].Spec.Hostname
			workerPod = pods[0].Name
			return ctx
		}).
		Assess("DaemonSet worker participates in the Slurm cluster", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			test.WaitForCommand(
				ctx,
				t,
				"kubectl",
				[]string{"exec", "-n", namespace, workerPod, "--", "scontrol", "ping"},
				"",
				"",
				nil,
				80*time.Second,
				5*time.Second,
			)

			test.WaitForCommand(
				ctx,
				t,
				"kubectl",
				[]string{"exec", "-n", namespace, workerPod, "--", "sinfo", "-N", "-n", workerHostname, "--Format=StateLong", "-h"},
				"idle",
				"kubectl",
				[]string{"exec", "-n", namespace, "slurm-controller-0", "--", "scancel", "-u", "slurm"},
				80*time.Second,
				5*time.Second,
			)

			checkHostnameResolution(ctx, t, namespace, "slurm-controller-0", workerHostname)
			return ctx
		}).
		Assess("DaemonSet scales down when the NodeSet selects one Kubernetes node", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			selectedKubeNode := &corev1.Node{}
			require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Name: selectedNode}, selectedKubeNode), "failed to get selected Kubernetes node")
			hostnameLabel := selectedKubeNode.Labels[corev1.LabelHostname]
			require.NotEmpty(t, hostnameLabel, "selected Kubernetes node %s has no %s label", selectedNode, corev1.LabelHostname)

			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")
			nodeSelector := make(map[string]string, len(originalNodeSelector)+1)
			for key, value := range originalNodeSelector {
				nodeSelector[key] = value
			}
			nodeSelector[corev1.LabelHostname] = hostnameLabel
			nodeset.Spec.Template.PodSpecWrapper.NodeSelector = nodeSelector
			require.NoError(t, crClient.Update(ctx, nodeset), "failed to restrict DaemonSet NodeSet to one Kubernetes node")

			_, pods := waitForDaemonSetReplicas(crClient, ctx, t, nodesetKey, 1)
			require.Equal(t, selectedNode, pods[0].Spec.NodeName)
			return ctx
		}).
		Assess("DaemonSet scales up when the original node selector is restored", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")
			nodeset.Spec.Template.PodSpecWrapper.NodeSelector = originalNodeSelector
			require.NoError(t, crClient.Update(ctx, nodeset), "failed to restore DaemonSet NodeSet node selector")

			_, pods := waitForDaemonSetReplicas(crClient, ctx, t, nodesetKey, initialDesired)
			seenNodes := make(map[string]struct{}, len(pods))
			for i := range pods {
				require.NotContains(t, seenNodes, pods[i].Spec.NodeName, "multiple DaemonSet pods were assigned to Kubernetes node %s", pods[i].Spec.NodeName)
				seenNodes[pods[i].Spec.NodeName] = struct{}{}
			}
			return ctx
		}).Feature()
}

func updateNodeSetSyncTopology(
	ctx context.Context,
	crClient crclient.Client,
	nodesetKey crclient.ObjectKey,
	enabled *bool,
) error {
	nodeset := &slinkyv1beta1.NodeSet{}
	if err := crClient.Get(ctx, nodesetKey, nodeset); err != nil {
		return fmt.Errorf("get NodeSet %s: %w", nodesetKey, err)
	}
	return objectutils.PatchObject(crClient, ctx, nodeset, func(nodeset *slinkyv1beta1.NodeSet) error {
		nodeset.Spec.SyncTopology = enabled
		return nil
	})
}

func updateNodeTopologyAnnotation(
	ctx context.Context,
	crClient crclient.Client,
	nodeKey crclient.ObjectKey,
	topology *string,
) error {
	node := &corev1.Node{}
	if err := crClient.Get(ctx, nodeKey, node); err != nil {
		return fmt.Errorf("get Kubernetes Node %s: %w", nodeKey, err)
	}
	return objectutils.PatchObject(crClient, ctx, node, func(node *corev1.Node) error {
		if topology == nil {
			delete(node.Annotations, slinkyv1beta1.AnnotationNodeTopologySpec)
			return nil
		}
		if node.Annotations == nil {
			node.Annotations = make(map[string]string)
		}
		node.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec] = *topology
		return nil
	})
}

func checkNodeSetTopologyUnchanged(
	ctx context.Context,
	t *testing.T,
	crClient crclient.Client,
	podKey crclient.ObjectKey,
	namespace string,
	slurmNodeName string,
	wantPodTopology string,
	wantSlurmTopology string,
) {
	t.Helper()

	deadline := time.Now().Add(topologySyncDisabledObservation)
	for {
		readCtx, cancel := context.WithTimeout(ctx, topologyReadTimeout)
		podTopology, slurmTopology, err := readNodeSetTopologies(readCtx, crClient, podKey, namespace, slurmNodeName)
		cancel()

		if ctx.Err() != nil {
			t.Fatalf("context ended while checking disabled topology synchronization: %v", ctx.Err())
		}
		require.NoError(t, err, "failed to read topology while syncTopology=false")
		require.Equal(t, wantPodTopology, podTopology, "NodeSet Pod topology changed while syncTopology=false")
		require.Equal(t, wantSlurmTopology, slurmTopology, "Slurm node topology changed while syncTopology=false")

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context ended while checking disabled topology synchronization: %v", ctx.Err())
		case <-time.After(min(time.Second, remaining)):
		}
	}
}

func readNodeSetTopologies(
	ctx context.Context,
	crClient crclient.Client,
	podKey crclient.ObjectKey,
	namespace string,
	slurmNodeName string,
) (string, string, error) {
	pod := &corev1.Pod{}
	if err := crClient.Get(ctx, podKey, pod); err != nil {
		return "", "", fmt.Errorf("get NodeSet Pod %s: %w", podKey, err)
	}
	nodeInfo, err := test.GetSlurmNodeInfo(ctx, namespace, slurmNodeName)
	if err != nil {
		return "", "", fmt.Errorf("get Slurm node %s: %w", slurmNodeName, err)
	}
	return pod.Annotations[slinkyv1beta1.AnnotationNodeTopologySpec], nodeInfo["Topology"], nil
}

func waitForNodeSetTopology(
	ctx context.Context,
	t *testing.T,
	crClient crclient.Client,
	podKey crclient.ObjectKey,
	namespace string,
	slurmNodeName string,
	want string,
) {
	t.Helper()

	var (
		podTopology   string
		slurmTopology string
		lastErr       error
	)
	err := wait.For(
		func(waitCtx context.Context) (bool, error) {
			podTopology, slurmTopology, lastErr = readNodeSetTopologies(waitCtx, crClient, podKey, namespace, slurmNodeName)
			return lastErr == nil && podTopology == want && slurmTopology == want, nil
		},
		wait.WithContext(ctx),
		wait.WithTimeout(2*time.Minute),
		wait.WithInterval(2*time.Second),
		wait.WithImmediate(),
	)
	require.NoError(
		t,
		err,
		"timed out waiting for NodeSet topology synchronization: Pod got %q, Slurm got %q, want %q; last read error: %v",
		podTopology,
		slurmTopology,
		want,
		lastErr,
	)
}

func testSlurmJWTKeyRotation(namespace string) types.Feature {
	return features.New("Assess Slurm JWT signing key rotation").
		Assess("referenced JWT Secret updates refresh the Slurm client", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			controller := &slinkyv1beta1.Controller{}
			controllerKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm",
			}
			require.NoError(t, crClient.Get(ctx, controllerKey, controller), "failed to get Controller")

			jwtKeyRef := controller.AuthJwtRef()
			secretKey := crclient.ObjectKey{
				Namespace: controller.Namespace,
				Name:      jwtKeyRef.Name,
			}
			jwtSecret := &corev1.Secret{}
			require.NoError(t, crClient.Get(ctx, secretKey, jwtSecret), "failed to get JWT Secret")

			// The chart-generated Secret is immutable. Recreate it once with the
			// same data so the test can exercise a real Secret Update event.
			if jwtSecret.Immutable != nil && *jwtSecret.Immutable {
				mutableSecret := jwtSecret.DeepCopy()
				mutableSecret.ResourceVersion = ""
				mutableSecret.UID = ""
				mutableSecret.CreationTimestamp = metav1.Time{}
				mutableSecret.DeletionTimestamp = nil
				mutableSecret.DeletionGracePeriodSeconds = nil
				mutableSecret.ManagedFields = nil
				mutableSecret.Immutable = nil

				require.NoError(t, crClient.Delete(ctx, jwtSecret), "failed to delete immutable JWT Secret")
				require.Eventually(t, func() bool {
					current := &corev1.Secret{}
					return apierrors.IsNotFound(crClient.Get(ctx, secretKey, current))
				}, 30*time.Second, 500*time.Millisecond, "timed out waiting for immutable JWT Secret deletion")
				require.NoError(t, crClient.Create(ctx, mutableSecret), "failed to recreate mutable JWT Secret")
			}

			// Capture the current slurmctld pod. Updating the key should replace it
			// so Slurm and the operator both begin using the new signing key.
			controllerPodKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-controller-0",
			}
			oldControllerPod := &corev1.Pod{}
			require.NoError(t, crClient.Get(ctx, controllerPodKey, oldControllerPod), "failed to get slurmctld pod")

			randomKey := make([]byte, 64)
			_, err = rand.Read(randomKey)
			require.NoError(t, err, "failed to generate rotated JWT signing key")

			jwtSecret = &corev1.Secret{}
			require.NoError(t, crClient.Get(ctx, secretKey, jwtSecret), "failed to get mutable JWT Secret")
			jwtSecret.Data[jwtKeyRef.Key] = []byte(hex.EncodeToString(randomKey))
			require.NoError(t, crClient.Update(ctx, jwtSecret), "failed to update JWT Secret")

			require.Eventually(t, func() bool {
				current := &corev1.Pod{}
				if err := crClient.Get(ctx, controllerPodKey, current); err != nil {
					return false
				}
				return current.UID != oldControllerPod.UID && podReady(current)
			}, 2*time.Minute, 2*time.Second, "timed out waiting for slurmctld to adopt the rotated JWT key")

			operatorClient := newSlurmRESTClient(t, config, namespace, jwtSecret.Data[jwtKeyRef.Key])

			require.Eventually(t, func() bool {
				pings := &slurmapi.V0044ControllerPingObjectList{}
				err := operatorClient.List(ctx, pings, &slurmclient.ListOptions{SkipCache: true})
				return err == nil
			}, 2*time.Minute, 2*time.Second, "timed out waiting for a controller ping using the rotated JWT key")

			return ctx
		}).Feature()
}

func podReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func testSlurmNodeSetDaemonCordon(namespace string) types.Feature {
	return features.New("Assess DaemonSet NodeSet recreation on a cordoned node").
		Assess("NodeSet pod is recreated on a cordoned Kubernetes node", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			crClient, err := GetControllerRuntimeClient(config)
			require.NoError(t, err, "failed to get controller-runtime client")

			nodesetKey := crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky",
			}
			nodeset := &slinkyv1beta1.NodeSet{}
			require.NoError(t, crClient.Get(ctx, nodesetKey, nodeset), "failed to get NodeSet")

			workerPod := &corev1.Pod{}
			require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{
				Namespace: namespace,
				Name:      "slurm-worker-slinky-0",
			}, workerPod), "failed to get StatefulSet NodeSet pod")
			require.NotEmpty(t, workerPod.Spec.NodeName, "NodeSet pod %s/%s has empty spec.nodeName", workerPod.Namespace, workerPod.Name)

			targetNodeName := workerPod.Spec.NodeName
			targetNode := &corev1.Node{}
			require.NoError(t, crClient.Get(ctx, crclient.ObjectKey{Name: targetNodeName}, targetNode), "failed to get Node %s", targetNodeName)
			targetHostname := targetNode.Labels[corev1.LabelHostname]
			if targetHostname == "" {
				targetHostname = targetNodeName
			}

			originalScalingMode := nodeset.Spec.ScaleStrategy.Mode
			originalReplicas := int32(1)
			if nodeset.Spec.Replicas != nil {
				originalReplicas = *nodeset.Spec.Replicas
			}
			originalSelector := maps.Clone(nodeset.Spec.Template.PodSpecWrapper.NodeSelector)

			t.Cleanup(func() {
				cleanupCtx := context.Background()
				setNodeUnschedulable(cleanupCtx, t, crClient, targetNodeName, false)
				restoreNodeSetStatefulSet(cleanupCtx, t, crClient, nodesetKey, originalScalingMode, originalReplicas, originalSelector)
			})

			pinNodeSetToDaemonNode(ctx, t, crClient, nodesetKey, targetHostname)
			daemonPod := waitForNodeSetPodOnNode(ctx, t, crClient, namespace, nodesetKey.Name, targetNodeName, "", slinkyv1beta1.ScalingModeDaemonset)
			require.True(
				t,
				hasUnschedulableToleration(daemonPod),
				"DaemonSet NodeSet pod %s/%s missing %s=NoSchedule toleration; got tolerations=%v",
				daemonPod.Namespace,
				daemonPod.Name,
				corev1.TaintNodeUnschedulable,
				daemonPod.Spec.Tolerations,
			)

			setNodeUnschedulable(ctx, t, crClient, targetNodeName, true)
			require.NoError(t, crClient.Delete(ctx, daemonPod), "failed to delete DaemonSet NodeSet pod %s/%s", daemonPod.Namespace, daemonPod.Name)

			replacement := waitForNodeSetPodOnNode(ctx, t, crClient, namespace, nodesetKey.Name, targetNodeName, daemonPod.UID, slinkyv1beta1.ScalingModeDaemonset)
			require.True(
				t,
				hasUnschedulableToleration(replacement),
				"replacement DaemonSet NodeSet pod %s/%s missing %s=NoSchedule toleration; got tolerations=%v",
				replacement.Namespace,
				replacement.Name,
				corev1.TaintNodeUnschedulable,
				replacement.Spec.Tolerations,
			)

			setNodeUnschedulable(ctx, t, crClient, targetNodeName, false)
			restoreNodeSetStatefulSet(ctx, t, crClient, nodesetKey, originalScalingMode, originalReplicas, originalSelector)
			// Replica counts can still describe the DaemonSet pod. Wait for the
			// named StatefulSet pod before the next feature uses it.
			err = wait.For(
				conditions.New(config.Client().Resources()).ResourceMatch(workerPod, func(object k8s.Object) bool {
					pod := object.(*corev1.Pod)
					return pod.DeletionTimestamp.IsZero() && podReady(pod)
				}),
				wait.WithContext(ctx),
				wait.WithTimeout(2*time.Minute),
				wait.WithInterval(5*time.Second),
				wait.WithImmediate(),
			)
			require.NoError(t, err, "timed out waiting for restored StatefulSet NodeSet pod %s/%s to be ready", workerPod.Namespace, workerPod.Name)

			return ctx
		}).Feature()
}

func pinNodeSetToDaemonNode(ctx context.Context, t *testing.T, crClient crclient.Client, nodesetKey crclient.ObjectKey, nodeName string) {
	t.Helper()

	mutateNodeSet(ctx, t, crClient, nodesetKey, func(nodeset *slinkyv1beta1.NodeSet) {
		nodeset.Spec.ScaleStrategy.Mode = slinkyv1beta1.ScalingModeDaemonset
		selector := maps.Clone(nodeset.Spec.Template.PodSpecWrapper.NodeSelector)
		if selector == nil {
			selector = map[string]string{}
		}
		selector["kubernetes.io/hostname"] = nodeName
		nodeset.Spec.Template.PodSpecWrapper.NodeSelector = selector
	})
}

func restoreNodeSetStatefulSet(ctx context.Context, t *testing.T, crClient crclient.Client, nodesetKey crclient.ObjectKey, scalingMode slinkyv1beta1.ScalingModeType, replicas int32, nodeSelector map[string]string) {
	t.Helper()

	mutateNodeSet(ctx, t, crClient, nodesetKey, func(nodeset *slinkyv1beta1.NodeSet) {
		nodeset.Spec.ScaleStrategy.Mode = scalingMode
		nodeset.Spec.Replicas = &replicas
		nodeset.Spec.Template.PodSpecWrapper.NodeSelector = maps.Clone(nodeSelector)
	})
}

func mutateNodeSet(ctx context.Context, t *testing.T, crClient crclient.Client, nodesetKey crclient.ObjectKey, mutate func(*slinkyv1beta1.NodeSet)) {
	t.Helper()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nodeset := &slinkyv1beta1.NodeSet{}
		if err := crClient.Get(ctx, nodesetKey, nodeset); err != nil {
			return err
		}
		mutate(nodeset)
		return crClient.Update(ctx, nodeset)
	})
	require.NoError(t, err, "failed to update NodeSet %s/%s", nodesetKey.Namespace, nodesetKey.Name)
}

func setNodeUnschedulable(ctx context.Context, t *testing.T, crClient crclient.Client, nodeName string, unschedulable bool) {
	t.Helper()

	require.Eventually(t, func() bool {
		node := &corev1.Node{}
		if err := crClient.Get(ctx, crclient.ObjectKey{Name: nodeName}, node); err != nil {
			t.Logf("failed to get Node %s: %v", nodeName, err)
			return false
		}
		if node.Spec.Unschedulable == unschedulable {
			return true
		}
		node.Spec.Unschedulable = unschedulable
		if err := crClient.Update(ctx, node); err != nil {
			t.Logf("failed to update Node %s unschedulable=%t: %v", nodeName, unschedulable, err)
			return false
		}
		return true
	}, 30*time.Second, 500*time.Millisecond, "timed out setting Node %s unschedulable=%t", nodeName, unschedulable)
}

func waitForNodeSetPodOnNode(
	ctx context.Context,
	t *testing.T,
	crClient crclient.Client,
	namespace string,
	nodesetName string,
	nodeName string,
	excludeUID k8stypes.UID,
	scalingMode slinkyv1beta1.ScalingModeType,
) *corev1.Pod {
	t.Helper()

	var found *corev1.Pod
	require.Eventually(t, func() bool {
		pods := &corev1.PodList{}
		err := crClient.List(ctx, pods,
			crclient.InNamespace(namespace),
			crclient.MatchingLabels{
				"app.kubernetes.io/name":     "slurmd",
				"app.kubernetes.io/instance": nodesetName,
			},
		)
		if err != nil {
			t.Logf("failed to list NodeSet pods for %s/%s: %v", namespace, nodesetName, err)
			return false
		}
		for i := range pods.Items {
			pod := &pods.Items[i]
			if pod.DeletionTimestamp != nil {
				continue
			}
			if excludeUID != "" && pod.UID == excludeUID {
				continue
			}
			if slinkyv1beta1.ScalingModeType(pod.Labels[slinkyv1beta1.LabelNodeSetScalingMode]) != scalingMode {
				continue
			}
			if pod.Spec.NodeName != nodeName {
				continue
			}
			if !podReady(pod) {
				continue
			}
			found = pod.DeepCopy()
			return true
		}
		return false
	}, 4*time.Minute, 2*time.Second,
		"timed out waiting for ready %s NodeSet pod of %s/%s on node %s (excluding uid %s)",
		scalingMode, namespace, nodesetName, nodeName, excludeUID,
	)
	return found
}

func hasUnschedulableToleration(pod *corev1.Pod) bool {
	for _, toleration := range pod.Spec.Tolerations {
		if toleration.Key == corev1.TaintNodeUnschedulable && toleration.Effect == corev1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

// Accounting tests

func checkAccountingHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) {
	t.Helper()

	// Get Accounting CR
	accounting := &slinkyv1beta1.Accounting{}

	accountingKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, accountingKey, accounting)
	require.NoError(t, err, "failed to Get() accounting using accounting-runtime client")

	accountingUID := accounting.UID

	// Get Accounting StatefulSet using accounting CR
	statefulSetKey := accounting.Key()
	statefulSet := &appsv1.StatefulSet{}
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.NoError(
			collect,
			crClient.Get(ctx, statefulSetKey, statefulSet),
			"failed to Get() statefulset using controller-runtime client",
		)
	}, 30*time.Second, time.Second, "timed out waiting for accounting StatefulSet %s", statefulSetKey)

	// Confirm ownership of controller statefulset
	for _, owner := range statefulSet.OwnerReferences {
		require.Equal(t, accountingUID, owner.UID, "dubious ownership of statefulset: %v", statefulSet)
	}

	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulSet, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, *statefulSet.Spec.Replicas))
	if err != nil {
		_ = crClient.Get(ctx, accountingKey, accounting)
		t.Fatalf(
			"timed out waiting for accounting StatefulSet %s/%s to reach %d ready replicas: %v; Accounting status: %s; StatefulSet status: %s",
			statefulSet.Namespace,
			statefulSet.Name,
			*statefulSet.Spec.Replicas,
			err,
			test.StatusJSON(accounting.Status),
			test.StatusJSON(statefulSet.Status),
		)
	}
}

func testSlurmAccounting(namespace string) types.Feature {
	return features.New("Assess the functionality of the Slurm Accounting").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			return ctx
		}).
		Assess("Controller can contact accounting", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "ping"}
			var wants string

			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 80*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("Sacctmgr has cluster entry", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "show", "cluster", "format=cluster%30", "-n"}

			cmd := exec.Command(command, args...)
			output, err := cmd.Output()
			require.NoError(t, err, "sacctmgr show cluster returned non-zero error code")
			expectedClusterName := namespace + "_slurm"
			require.Equal(t, expectedClusterName, strings.TrimSpace(string(output)), "clustername in slurmdbd does not match expected %s", expectedClusterName)

			return ctx
		}).
		Assess("Sacctmgr add account", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "add", "account", "name=test", "-i"}
			var wants string
			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 40*time.Second, 5*time.Second)

			args = []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=test", "-n", "format=account"}
			wants = "test"

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 40*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("Sacctmgr add user", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "add", "user", "account=test", "name=testuser", "-i"}

			var wants string
			var cleanup_command string
			var cleanup_args []string

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 40*time.Second, 5*time.Second)

			args = []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "show", "user", "name=testuser", "-n", "format=user"}
			wants = "testuser"

			test.WaitForCommand(ctx, t, command, args, wants, cleanup_command, cleanup_args, 40*time.Second, 5*time.Second)

			return ctx
		}).
		Assess("Sacctmgr delete account", func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {

			command := "kubectl"
			args := []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "delete", "account", "test", "-i"}

			cmd := exec.Command(command, args...)
			_, err := cmd.Output()
			require.NoError(t, err, "sacctmgr add account returned non-zero error code")

			args = []string{"exec", "-n", namespace, "slurm-controller-0", "--", "sacctmgr", "show", "account", "name=test", "-n", "format=account"}
			cmd = exec.Command(command, args...)
			output, err := cmd.Output()
			require.NoError(t, err, "sacctmgr show account returned non-zero error code")
			require.NotEqual(t, "test", strings.TrimSpace(string(output)), "account test was not deleted from slurmdbd")

			return ctx
		}).Feature()
}

// LoginSet tests

func checkLoginSetHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, namespace string) {
	t.Helper()

	// Get LoginSet CR
	loginSet := &slinkyv1beta1.LoginSet{}

	loginSetKey := crclient.ObjectKey{
		Namespace: namespace,
		Name:      "slurm-login-slinky",
	}

	err := crClient.Get(ctx, loginSetKey, loginSet)
	require.NoError(t, err, "failed to Get() loginSet using controller-runtime client")

	loginSetUID := loginSet.UID

	// Get loginSet Deployment using loginSet CR
	deploymentKey := loginSet.Key()
	deployment := &appsv1.Deployment{}
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		assert.NoError(
			collect,
			crClient.Get(ctx, deploymentKey, deployment),
			"failed to Get() deployment using controller-runtime client",
		)
	}, 30*time.Second, time.Second, "timed out waiting for LoginSet Deployment %s", deploymentKey)

	// Confirm ownership of loginSet deployment
	for _, owner := range deployment.OwnerReferences {
		require.Equal(t, loginSetUID, owner.UID, "dubious ownership of deployment: %v", deployment)
	}

	// Check whether loginSet deployment is healthy
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, *deployment.Spec.Replicas))
	if err != nil {
		_ = crClient.Get(ctx, loginSetKey, loginSet)
		t.Fatalf(
			"timed out waiting for LoginSet Deployment %s/%s to reach %d ready replicas: %v; LoginSet status: %s; Deployment status: %s",
			deployment.Namespace,
			deployment.Name,
			*deployment.Spec.Replicas,
			err,
			test.StatusJSON(loginSet.Status),
			test.StatusJSON(deployment.Status),
		)
	}
}

func newSlurmRESTClient(t *testing.T, config *envconf.Config, namespace string, jwtKey []byte) slurmclient.Client {
	t.Helper()
	authToken, err := slurmjwt.NewToken(jwtKey).NewSignedToken()
	require.NoError(t, err, "failed to generate Slurm auth token")
	restConfig := rest.CopyConfig(config.Client().RESTConfig())
	// HA ping also waits for the unresponsive peer's Slurm RPC timeout.
	restConfig.Timeout = 30 * time.Second
	httpClient, err := rest.HTTPClientFor(restConfig)
	require.NoError(t, err, "failed to create Kubernetes API HTTP client")
	server := fmt.Sprintf("%s/api/v1/namespaces/%s/services/http:slurm-restapi:slurmrestd/proxy",
		strings.TrimRight(restConfig.Host, "/"), namespace)
	client, err := slurmclient.NewClient(&slurmclient.Config{
		Server:        server,
		TokenProvider: clienttoken.StaticProvider(authToken),
		HTTPClient:    httpClient,
	})
	require.NoError(t, err, "failed to create Slurm REST client")
	return client
}
