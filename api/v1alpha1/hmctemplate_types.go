/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TemplateSpec defines the desired state of Template
type TemplateSpec struct {
	// Provider specifies a CAPI provider associated with the template.
	// +kubebuilder:validation:Enum=aws
	// +kubebuilder:validation:Required
	Provider string `json:"provider"`
	// HelmChartURL is a URL of the helm chart representing the template.
	// +kubebuilder:validation:Required
	HelmChartURL string `json:"helmChartURL"`
}

// TemplateStatus defines the observed state of Template
type TemplateStatus struct {
	TemplateValidationStatus `json:",inline"`
	// Descriptions contains information about the template.
	// +optional
	Description string `json:"description"`
}

type TemplateValidationStatus struct {
	// Valid indicates whether the template passed validation or not.
	Valid bool `json:"valid"`
	// ValidationError provides information regarding issues encountered during template validation.
	// +optional
	ValidationError string `json:"validationError"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Template is the Schema for the templates API
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemplateSpec   `json:"spec,omitempty"`
	Status TemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TemplateList contains a list of Template
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Template{}, &TemplateList{})
}
