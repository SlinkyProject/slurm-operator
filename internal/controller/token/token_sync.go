// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/controller/token/slurmjwt"
	"github.com/togethercomputer/slurm-operator/internal/utils/objectutils"
)

type SyncStep struct {
	Name string
	Sync func(ctx context.Context, token *slinkyv1beta1.Token) error
}

// Sync implements control logic for synchronizing a Token.
func (r *TokenReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	token := &slinkyv1beta1.Token{}
	if err := r.Get(ctx, req.NamespacedName, token); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Token has been deleted", "request", req)
			return nil
		}
		return err
	}

	syncSteps := []SyncStep{
		{
			Name: "Secret",
			Sync: func(ctx context.Context, token *slinkyv1beta1.Token) error {
				object, err := r.builder.BuildTokenSecret(token)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, object, false); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "Refresh",
			Sync: func(ctx context.Context, token *slinkyv1beta1.Token) error {
				if !token.Spec.Refresh {
					return nil
				}

				authToken, err := r.refResolver.GetSecretKeyRef(ctx, token.SecretRef(), token.Namespace)
				if err != nil {
					return err
				}
				jwtHs256Ref := token.JwtHs256Ref()
				signingKey, err := r.refResolver.GetSecretKeyRef(ctx, &jwtHs256Ref.SecretKeySelector, jwtHs256Ref.Namespace)
				if err != nil {
					return err
				}

				authTokenClaims, err := slurmjwt.ParseTokenClaims(string(authToken), signingKey)
				if err != nil {
					logger.V(1).Error(err, "failed to parse Slurm auth token claims")
				}
				exp, err := authTokenClaims.GetExpirationTime()
				if err != nil {
					logger.V(1).Error(err, "failed to get expiration time")
				}

				now := time.Now()
				expirationTime := now
				if exp != nil {
					expirationTime = time.Time(exp.Time)
				}

				key := token.Key().String()
				durationStore.Push(key, 30*time.Second)

				refreshTime := expirationTime.Add(-token.Lifetime() * 1 / 5)
				if now.Before(refreshTime) {
					logger.V(2).Info("token is not near expiration time yet, skipping...", "expirationTime", expirationTime)
					return nil
				}

				object, err := r.builder.BuildTokenSecret(token)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}

				return nil
			},
		},
	}

	for _, s := range syncSteps {
		if err := s.Sync(ctx, token); err != nil {
			e := fmt.Errorf("[%s]: %w", s.Name, err)
			errors := []error{e}
			if err := r.syncStatus(ctx, token); err != nil {
				e := fmt.Errorf("[%s]: %w", s.Name, err)
				errors = append(errors, e)
			}
			return utilerrors.NewAggregate(errors)
		}
	}

	return r.syncStatus(ctx, token)
}
