// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	slinkyv1beta1 "github.com/SlinkyProject/slurm-operator/api/v1beta1"
	"github.com/SlinkyProject/slurm-operator/internal/defaults"
	"github.com/SlinkyProject/slurm-operator/internal/syncsteps"
	"github.com/SlinkyProject/slurm-operator/internal/utils/objectutils"
)

// Sync implements control logic for synchronizing a Controller.
func (r *ControllerReconciler) Sync(ctx context.Context, req reconcile.Request) error {
	logger := log.FromContext(ctx)

	controller := &slinkyv1beta1.Controller{}
	if err := r.Get(ctx, req.NamespacedName, controller); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Controller has been deleted", "request", req)
			return nil
		}
		return err
	}
	controller = controller.DeepCopy()
	defaults.SetControllerDefaults(controller)

	if !controller.DeletionTimestamp.IsZero() {
		logger.Info("Controller is being deleted, skipping sync", "request", req)
		return nil
	}

	steps := []syncsteps.Step[*slinkyv1beta1.Controller]{
		{
			Name: "Service",
			// Downstream steps depend on the Service's shape (per-pod DNS
			// under HA); do not build config/StatefulSet against a Service
			// that failed to sync.
			StopOnError: true,
			SyncFn: func(ctx context.Context, controller *slinkyv1beta1.Controller) error {
				if controller.Spec.External {
					return nil
				}
				object, err := r.builder.BuildControllerService(controller)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				// clusterIP is immutable, so toggling Replicas across 1 cannot
				// be patched: delete the outdated Service and recreate it.
				existing := &corev1.Service{}
				switch err := r.Get(ctx, client.ObjectKeyFromObject(object), existing); {
				case apierrors.IsNotFound(err):
					// SyncObject will create it.
				case err != nil:
					return fmt.Errorf("failed to get service: %w", err)
				case controllerOwns(existing, controller) &&
					(existing.Spec.ClusterIP == corev1.ClusterIPNone) != (object.Spec.ClusterIP == corev1.ClusterIPNone):
					switch err := r.Delete(ctx, existing, client.Preconditions{UID: &existing.UID}); {
					case apierrors.IsNotFound(err):
						// Already gone; SyncObject will create it.
					case err != nil:
						return fmt.Errorf("failed to delete outdated service: %w", err)
					default:
						r.eventRecorder.Eventf(controller, existing, corev1.EventTypeNormal, objectutils.ReasonDeleteSucceeded, "Delete", "Deleted Service %s: clusterIP is immutable and replicas toggled headlessness, recreating", klog.KObj(existing))
					}
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, controller, object, false); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "Config",
			SyncFn: func(ctx context.Context, controller *slinkyv1beta1.Controller) error {
				var object *corev1.ConfigMap
				var err error
				if controller.Spec.External {
					object, err = r.builder.BuildControllerConfigExternal(controller)
				} else {
					object, err = r.builder.BuildControllerConfig(controller)
				}
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, controller, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "StatefulSet",
			SyncFn: func(ctx context.Context, controller *slinkyv1beta1.Controller) error {
				if controller.Spec.External {
					return nil
				}
				object, err := r.builder.BuildController(controller)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}
				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, controller, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
		{
			Name: "ServiceMonitor",
			SyncFn: func(ctx context.Context, controller *slinkyv1beta1.Controller) error {
				object, err := r.builder.BuildControllerServiceMonitor(controller)
				if err != nil {
					return fmt.Errorf("failed to build: %w", err)
				}

				if !controller.Spec.Metrics.Enabled || !controller.Spec.Metrics.ServiceMonitor.Enabled {
					// Determine GVK for ServiceMonitor
					serviceMonitor, err := r.GroupVersionKindFor(object.DeepCopy())
					if err != nil {
						return err
					}

					// Determine if the ServiceMonitor Kind is installed server-side
					if _, err = r.RESTMapper().RESTMapping(serviceMonitor.GroupKind(), serviceMonitor.Version); err != nil {
						if meta.IsNoMatchError(err) {
							logger.Info("skipping sync of servicemonitor for controller because GVK is not recognized", "GVK", serviceMonitor)
							return nil
						}
						return err
					}

					if err := objectutils.DeleteObject(r.Client, ctx, r.eventRecorder, controller, object); err != nil {
						return fmt.Errorf("failed to delete object (%s): %w", klog.KObj(object), err)
					}
					return nil
				}

				if err := objectutils.SyncObject(r.Client, ctx, r.eventRecorder, controller, object, true); err != nil {
					return fmt.Errorf("failed to sync object (%s): %w", klog.KObj(object), err)
				}
				return nil
			},
		},
	}

	if err := syncsteps.Sync(ctx, r.eventRecorder, controller, steps); err != nil {
		errs := []error{err}
		if err := r.syncStatus(ctx, controller); err != nil {
			e := fmt.Errorf("failed status syncFn: %w", err)
			errs = append(errs, e)
		}
		return utilerrors.NewAggregate(errs)
	}

	return r.syncStatus(ctx, controller)
}

// controllerOwns reports whether obj has a controller owner reference naming
// the given Controller CR. Matching by name rather than UID keeps the guard
// effective when a Controller is deleted and recreated under the same name
// before garbage collection removes the old Service.
func controllerOwns(obj client.Object, controller *slinkyv1beta1.Controller) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller &&
			strings.HasPrefix(ref.APIVersion, slinkyv1beta1.GroupVersion.Group+"/") &&
			ref.Kind == slinkyv1beta1.ControllerKind &&
			ref.Name == controller.Name {
			return true
		}
	}
	return false
}
