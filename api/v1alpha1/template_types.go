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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	helmcontrollerv2 "github.com/fluxcd/helm-controller/api/v2"
)

const (
	// TemplateKind is the string representation of a Template.
	TemplateKind   = "Template"
	DeploymentKind = "Deployment"
)

// TemplateSpec defines the desired state of Template
type TemplateSpec struct {
	// Provider specifies a CAPI provider associated with the template.
	// +kubebuilder:validation:Enum=aws
	// +kubebuilder:validation:Required
	Provider string `json:"provider"`
	// Helm holds a reference to a Helm chart representing the HMC template
	// +kubebuilder:validation:Required
	Helm HelmSpec `json:"helm"`
}

// +kubebuilder:validation:XValidation:rule="(has(self.chartName) && !has(self.chartRef)) || (!has(self.chartName) && has(self.chartRef))", message="either chartName or chartRef must be set"

// HelmSpec references a Helm chart representing the HMC template
type HelmSpec struct {
	// ChartName is a name of a Helm chart representing the template in the HMC repository.
	// +optional
	ChartName string `json:"chartName,omitempty"`
	// ChartVersion is a version of a Helm chart representing the template in the HMC repository.
	// +optional
	ChartVersion string `json:"chartVersion,omitempty"`
	// ChartRef is a reference to a source controller resource containing the
	// Helm chart representing the template.
	// +optional
	ChartRef *helmcontrollerv2.CrossNamespaceSourceReference `json:"chartRef,omitempty"`
}

// TemplateStatus defines the observed state of Template
type TemplateStatus struct {
	TemplateValidationStatus `json:",inline"`
	// Descriptions contains information about the template.
	// +optional
	Description string `json:"description,omitempty"`
	// Config demonstrates available parameters for template customization,
	// that can be used when creating Deployment objects.
	// +optional
	Config *apiextensionsv1.JSON `json:"config,omitempty"`
	// ChartRef is a reference to a source controller resource containing the
	// Helm chart representing the template.
	// +optional
	ChartRef *helmcontrollerv2.CrossNamespaceSourceReference `json:"chartRef,omitempty"`
}

type TemplateValidationStatus struct {
	// Valid indicates whether the template passed validation or not.
	Valid bool `json:"valid"`
	// ValidationError provides information regarding issues encountered during template validation.
	// +optional
	ValidationError string `json:"validationError,omitempty"`
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
