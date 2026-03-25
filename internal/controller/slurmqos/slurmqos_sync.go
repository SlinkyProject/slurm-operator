// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmqos

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slurmapi "github.com/SlinkyProject/slurm-client/api/v0044"
	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
)

func (r *SlurmQosReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	qos := &slinkyv1beta1.SlurmQos{}
	if err := r.Get(ctx, req.NamespacedName, qos); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	qosCopy := qos.DeepCopy()
	defaults.SetSlurmQosDefaults(qosCopy)

	controllerKey := qosCopy.Spec.ControllerRef.NamespacedName()
	if controllerKey.Namespace == "" {
		controllerKey.Namespace = qosCopy.Namespace
	}

	slurmClient, err := r.resolveSlurmClient(ctx, qosCopy, controllerKey)

	// Handle deletion before checking client resolution errors.
	// The Controller or client may be gone already during teardown.
	if !qosCopy.DeletionTimestamp.IsZero() {
		return r.handleDelete(ctx, qosCopy, slurmClient)
	}

	if err != nil {
		statusErr := r.syncStatus(ctx, qosCopy, false, nil, err.Error())
		if statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return err
	}

	if !controllerutil.ContainsFinalizer(qosCopy, finalizerName) {
		controllerutil.AddFinalizer(qosCopy, finalizerName)
		if err := r.Update(ctx, qosCopy); err != nil {
			return err
		}
	}

	slurmQos, err := buildV0044Qos(qosCopy)
	if err != nil {
		statusErr := r.syncStatus(ctx, qosCopy, false, nil, fmt.Sprintf("failed to build QOS payload: %v", err))
		if statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return err
	}

	body := slurmapi.V0044OpenapiSlurmdbdQosResp{
		Qos: []slurmapi.V0044Qos{slurmQos},
	}
	qosObj := &slurmtypes.V0044Qos{}
	qosObj.Name = slurmQos.Name
	if err := slurmClient.Create(ctx, qosObj, body); err != nil {
		statusErr := r.syncStatus(ctx, qosCopy, false, nil, fmt.Sprintf("failed to create/update QOS: %v", err))
		if statusErr != nil {
			logger.Error(statusErr, "failed to update status")
		}
		return err
	}

	logger.Info("Successfully synced QOS", "name", qosCopy.Name)
	return r.syncStatus(ctx, qosCopy, true, qosObj.Id, "")
}

// resolveSlurmClient looks up the Controller CR, verifies accounting is enabled,
// and returns the slurm client from the ClientMap. Returns (nil, err) on failure.
func (r *SlurmQosReconciler) resolveSlurmClient(
	ctx context.Context,
	qos *slinkyv1beta1.SlurmQos,
	controllerKey types.NamespacedName,
) (slurmclient.Client, error) {
	controller := &slinkyv1beta1.Controller{}
	if err := r.Get(ctx, controllerKey, controller); err != nil {
		return nil, fmt.Errorf("controller %q not found: %w", controllerKey, err)
	}
	if controller.Spec.AccountingRef == nil {
		return nil, fmt.Errorf("controller %q does not have accounting enabled (accountingRef is nil)", controllerKey)
	}

	slurmClient := r.ClientMap.Get(controllerKey)
	if slurmClient == nil {
		return nil, fmt.Errorf("no slurm client available for controller %q", controllerKey)
	}
	return slurmClient, nil
}

func (r *SlurmQosReconciler) handleDelete(ctx context.Context, qos *slinkyv1beta1.SlurmQos, slurmClient slurmclient.Client) error {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(qos, finalizerName) {
		return nil
	}

	if slurmClient != nil {
		qosObj := &slurmtypes.V0044Qos{}
		qosObj.Name = ptr.To(qos.Name)
		if err := slurmClient.Delete(ctx, qosObj); err != nil {
			return fmt.Errorf("failed to delete QOS %q from Slurm: %w", qos.Name, err)
		}
	} else {
		logger.Info("Controller or slurm client unavailable during delete; QOS may be orphaned in Slurm",
			"name", qos.Name)
	}

	controllerutil.RemoveFinalizer(qos, finalizerName)
	return r.Update(ctx, qos)
}

func buildV0044Qos(qos *slinkyv1beta1.SlurmQos) (slurmapi.V0044Qos, error) {
	var result slurmapi.V0044Qos

	// Unmarshal raw config first so CR-level fields take precedence
	if qos.Spec.Config != nil && qos.Spec.Config.Raw != nil {
		if err := json.Unmarshal(qos.Spec.Config.Raw, &result); err != nil {
			return slurmapi.V0044Qos{}, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	// CR-level fields always override anything from raw config
	result.Name = ptr.To(qos.Name)
	if qos.Spec.Description != "" {
		result.Description = ptr.To(qos.Spec.Description)
	}

	return result, nil
}
