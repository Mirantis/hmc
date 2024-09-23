// Copyright 2024
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ClusterTemplateChain is the Schema for the cluster template chain API
type ClusterTemplateChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Spec is immutable"
	Spec TemplateChainSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterTemplateChainList contains a list of ClusterTemplateChain
type ClusterTemplateChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTemplateChain `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster

// ServiceTemplateChain is the Schema for the service template chain API
type ServiceTemplateChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Spec is immutable"
	Spec TemplateChainSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// ServiceTemplateChainList contains a list of ServiceTemplateChain
type ServiceTemplateChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServiceTemplateChain `json:"items"`
}

// TemplateChainSpec defines the observed state of TemplateChain
type TemplateChainSpec struct {
	// SupportedTemplates is the list of supported Templates definitions and all available upgrade sequences for it.
	// +optional
	SupportedTemplates []SupportedTemplate `json:"supportedTemplates,omitempty"`
}

// SupportedTemplate is the supported Template definition and all available upgrade sequences for it
type SupportedTemplate struct {
	// Name is the name of the Template.
	Name string `json:"name"`
	// AvailableUpgrades is the list of available upgrades for the specified Template.
	// +optional
	AvailableUpgrades []AvailableUpgrade `json:"availableUpgrades,omitempty"`
}

// AvailableUpgrade is the definition of the available upgrade for the Template
type AvailableUpgrade struct {
	// Name is the name of the Template to which the upgrade is available.
	Name string `json:"name"`
}

func init() {
	SchemeBuilder.Register(&ClusterTemplateChain{}, &ClusterTemplateChainList{})
	SchemeBuilder.Register(&ServiceTemplateChain{}, &ServiceTemplateChainList{})
}
