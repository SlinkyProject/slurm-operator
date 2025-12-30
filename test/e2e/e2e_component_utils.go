// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"testing"
	"time"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/test"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

// Dependency Component Health Checks

func checkMariaDBHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
	// Get MariaDB CR

	mariadb := &mariadbv1alpha1.MariaDB{}

	mariadbKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
		Name:      "mariadb",
	}

	err := crClient.Get(ctx, mariadbKey, mariadb)
	if err != nil {
		t.Fatal("failed to Get() mariadb using controller-runtime client")
	}

	// Get every StatefulSet
	statefulSetList := appsv1.StatefulSetList{}
	err = crClient.List(ctx, &statefulSetList)
	if err != nil {
		t.Fatal("failed to List() StatefulSets using controller-runtime client")
	}

	// Build a list of StatefulSets owned by this MariaDB CR
	ownedStatefulSets := appsv1.StatefulSetList{}
	for _, statefulSet := range statefulSetList.Items {
		for _, owner := range statefulSet.OwnerReferences {
			if owner.UID == mariadb.UID {
				ownedStatefulSets.Items = append(ownedStatefulSets.Items, statefulSet)
			}
		}
	}

	// Get MariaDB StatefulSet using CR
	for _, statefulSet := range ownedStatefulSets.Items {
		err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(&statefulSet, func(object k8s.Object) int32 {
			return object.(*appsv1.StatefulSet).Status.ReadyReplicas
		}, *statefulSet.Spec.Replicas))
		if err != nil {
			t.Fatalf("timed out waiting for StatefulSet %v to reach a ready state", statefulSet.Name)
		}
	}

	return ctx
}

// Slinky Component Health Checks

func checkControllerHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) {
	// Get Controller CR
	controller := &slinkyv1beta1.Controller{}

	controllerKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, controllerKey, controller)
	if err != nil {
		t.Fatal("failed to Get() controller using controller-runtime client")
	}

	controllerUID := controller.UID

	// Get Controller StatefulSet using controller CR
	statefulSetKey := controller.Key()
	statefulSet := &appsv1.StatefulSet{}
	err = crClient.Get(ctx, statefulSetKey, statefulSet)
	if err != nil {
		t.Fatal("failed to Get() statefulset using controller-runtime client")
	}

	// Confirm ownership of controller statefulset
	for _, owner := range statefulSet.OwnerReferences {
		if owner.UID != controllerUID {
			t.Fatalf("dubious ownership of statefulset: %v", statefulSet)
		}
	}

	// Wait for controller statefulset to become ready
	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulSet, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, *statefulSet.Spec.Replicas))
	if err != nil {
		t.Fatalf("timed out waiting for StatefulSet %v to reach a ready state", statefulSet.Name)
	}
}

func checkRestAPIHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) {
	// Get RestAPI CR
	restapi := &slinkyv1beta1.RestApi{}

	restapiKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
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
	}, *deployment.Spec.Replicas))
	if err != nil {
		t.Fatalf("timed out waiting for Deployment %v to reach a ready state", deployment.Name)
	}
}

func checkNodeSetReplicas(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config, nodesetKey crclient.ObjectKey) {
	nodeset := &slinkyv1beta1.NodeSet{}

	for retry := range 16 {

		err := crClient.Get(ctx, nodesetKey, nodeset)
		if err != nil {
			t.Fatal("failed to Get() NodeSet using controller-runtime client")
		}

		if *nodeset.Spec.Replicas == nodeset.Status.AvailableReplicas {
			break
		}

		if retry == 15 {
			t.Fatalf("Timed out waiting for NodeSet replicas to become ready. \nDesired replicas: %d \nReady replicas: %d", *nodeset.Spec.Replicas, nodeset.Status.AvailableReplicas)
		}

		time.Sleep(5 * time.Second)
	}
}

func checkAccountingHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) {
	// Get Accounting CR
	accounting := &slinkyv1beta1.Accounting{}

	accountingKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
		Name:      "slurm",
	}

	err := crClient.Get(ctx, accountingKey, accounting)
	if err != nil {
		t.Fatal("failed to Get() accounting using accounting-runtime client")
	}

	accountingUID := accounting.UID

	// Get Accounting StatefulSet using accounting CR
	statefulSetKey := accounting.Key()
	statefulSet := &appsv1.StatefulSet{}
	err = crClient.Get(ctx, statefulSetKey, statefulSet)
	if err != nil {
		t.Fatal("failed to Get() statefulset using controller-runtime client")
	}

	// Confirm ownership of controller statefulset
	for _, owner := range statefulSet.OwnerReferences {
		if owner.UID != accountingUID {
			t.Fatalf("dubious ownership of statefulset: %v", statefulSet)
		}
	}

	err = wait.For(conditions.New(config.Client().Resources()).ResourceScaled(statefulSet, func(object k8s.Object) int32 {
		return object.(*appsv1.StatefulSet).Status.ReadyReplicas
	}, *statefulSet.Spec.Replicas))
	if err != nil {
		t.Fatalf("timed out waiting for StatefulSet %v to reach a ready state", statefulSet.Name)
	}
}

func checkLoginSetHealth(crClient crclient.Client, ctx context.Context, t *testing.T, config *envconf.Config) {
	// Get LoginSet CR
	loginSet := &slinkyv1beta1.LoginSet{}

	loginSetKey := crclient.ObjectKey{
		Namespace: test.SlurmNamespace,
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
	}, *deployment.Spec.Replicas))
	if err != nil {
		t.Fatalf("timed out waiting for Deployment %v to reach a ready state", deployment.Name)
	}
}
