// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	_ "embed"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/go-logr/logr"
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
