// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	_ "embed"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"

	"github.com/go-logr/logr"
)

type Actuator interface {
	// Reconcile the Extension resource.
	Reconcile(context.Context, logr.Logger, *extensionsv1alpha1.Logging, *extensions.Cluster) error
	// Delete the Extension resource.
	Delete(context.Context, logr.Logger, *extensionsv1alpha1.Logging, *extensions.Cluster) error
	// Restore the Extension resource.
	Restore(context.Context, logr.Logger, *extensionsv1alpha1.Logging, *extensions.Cluster) error
	// Migrate the Extension resource.
	Migrate(context.Context, logr.Logger, *extensionsv1alpha1.Logging, *extensions.Cluster) error
}

// loggingReplicaFunc returns the desired replicas for the logging components
var loggingReplicaFunc = func(cluster *extensions.Cluster) int32 {
	switch {
	// If the cluster is hibernated then there is no further need of Logging pods and therefore its desired replicas is 0
	case extensionscontroller.IsHibernated(cluster):
		return 0
	// If the cluster is created with hibernation enabled, then desired replicas for the Logging pods is 0
	case extensionscontroller.IsHibernationEnabled(cluster) && extensionscontroller.IsCreationInProcess(cluster):
		return 1
	// If shoot is either waking up or in the process of hibernation then, Logging is required and therefore its desired replicas is 1
	case extensionscontroller.IsHibernatingOrWakingUp(cluster):
		return 1
	// If the shoot is awake then MCM should be available and therefore its desired replicas is 1
	default:
		return 1
	}
}
