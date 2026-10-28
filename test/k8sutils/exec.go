// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package k8sutils

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/streaming/pkg/httpstream"
)

// annotationDefaultContainer names the container that exec uses when the
// caller does not name one.
const annotationDefaultContainer = "kubectl.kubernetes.io/default-container"

// PodExec runs command in a container of the pod through the Kubernetes API
// and returns its stdout. An empty container selects the pod's default
// container. The error includes the command and both output streams.
func PodExec(ctx context.Context, config *rest.Config, namespace, pod, container string, command ...string) (string, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("create Kubernetes clientset: %w", err)
	}
	if container == "" {
		current, err := clientset.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
		if err != nil {
			return "", fmt.Errorf("get pod %s/%s: %w", namespace, pod, err)
		}
		if container, err = defaultContainer(current); err != nil {
			return "", err
		}
	}

	request := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)
	executor, err := newExecutor(config, request.URL())
	if err != nil {
		return "", fmt.Errorf("create executor for pod %s/%s: %w", namespace, pod, err)
	}

	var stdout, stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		return stdout.String(), fmt.Errorf("exec %q in pod %s/%s container %s: %w; stdout: %q; stderr: %q",
			strings.Join(command, " "), namespace, pod, container, err,
			strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// defaultContainer returns the container named by the pod's default-container
// annotation, or its first container.
func defaultContainer(pod *corev1.Pod) (string, error) {
	if name := pod.Annotations[annotationDefaultContainer]; name != "" {
		return name, nil
	}
	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("pod %s/%s has no containers", pod.Namespace, pod.Name)
	}
	return pod.Spec.Containers[0].Name, nil
}

// newExecutor streams over WebSocket and falls back to SPDY for API servers
// that do not support it, as kubectl exec does.
func newExecutor(config *rest.Config, execURL *url.URL) (remotecommand.Executor, error) {
	websocket, err := remotecommand.NewWebSocketExecutor(config, "GET", execURL.String())
	if err != nil {
		return nil, err
	}
	spdy, err := remotecommand.NewSPDYExecutor(config, "POST", execURL)
	if err != nil {
		return nil, err
	}
	return remotecommand.NewFallbackExecutor(websocket, spdy, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
}
