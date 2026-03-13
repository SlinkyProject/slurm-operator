// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/token/slurmjwt"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
	jwt "github.com/golang-jwt/jwt/v5"
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
	token = token.DeepCopy()
	defaults.SetTokenDefaults(token)

	if token.DeletionTimestamp.IsZero() {
		now := time.Now()
		key := objectutils.KeyFunc(token)
		expirationTime, err := r.getExpTime(ctx, token)
		if err != nil {
			durationStore.Push(key, 30*time.Second)
		} else {
			refreshTime := expirationTime.Add(-token.Lifetime() * 1 / 5)
			durationStore.Push(key, refreshTime.Sub(now))
		}
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
				if !ptr.Deref(token.Spec.Refresh, defaults.DefaultTokenRefresh) {
					return nil
				}

				now := time.Now()
				expirationTime, err := r.getExpTime(ctx, token)
				if err != nil {
					if errors.Is(err, jwt.ErrTokenExpired) {
						logger.Info("Token's JWT is expired")
					} else {
						return err
					}
				}

				refreshTime := now
				if !expirationTime.IsZero() {
					refreshTime = expirationTime.Add(-token.Lifetime() * 1 / 5)
					key := objectutils.KeyFunc(token)
					durationStore.Push(key, refreshTime.Sub(now))
				}

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

func (r *TokenReconciler) getExpTime(ctx context.Context, token *slinkyv1beta1.Token) (time.Time, error) {
	authToken, err := r.refResolver.GetSecretKeyRef(ctx, token.SecretRef(), token.Namespace)
	if err != nil {
		return time.Time{}, err
	}
	jwtRef := token.JwtRef()
	signingKey, err := r.refResolver.GetSecretKeyRef(ctx, jwtRef.SecretKeySelector, jwtRef.Namespace)
	if err != nil {
		return time.Time{}, err
	}

	authTokenClaims, err := slurmjwt.ParseTokenClaims(string(authToken), signingKey)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse Slurm auth token claims: %w", err)
	}
	exp, err := authTokenClaims.GetExpirationTime()
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get expiration time: %w", err)
	}

	now := time.Now()
	expirationTime := now
	if exp != nil {
		expirationTime = time.Time(exp.Time)
	}

	return expirationTime, nil
}
