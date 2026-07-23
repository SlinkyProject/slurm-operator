// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmcontrol

import (
	"context"
	"errors"
	"net/http"

	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/clientmap"
)

var ErrNoSlurmClient = errors.New("NoSlurmClient")

type SlurmControlInterface interface {
	// GetActiveHAController returns a list of controller pings.
	GetActiveHAController(ctx context.Context, controller *slinkyv1beta1.Controller) ([]ControllerPing, error)
}

// realSlurmControl is the default implementation of SlurmControlInterface.
type realSlurmControl struct {
	clientMap *clientmap.ClientMap
}

type ControllerPing struct {
	Name   string
	Active bool
}

// GetActiveHAController implements SlurmControlInterface.
func (r *realSlurmControl) GetActiveHAController(ctx context.Context, controller *slinkyv1beta1.Controller) ([]ControllerPing, error) {
	logger := log.FromContext(ctx)

	slurmClient := r.lookupClient(controller)
	if slurmClient == nil {
		logger.V(2).Info("no client for controller, cannot do GetActiveHAController()")
		return nil, ErrNoSlurmClient
	}

	opts := &slurmclient.ListOptions{
		SkipCache: true,
	}
	controllerPingList := &slurmtypes.V0044ControllerPingList{}
	if err := slurmClient.List(ctx, controllerPingList, opts); err != nil {
		if tolerateError(err) {
			return nil, nil
		}
		return nil, err
	}

	controllerPings := make([]ControllerPing, 0, len(controllerPingList.Items))
	foundActive := false
	for _, ping := range controllerPingList.Items {
		controllerPing := ControllerPing{
			Name:   ptr.Deref(ping.Hostname, ""),
			Active: ping.Responding && !foundActive,
		}
		if !foundActive {
			foundActive = ping.Responding
		}
		controllerPings = append(controllerPings, controllerPing)
	}

	return controllerPings, nil
}

func (r *realSlurmControl) lookupClient(controller *slinkyv1beta1.Controller) slurmclient.Client {
	key := ktypes.NamespacedName{
		Namespace: controller.Namespace,
		Name:      controller.Name,
	}
	return r.clientMap.Get(key)
}

var _ SlurmControlInterface = &realSlurmControl{}

func NewSlurmControl(clientMap *clientmap.ClientMap) SlurmControlInterface {
	return &realSlurmControl{
		clientMap: clientMap,
	}
}

func tolerateError(err error) bool {
	if err == nil {
		return true
	}
	errText := err.Error()
	if errText == http.StatusText(http.StatusNotFound) ||
		errText == http.StatusText(http.StatusNoContent) {
		return true
	}
	return false
}
