// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/controller/token/slurmjwt"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/syncsteps"
	"github.com/SlinkyProject/slurm-operator/internal/utils/mathutils"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

// Sync implements control logic for synchronizing a Token.
func (r *TokenReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	token := &slinkyv1beta1.Token{}
	if err := r.Get(ctx, req.NamespacedName, token); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Token has been deleted")
			return nil
		}
		return err
	}
	token = token.DeepCopy()
	defaults.SetTokenDefaults(token)

	key := objectutils.KeyFunc(token)
	if !token.DeletionTimestamp.IsZero() {
		logger.Info("Token is being deleted, skipping sync")
		return nil
	}

	steps := []syncsteps.Step[*slinkyv1beta1.Token]{
		{
			Name: "Secret",
			SyncFn: func(ctx context.Context, token *slinkyv1beta1.Token) error {
				object, err := r.builder.BuildTokenSecret(token)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, token, object, false); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				if err := r.adoptSecret(ctx, token, object); err != nil {
					return fmt.Errorf("failed to adopt object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "Refresh",
			SyncFn: func(ctx context.Context, token *slinkyv1beta1.Token) error {
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
					// Requeue at 80% of Lifetime
					refreshTime = expirationTime.Add(-token.Lifetime() / 5)
					requeueAfter := mathutils.Clamp(refreshTime.Sub(now), 1*time.Second, token.Lifetime())
					durationStore.Push(key, requeueAfter)
				}

				if now.Before(refreshTime) {
					logger.V(2).Info("token is not near expiration time yet, skipping...", "expirationTime", expirationTime)
					return nil
				}

				object, err := r.builder.BuildTokenSecret(token)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, token, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}

				// Requeue at 80% of Lifetime
				requeueAfter := token.Lifetime() - token.Lifetime()/5
				durationStore.Push(key, requeueAfter)

				return nil
			},
		},
	}

	if err := syncsteps.Sync(ctx, r.eventRecorder, token, steps); err != nil {
		errs := []error{err}
		if err := r.syncStatus(ctx, token); err != nil {
			e := fmt.Errorf("failed status syncFn: %w", err)
			errs = append(errs, e)
		}
		return utilerrors.NewAggregate(errs)
	}

	return r.syncStatus(ctx, token)
}

// adoptSecret repoints the auth-token Secret's controller owner at the Token.
//
// Operator versions before this fix set the JWT signing key Secret as the owner,
// which orphaned the credential when the Token was deleted and made the Secret
// watch in SetupWithManager unresolvable. Those Secrets cannot be repaired by
// the normal sync: SyncObject is called create-only here, and skips immutable
// Secrets entirely. Owner references are metadata, so they remain patchable even
// when the Secret's contents are immutable.
//
// Only Secrets owned by this Token's signing key are adopted. spec.secretRef can
// name any Secret, so anything else is left alone rather than taking ownership of
// a resource this controller did not create.
func (r *TokenReconciler) adoptSecret(ctx context.Context, token *slinkyv1beta1.Token, desired *corev1.Secret) error {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.Get(ctx, token.SecretKey(), secret); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !secret.DeletionTimestamp.IsZero() {
		return nil
	}

	owner := metav1.GetControllerOf(secret)
	if owner == nil || owner.UID == token.UID {
		return nil
	}
	if owner.Kind != "Secret" || owner.Name != token.JwtRef().Name {
		logger.V(1).Info("Auth-token Secret is owned by another resource, skipping adoption",
			"secret", klog.KObj(secret), "owner", owner)
		return nil
	}

	logger.Info("Adopting auth-token Secret", "secret", klog.KObj(secret))
	return objectutils.PatchObject(r.Client, ctx, secret, func(o *corev1.Secret) error {
		o.OwnerReferences = desired.OwnerReferences
		return nil
	})
}

func (r *TokenReconciler) getExpTime(ctx context.Context, token *slinkyv1beta1.Token) (time.Time, error) {
	authToken, err := r.refResolver.GetSecretKeyRef(ctx, token.SecretRef(), token.Namespace)
	if err != nil {
		return time.Time{}, err
	}
	jwtRef := token.JwtRef()
	signingKey, err := r.refResolver.GetSecretKeyRef(ctx, jwtRef, token.Namespace)
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
