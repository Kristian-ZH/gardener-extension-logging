// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	_ "embed"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Actuator interface {
	// Reconcile the Extension resource.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Logging) error
	// Delete the Extension resource.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Logging) error
	// Restore the Extension resource.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.Logging) error
	// Migrate the Extension resource.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Logging) error
}

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator() Actuator {
	return &actuator{
		logger: log.Log.WithName("FirstLogger"),
	}
}

type actuator struct {
	logger logr.Logger // logger
}

// Reconcile the Extension resource.
func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, ex *extensionsv1alpha1.Logging) error {
	a.logger.Info("Hello World, I just entered the Reconcile method")
	fmt.Println("PRINTLN Hello World, I just entered the Reconcile method")
	return nil
}

// Delete the Extension resource.
func (a *actuator) Delete(ctx context.Context, _ logr.Logger, ex *extensionsv1alpha1.Logging) error {
	a.logger.Info("Hello World, I just entered the Delete method")
	return nil
}

// Restore the Extension resource.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Logging) error {
	return a.Reconcile(ctx, log, ex)
}

// Migrate the Extension resource.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Logging) error {
	return a.Delete(ctx, log, ex)
}
