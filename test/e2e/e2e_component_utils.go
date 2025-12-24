// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/SlinkyProject/slurm-operator/internal/utils/podutils"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

func checkControllerHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get Controller CR
	controller := &slinkyv1beta1.Controller{}

	controllerKey := crclient.ObjectKey{
		Namespace: slurmNamespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, controllerKey, controller)
	if err != nil {
		t.Fatal("failed to Get() controller using controller-runtime client")
	}

	controllerUID := controller.UID

	// Get Controller StatefulSet using controller CR
	statefulSetKey := controller.Key()
	statefulset := &appsv1.StatefulSet{}
	err = crClient.Get(ctx, statefulSetKey, statefulset)
	if err != nil {
		t.Fatal("failed to Get() statefulset using controller-runtime client")
	}

	// Confirm ownership of controller statefulset
	for _, owner := range statefulset.OwnerReferences {
		if owner.UID != controllerUID {
			t.Fatalf("dubious ownership of statefulset: %v", statefulset)
		}
	}

	// Wait for controller statefulset to become ready
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulset, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, 1))
	if err != nil {
		t.Fatalf("timed out waiting for StatefulSet %v to reach a ready state", statefulset.Name)
	}
}

func checkRestAPIHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get RestAPI CR
	restapi := &slinkyv1beta1.RestApi{}

	restapiKey := crclient.ObjectKey{
		Namespace: slurmNamespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, restapiKey, restapi)
	if err != nil {
		t.Fatal("failed to Get() restapi using controller-runtime client")
	}

	restapiUID := restapi.UID

	// Get RestAPI Deployment using RestAPI CR
	deploymentKey := restapi.Key()
	deployment := &appsv1.Deployment{}
	err = crClient.Get(ctx, deploymentKey, deployment)
	if err != nil {
		t.Fatal("failed to Get() deployment using controller-runtime client")
	}

	// Confirm ownership of RestAPI deployment
	for _, owner := range deployment.OwnerReferences {
		if owner.UID != restapiUID {
			t.Fatalf("dubious ownership of deployment: %v", deployment)
		}
	}

	// Check whether RestAPI deployment is healthy
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, 1))
	if err != nil {
		t.Fatalf("timed out waiting for Deployment %v to reach a ready state", deployment.Name)
	}
}

func checkNodeSetHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get NodeSet CR
	nodeSetList := slinkyv1beta1.NodeSetList{}
	err := crClient.List(ctx, &nodeSetList)
	if err != nil {
		t.Fatal("failed to List() NodeSets using controller-runtime client")
	}

	// For every NodeSet CR
	for _, nodeSet := range nodeSetList.Items {
		nodesetUID := nodeSet.UID

		// Get all pods
		podList := corev1.PodList{}
		err = crClient.List(ctx, &podList)
		if err != nil {
			t.Fatal("failed to List() Pods using controller-runtime client")
		}

		// Build a list of pods owned by this NodeSet CR
		ownedPods := corev1.PodList{}
		for _, pod := range podList.Items {
			for _, owner := range pod.OwnerReferences {
				if owner.UID == nodesetUID {
					ownedPods.Items = append(ownedPods.Items, pod)
				}
			}
		}

		// Check the health of all pods owned by this NodeSet CR
		for _, pod := range ownedPods.Items {
			// Check whether pod's containers are healthy
			err := wait.For(conditions.New(config.Client().Resources()).ResourceMatch(&pod, func(object k8s.Object) bool {
				return podutils.IsRunning(&pod)
			}))
			if err != nil {
				t.Fatalf("timed out waiting for Pod %v to reach a ready state", pod.Name)
			}
		}
	}
}

func checkAccountingHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get Accounting CR
	accounting := &slinkyv1beta1.Accounting{}

	accountingKey := crclient.ObjectKey{
		Namespace: slurmNamespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, accountingKey, accounting)
	if err != nil {
		t.Fatal("failed to Get() accounting using accounting-runtime client")
	}

	accountingUID := accounting.UID

	// Get Accounting StatefulSet using accounting CR
	statefulSetKey := accounting.Key()
	statefulset := &appsv1.StatefulSet{}
	err = crClient.Get(ctx, statefulSetKey, statefulset)
	if err != nil {
		t.Fatal("failed to Get() statefulset using controller-runtime client")
	}

	// Confirm ownership of controller statefulset
	for _, owner := range statefulset.OwnerReferences {
		if owner.UID != accountingUID {
			t.Fatalf("dubious ownership of statefulset: %v", statefulset)
		}
	}

	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulset, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, 1))
	if err != nil {
		t.Fatalf("timed out waiting for StatefulSet %v to reach a ready state", statefulset.Name)
	}
}

func checkLoginSetHealth(crClient crclient.Client, ctx context.Context, slurmNamespace string, t *testing.T, config *envconf.Config) {
	// Get LoginSet  CR
	loginSet := &slinkyv1beta1.LoginSet{}

	loginSetKey := crclient.ObjectKey{
		Namespace: slurmNamespace,
		Name:      "slurm-login-slinky",
	}

	err := crClient.Get(ctx, loginSetKey, loginSet)
	if err != nil {
		t.Fatal("failed to Get() loginSet using controller-runtime client")
	}

	loginSetUID := loginSet.UID

	// Get loginSet Deployment using loginSet CR
	deploymentKey := loginSet.Key()
	deployment := &appsv1.Deployment{}
	err = crClient.Get(ctx, deploymentKey, deployment)
	if err != nil {
		t.Fatal("failed to Get() deployment using controller-runtime client")
	}

	// Confirm ownership of loginSet deployment
	for _, owner := range deployment.OwnerReferences {
		if owner.UID != loginSetUID {
			t.Fatalf("dubious ownership of deployment: %v", deployment)
		}
	}

	// Check whether loginSet deployment is healthy
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(deployment, func(object k8s.Object) int32 {
		return object.(*appsv1.Deployment).Status.ReadyReplicas
	}, 1))
	if err != nil {
		t.Fatalf("timed out waiting for Deployment %v to reach a ready state", deployment.Name)
	}
}
