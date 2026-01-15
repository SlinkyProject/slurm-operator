// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package slurmclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slurmclient "github.com/SlinkyProject/slurm-client/pkg/client"
	slurmobject "github.com/SlinkyProject/slurm-client/pkg/object"
	slurmtypes "github.com/SlinkyProject/slurm-client/pkg/types"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	builder "github.com/SlinkyProject/slurm-operator/internal/builder/restapibuilder"
	"github.com/SlinkyProject/slurm-operator/internal/controller/nodeset/eventhandler"
	"github.com/SlinkyProject/slurm-operator/internal/controller/token/slurmjwt"
)

// Sync implements control logic for synchronizing a Restapi.
func (r *SlurmClientReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	controller := &slinkyv1beta1.Controller{}
	if err := r.Get(ctx, req.NamespacedName, controller); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Removed slurm client", "controller", req)
			_ = r.ClientMap.Remove(req.NamespacedName)
			return nil
		}
		return err
	}
	controllerKey := client.ObjectKeyFromObject(controller)

	server, err := r.getRestApiServer(ctx, controller)
	if err != nil {
		if apierrors.IsNotFound(err) {
			_ = r.ClientMap.Remove(controllerKey)
			durationStore.Push(controllerKey.String(), 10*time.Second)
			return nil
		}
		return err
	}

	signingKey, err := r.refResolver.GetSecretKeyRef(ctx, controller.AuthJwtHs256Ref(), controller.Namespace)
	if err != nil {
		return err
	}

	lifetime := 15 * time.Minute
	refresh := lifetime * 4 / 5
	authToken, err := slurmjwt.NewToken(signingKey).
		WithLifetime(lifetime).
		NewSignedToken()
	if err != nil {
		return fmt.Errorf("failed to create Slurm auth token: %w", err)
	}

	authTokenClaims, err := slurmjwt.ParseTokenClaims(authToken, signingKey)
	if err != nil {
		return fmt.Errorf("failed to parse Slurm auth token: %w", err)
	}
	exp, err := authTokenClaims.GetExpirationTime()
	if err != nil {
		return fmt.Errorf("failed to get expiration time: %w", err)
	}

	if t := durationStore.Peek(controllerKey.String()); t == 0 {
		logger.Info("Refresh token before expiration", "exp", exp, "refresh", time.Now().Add(refresh))
		durationStore.Push(controllerKey.String(), refresh)
	}

	// There is an existing client, handle in-place updates
	if slurmClient := r.ClientMap.Get(controllerKey); slurmClient != nil {
		slurmClient.SetServer(server)
		slurmClient.SetToken(authToken)
		return nil
	}

	config := &slurmclient.Config{
		Server:    server,
		AuthToken: authToken,
	}
	options := &slurmclient.ClientOptions{
		DisableFor: []slurmobject.Object{
			&slurmtypes.V0041ControllerPing{},
		},
	}
	slurmClient, err := slurmclient.NewClient(config, options)
	if err != nil {
		return fmt.Errorf("failed to create slurm client: %w", err)
	}
	eventhandler.SetEventHandler(slurmClient, r.EventCh)

	if r.ClientMap.Add(controllerKey, slurmClient) {
		logger.Info("Added slurm client", "controller", controllerKey.String())
	}

	return nil
}

func (r *SlurmClientReconciler) getRestApiServer(ctx context.Context, controller *slinkyv1beta1.Controller) (string, error) {
	logger := log.FromContext(ctx)

	restapiList, err := r.refResolver.GetRestapisForController(ctx, controller)
	if err != nil {
		return "", err
	}
	if len(restapiList.Items) == 0 {
		return "", errors.New(http.StatusText(http.StatusNotFound))
	}

	server := fmt.Sprintf("http://%s:%d", restapiList.Items[0].ServiceFQDNShort(), builder.SlurmrestdPort)
	if val := os.Getenv("DEBUG"); val == "1" {
		logger.Info("overriding restapi URL with localhost")
		server = fmt.Sprintf("http://localhost:%d", builder.SlurmrestdPort)
	}

	return server, nil
}
