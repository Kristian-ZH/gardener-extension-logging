// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"

	"github.com/Kristian-ZH/gardener-extension-logging/pkg/apis/config"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	versionutils "github.com/gardener/gardener/pkg/utils/version"

	"github.com/go-logr/logr"
)

// Actuator is the Actuator's interface
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

func isShootEventLoggerEnabled(config config.Configuration) bool {
	return config.FeatureGates["eventLoggingEnabled"]
}

func isShootNodeLoggingEnabled(config config.Configuration, shootPurpose *v1beta1.ShootPurpose) bool {
	if shootPurpose == nil || !config.FeatureGates["shootNodeLoggingEnabled"] {
		return false
	}

	allowedPurposes := config.ShootPurposesWithNodeLogging
	for _, purpose := range allowedPurposes {
		fmt.Println(string(*shootPurpose) + " " + purpose)
		if string(*shootPurpose) == purpose {
			return true
		}
	}

	return false
}

func managedIngress(seed *v1beta1.Seed) bool {
	return seed.Spec.DNS.Provider != nil && seed.Spec.Ingress != nil && seed.Spec.Ingress.Controller.Kind == v1beta1constants.IngressKindNginx
}

// ComputeNginxIngressClass returns the IngressClass for the Nginx Ingress controller
func ComputeNginxIngressClass(seed *v1beta1.Seed, kubernetesVersion *string) (string, error) {
	managed := managedIngress(seed)

	if kubernetesVersion == nil {
		return "", fmt.Errorf("kubernetes version is missing in status for seed %q", seed.Name)
	}
	// We need to use `versionutils.CompareVersions` because this function normalizes the seed version first.
	// This is especially necessary if the seed cluster is a non Gardener managed cluster and thus might have some
	// custom version suffix.
	greaterEqual122, err := versionutils.CompareVersions(*kubernetesVersion, ">=", "1.22")
	if err != nil {
		return "", err
	}

	if managed && greaterEqual122 {
		return v1beta1constants.SeedNginxIngressClass122, nil
	}
	if managed {
		return v1beta1constants.SeedNginxIngressClass, nil
	}
	return v1beta1constants.NginxIngressClass, nil
}
