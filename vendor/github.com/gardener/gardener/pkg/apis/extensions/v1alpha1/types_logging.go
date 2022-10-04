// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ Object = (*Logging)(nil)

// LoggingResource is a constant for the name of the Extension resource.
const LoggingResource = "Logging"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope=Namespaced,path=extensions,shortName=ext,singular=logging
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=Type,JSONPath=".spec.type",type=string,description="The type of the Logging resource."
// +kubebuilder:printcolumn:name=Status,JSONPath=".status.lastOperation.state",type=string,description="Status of Logging resource."
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date,description="creation timestamp"

// Logging is a specification for a Logging resource.
type Logging struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the Logging.
	// If the object's deletion timestamp is set, this field is immutable.
	Spec LoggingSpec `json:"spec"`
	// +optional
	Status LoggingStatus `json:"status"`
}

// GetExtensionSpec implements Object.
func (i *Logging) GetExtensionSpec() Spec {
	return &i.Spec
}

// GetExtensionStatus implements Object.
func (i *Logging) GetExtensionStatus() Status {
	return &i.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// LoggingList is a list of Logging resources.
type LoggingList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Logging `json:"items"`
}

// ExtensionSpec is the spec for a Extension resource.
type LoggingSpec struct {
	// DefaultSpec is a structure containing common fields used by all extension resources.
	DefaultSpec `json:",inline"`
	HvpaEnabled bool `json:"hvpaEnabled,omitempty"`
}

// ExtensionStatus is the status for a Logging resource.
type LoggingStatus struct {
	// DefaultStatus is a structure containing common fields used by all extension resources.
	DefaultStatus     `json:",inline"`
	GrafanaDatasource string `json:"grafanaDatasource,omitempty"`
	Files             []File `json:"files,omitempty"`
	Units             []Unit `json:"units,omitempty"`
}
