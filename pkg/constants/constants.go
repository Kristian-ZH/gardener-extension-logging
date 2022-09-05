// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// ExtensionType is the name of the extension type.
	ExtensionType = "logging"
	// ServiceName is the name of the service.
	ServiceName = ExtensionType

	extensionServiceName = "extension-" + ServiceName
	// EgressFilterSecretName name of the secret containing the egress filter lists
	EgressFilterSecretName = extensionServiceName
	// NamespaceKubeSystem kube-system namespace
	NamespaceKubeSystem = "kube-system"
	// ManagedResourceNamesShoot is the name used to describe the managed shoot resources.
	ManagedResourceNamesShoot = extensionServiceName + "-shoot"
	// ManagedResourceNamesSeed is the name used to describe the managed seed resources.
	ManagedResourceNamesSeed = extensionServiceName + "-seed"
)
