// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lifecycle

import (
	"context"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/go-logr/logr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type reconciler struct {
	seedActuator  Actuator
	shootActuator Actuator
	client        client.Client
	reader        client.Reader
	statusUpdater extensionscontroller.StatusUpdaterCustom
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// dnsrecord resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(seedActuator, shootActuator Actuator) reconcile.Reconciler {
	return reconcilerutils.OperationAnnotationWrapper(
		func() client.Object { return &extensionsv1alpha1.Logging{} },
		&reconciler{
			seedActuator:  seedActuator,
			shootActuator: shootActuator,
			statusUpdater: extensionscontroller.NewStatusUpdater(),
		},
	)
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	var err error
	if err = f(r.seedActuator); err != nil {
		return err
	}
	if err = f(r.shootActuator); err != nil {
		return err
	}

	return nil
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	r.statusUpdater.InjectClient(client)
	return nil
}

func (r *reconciler) InjectAPIReader(reader client.Reader) error {
	r.reader = reader
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	loggingResource := &extensionsv1alpha1.Logging{}
	if err := r.client.Get(ctx, request.NamespacedName, loggingResource); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var cluster *extensions.Cluster
	var err error
	if loggingResource.Spec.Type != "seed" {
		cluster, err = extensionscontroller.GetCluster(ctx, r.client, loggingResource.Namespace)
		if err != nil {
			return reconcile.Result{}, err
		}
		// TODO: Discuss if this is needed
		// if extensionscontroller.IsFailed(cluster) {
		// 	log.Info("Skipping the reconciliation of Logging of failed shoot")
		// 	return reconcile.Result{}, nil
		// }
	}

	operationType := gardencorev1beta1helper.ComputeOperationType(loggingResource.ObjectMeta, loggingResource.Status.LastOperation)

	switch {
	case extensionscontroller.ShouldSkipOperation(operationType, loggingResource):
		return reconcile.Result{}, nil
	case operationType == gardencorev1beta1.LastOperationTypeMigrate:
		return r.migrate(ctx, log, loggingResource, cluster)
	case loggingResource.DeletionTimestamp != nil:
		return r.delete(ctx, log, loggingResource, cluster)
	case operationType == gardencorev1beta1.LastOperationTypeRestore:
		return r.restore(ctx, log, loggingResource, cluster)
	default:
		return r.reconcile(ctx, log, loggingResource, cluster, operationType)
	}
}

func (r *reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	logging *extensionsv1alpha1.Logging,
	cluster *extensions.Cluster,
	operationType gardencorev1beta1.LastOperationType,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(logging, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, logging, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	updateStatusFunc := func(status extensionsv1alpha1.Status) error {
		loggingStatus := status.(*extensionsv1alpha1.LoggingStatus)
		loggingStatus.GrafanaDatasource = `
- name: loki
  type: loki
  access: proxy
  url: http://loki.` + logging.Namespace + `.svc:3100`
		return nil
	}

	if err := r.statusUpdater.ProcessingCustom(ctx, log, logging, operationType, "Reconciling the Logging", updateStatusFunc); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the reconciliation of logging")
	var units []extensionsv1alpha1.Unit
	var files []extensionsv1alpha1.File
	var err error
	if logging.Spec.Type == "seed" {
		if err, _, _ = r.seedActuator.Reconcile(ctx, log, logging, cluster); err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, logging, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling Logging", nil)
			return reconcilerutils.ReconcileErr(err)
		}
	} else {
		if err, units, files = r.shootActuator.Reconcile(ctx, log, logging, cluster); err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, logging, reconcilerutils.ReconcileErrCauseOrErr(err), operationType, "Error reconciling Logging", nil)
			return reconcilerutils.ReconcileErr(err)
		}
	}

	updateFilesAndUnitsFunc := func(status extensionsv1alpha1.Status) error {
		loggingStatus := status.(*extensionsv1alpha1.LoggingStatus)
		loggingStatus.Units = units
		loggingStatus.Files = files

		return nil
	}

	if err := r.statusUpdater.SuccessCustom(ctx, log, logging, operationType, "Successfully reconciled Logging", updateFilesAndUnitsFunc); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	logging *extensionsv1alpha1.Logging,
	cluster *extensions.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(logging, FinalizerName) {
		log.Info("Deleting Logging causes a no-op as there is no finalizer")
		return reconcile.Result{}, nil
	}

	if err := r.statusUpdater.ProcessingCustom(ctx, log, logging, gardencorev1beta1.LastOperationTypeDelete, "Deleting the Logging", nil); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Starting the deletion of Logging")
	if logging.Spec.Type == "seed" {
		if err := r.seedActuator.Delete(ctx, log, logging, cluster); err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, logging, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting Logging", nil)
			return reconcilerutils.ReconcileErr(err)
		}
	} else {
		if err := r.shootActuator.Delete(ctx, log, logging, cluster); err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, logging, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error deleting Logging", nil)
			return reconcilerutils.ReconcileErr(err)
		}
	}

	if err := r.statusUpdater.SuccessCustom(ctx, log, logging, gardencorev1beta1.LastOperationTypeDelete, "Successfully deleted Logging", nil); err != nil {
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(logging, FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.client, logging, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) migrate(
	ctx context.Context,
	log logr.Logger,
	logging *extensionsv1alpha1.Logging,
	cluster *extensions.Cluster,
) (
	reconcile.Result,
	error,
) {
	if err := r.statusUpdater.ProcessingCustom(ctx, log, logging, gardencorev1beta1.LastOperationTypeMigrate, "Migrating the Logging", nil); err != nil {
		return reconcile.Result{}, err
	}

	if logging.Spec.Type == "seed" {
		if err := r.seedActuator.Migrate(ctx, log, logging, cluster); err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, logging, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating Loggng", nil)
			return reconcilerutils.ReconcileErr(err)
		}
	} else {
		if err := r.shootActuator.Migrate(ctx, log, logging, cluster); err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, logging, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeMigrate, "Error migrating Loggng", nil)
			return reconcilerutils.ReconcileErr(err)
		}
	}

	if err := r.statusUpdater.SuccessCustom(ctx, log, logging, gardencorev1beta1.LastOperationTypeMigrate, "Successfully migrated Logging", nil); err != nil {
		return reconcile.Result{}, err
	}

	log.Info("Removing all finalizers")
	if err := controllerutils.RemoveAllFinalizers(ctx, r.client, logging); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing finalizers: %w", err)
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, logging, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from Logging: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) restore(
	ctx context.Context,
	log logr.Logger,
	logging *extensionsv1alpha1.Logging,
	cluster *extensions.Cluster,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(logging, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.client, logging, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	updateStatusFunc := func(status extensionsv1alpha1.Status) error {
		loggingStatus := status.(*extensionsv1alpha1.LoggingStatus)
		loggingStatus.GrafanaDatasource = `
- name: loki
  type: loki
  access: proxy
  url: http://loki.` + logging.Namespace + `.svc:3100`
		return nil
	}

	if err := r.statusUpdater.ProcessingCustom(ctx, log, logging, gardencorev1beta1.LastOperationTypeRestore, "Restoring the Logging", updateStatusFunc); err != nil {
		return reconcile.Result{}, err
	}

	var units []extensionsv1alpha1.Unit
	var files []extensionsv1alpha1.File
	var err error
	if logging.Spec.Type == "seed" {
		if err, _, _ = r.seedActuator.Restore(ctx, log, logging, cluster); err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, logging, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error restoring Logging", nil)
			return reconcilerutils.ReconcileErr(err)
		}
	} else {
		if err, units, files = r.shootActuator.Restore(ctx, log, logging, cluster); err != nil {
			_ = r.statusUpdater.ErrorCustom(ctx, log, logging, reconcilerutils.ReconcileErrCauseOrErr(err), gardencorev1beta1.LastOperationTypeDelete, "Error restoring Logging", nil)
			return reconcilerutils.ReconcileErr(err)
		}
	}

	updateFilesAndUnitsFunc := func(status extensionsv1alpha1.Status) error {
		loggingStatus := status.(*extensionsv1alpha1.LoggingStatus)
		loggingStatus.Units = units
		loggingStatus.Files = files

		return nil
	}

	if err := r.statusUpdater.SuccessCustom(ctx, log, logging, gardencorev1beta1.LastOperationTypeRestore, "Successfully restored Logging", updateFilesAndUnitsFunc); err != nil {
		return reconcile.Result{}, err
	}

	if err := extensionscontroller.RemoveAnnotation(ctx, r.client, logging, v1beta1constants.GardenerOperation); err != nil {
		return reconcile.Result{}, fmt.Errorf("error removing annotation from Logging: %+v", err)
	}

	return reconcile.Result{}, nil
}
