// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slinkyv1beta1 "github.com/togethercomputer/slurm-operator/api/v1beta1"
	"github.com/togethercomputer/slurm-operator/internal/controller/token/slurmjwt"
	"github.com/togethercomputer/slurm-operator/internal/utils/objectutils"
	"github.com/togethercomputer/slurm-operator/internal/utils/structutils"
)

// syncStatus handles determining and updating the status.
func (r *TokenReconciler) syncStatus(
	ctx context.Context,
	token *slinkyv1beta1.Token,
) error {
	logger := log.FromContext(ctx)

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
		return fmt.Errorf("failed to parse Slurm auth token: %w", err)
	}
	iat, err := authTokenClaims.GetIssuedAt()
	if err != nil {
		return fmt.Errorf("failed to get issued at time: %w", err)
	}

	var issuedAt *metav1.Time
	if iat != nil {
		issuedAt = ptr.To(metav1.NewTime(iat.Time))
	}

	newStatus := &slinkyv1beta1.TokenStatus{
		IssuedAt:   issuedAt,
		Conditions: structutils.MergeList(token.Status.Conditions),
	}

	if apiequality.Semantic.DeepEqual(token.Status, newStatus) {
		logger.V(2).Info("Token Status has not changed, skipping status update",
			"token", klog.KObj(token), "status", token.Status)
		return nil
	}

	if err := r.updateStatus(ctx, token, newStatus); err != nil {
		return fmt.Errorf("error updating Token(%s) status: %w",
			klog.KObj(token), err)
	}

	return nil
}

func (r *TokenReconciler) updateStatus(
	ctx context.Context,
	token *slinkyv1beta1.Token,
	newStatus *slinkyv1beta1.TokenStatus,
) error {
	logger := log.FromContext(ctx)
	tokenKey := objectutils.NamespacedName(token)

	logger.V(1).Info("Pending Token Status update",
		"token", klog.KObj(token), "newStatus", newStatus)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		toUpdate := &slinkyv1beta1.Token{}
		if err := r.Get(ctx, tokenKey, toUpdate); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		toUpdate.Status = *newStatus
		return r.Status().Update(ctx, toUpdate)
	})
}
