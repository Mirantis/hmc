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

// HMCTemplateSpec defines the desired state of HMCTemplate
type HMCTemplateSpec struct {
	// Provider specifies a CAPI provider associated with the template.
	// +kubebuilder:validation:Enum=aws
	// +kubebuilder:validation:Required
	Provider string `json:"provider"`
	// HelmChartURL is a URL of the helm chart representing the template.
	// +kubebuilder:validation:Required
	HelmChartURL string `json:"helmChartURL"`
}

// HMCTemplateStatus defines the observed state of HMCTemplate
type HMCTemplateStatus struct {
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

// HMCTemplate is the Schema for the hmctemplates API
type HMCTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HMCTemplateSpec   `json:"spec,omitempty"`
	Status HMCTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HMCTemplateList contains a list of HMCTemplate
type HMCTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HMCTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HMCTemplate{}, &HMCTemplateList{})
}
