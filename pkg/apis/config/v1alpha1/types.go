// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	healthcheckconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Configuration contains information about the Logging service configuration.
type Configuration struct {
	metav1.TypeMeta `json:",inline"`

	// HealthCheckConfig is the config for the health check controller.
	// +optional
	HealthCheckConfig *healthcheckconfigv1alpha1.HealthCheckConfig `json:"healthCheckConfig,omitempty"`

	// ShootPurposesWithNodeLogging are the shoot purposes for which there will be installed node logging.
	ShootPurposesWithNodeLogging []string `json:"shootPurposesWithNodeLogging,omitempty"`

	// FeatureGates is a map of feature names to bools that enable features.
	// Default: nil
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}
