// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

// Package slurm runs the Slurm client commands (sbatch, srun, scontrol,
// scancel, sacct, and sacctmgr) inside the pods of a Slurm cluster through the
// Kubernetes exec API, and decodes their output for e2e tests. Commands that
// support --json are decoded into the slurm-client API types of DataParser.
package slurm

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/test/k8sutils"
)

// ControllerPod is the pod the Slurm chart runs slurmctld in.
const ControllerPod = "slurm-controller-0"

// Client runs Slurm commands in a pod of a Slurm cluster.
type Client struct {
	Namespace string
	Pod       string

	config *rest.Config
}

// NewClient returns a Client that runs commands in the controller pod of the
// Slurm cluster in namespace, using config to reach the Kubernetes API.
func NewClient(config *rest.Config, namespace string) *Client {
	return &Client{Namespace: namespace, Pod: ControllerPod, config: config}
}

// InPod returns a Client that runs commands in the default container of
// another pod in the same namespace.
func (c *Client) InPod(pod string) *Client {
	return &Client{Namespace: c.Namespace, Pod: pod, config: c.config}
}

// Exec runs command in the default container of the client's pod and returns
// its stdout.
func (c *Client) Exec(ctx context.Context, command ...string) (string, error) {
	return k8sutils.PodExec(ctx, c.config, c.Namespace, c.Pod, "", command...)
}

// NodePod returns the running NodeSet pod that backs the Slurm node.
func (c *Client) NodePod(ctx context.Context, node string) (string, error) {
	clientset, err := kubernetes.NewForConfig(c.config)
	if err != nil {
		return "", fmt.Errorf("create Kubernetes clientset: %w", err)
	}
	pods, err := clientset.CoreV1().Pods(c.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: slinkyv1beta1.LabelNodeSetPodHostname + "=" + node,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return "", fmt.Errorf("list pods of Slurm node %s in namespace %s: %w", node, c.Namespace, err)
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no running pod backs Slurm node %s in namespace %s", node, c.Namespace)
	}
	return pods.Items[0].Name, nil
}
